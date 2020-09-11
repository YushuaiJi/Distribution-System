package raftkv

import (
	"bytes"
	"labgob"
	"labrpc"
	"log"
	"raft"
	"sync"
	"time"
)

func init() {
	log.SetFlags(log.Flags() | log.Lmicroseconds)
}

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Key   string
	Value string
	Name  string

	ClientId  int64
	RequestId int
}

// notify the RPC handler that a request from a client has been done
type Notification struct {
	ClientId  int64
	RequestId int
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	db                   map[string]string         // 3A
	dispatcher           map[int]chan Notification // 3A
	lastAppliedRequestId map[int64]int             // 3A

	appliedRaftLogIndex int // 3B
}

func (kv *KVServer) shouldTakeSnapshot() bool {
	if kv.maxraftstate == -1 {
		return false
	}

	if kv.rf.GetRaftStateSize() >= kv.maxraftstate {
		return true
	}
	return false
}

func (kv *KVServer) takeSnapshot() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	kv.mu.Lock()
	e.Encode(kv.db)
	e.Encode(kv.lastAppliedRequestId)
	appliedRaftLogIndex := kv.appliedRaftLogIndex
	kv.mu.Unlock()

	kv.rf.ReplaceLogWithSnapshot(appliedRaftLogIndex, w.Bytes())
}

// should be called with lock
func (kv *KVServer) isDuplicateRequest(clientId int64, requestId int) bool {
	appliedRequestId, ok := kv.lastAppliedRequestId[clientId]
	if ok == false || requestId > appliedRequestId {
		return false
	}
	return true
}

func (kv *KVServer) waitApplying(op Op, timeout time.Duration) bool {
	// return common part of GetReply and PutAppendReply
	// i.e., WrongLeader
	index, _, isLeader := kv.rf.Start(op)
	if isLeader == false {
		return true
	}

	if kv.shouldTakeSnapshot() {
		kv.takeSnapshot()
	}

	var wrongLeader bool

	kv.mu.Lock()
	if _, ok := kv.dispatcher[index]; !ok {
		kv.dispatcher[index] = make(chan Notification, 1)
	}
	ch := kv.dispatcher[index]
	kv.mu.Unlock()
	select {
	case notify := <-ch:
		if notify.ClientId != op.ClientId || notify.RequestId != op.RequestId {
			// leader has changed
			wrongLeader = true
		} else {
			wrongLeader = false
		}

	case <-time.After(timeout):
		kv.mu.Lock()
		if kv.isDuplicateRequest(op.ClientId, op.RequestId) {
			wrongLeader = false
		} else {
			wrongLeader = true
		}
		kv.mu.Unlock()
	}
	DPrintf("kvserver %d got %s() RPC, insert op %+v at %d, reply WrongLeader = %v",
		kv.me, op.Name, op, index, wrongLeader)

	kv.mu.Lock()
	delete(kv.dispatcher, index)
	kv.mu.Unlock()
	return wrongLeader
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	op := Op{
		Key:       args.Key,
		Name:      "Get",
		ClientId:  args.ClientId,
		RequestId: args.RequestId,
	}

	// wait for being applied
	// or leader changed (log is overrided, and never gets applied)
	reply.WrongLeader = kv.waitApplying(op, 500*time.Millisecond)

	if reply.WrongLeader == false {
		kv.mu.Lock()
		value, ok := kv.db[args.Key]
		kv.mu.Unlock()
		if ok {
			reply.Value = value
			return
		}
		// not found
		reply.Err = ErrNoKey
	}

}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	op := Op{
		Key:       args.Key,
		Value:     args.Value,
		Name:      args.Op,
		ClientId:  args.ClientId,
		RequestId: args.RequestId,
	}

	// wait for being applied
	// or leader changed (log is overrided, and never gets applied)
	reply.WrongLeader = kv.waitApplying(op, 500*time.Millisecond)
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *KVServer) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) installSnapshot(snapshot []byte) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if snapshot != nil {
		r := bytes.NewBuffer(snapshot)
		d := labgob.NewDecoder(r)
		if d.Decode(&kv.db) != nil ||
			d.Decode(&kv.lastAppliedRequestId) != nil {
			DPrintf("kvserver %d fails to recover from snapshot", kv.me)
		}
	}
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.
	kv.db = make(map[string]string)
	kv.dispatcher = make(map[int]chan Notification)
	kv.lastAppliedRequestId = make(map[int64]int)

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// 3B: recover from snapshot
	snapshot := persister.ReadSnapshot()
	kv.installSnapshot(snapshot)

	// You may need initialization code here.
	go func() {
		for msg := range kv.applyCh {
			if msg.CommandValid == false {
				switch msg.Command.(string) {
				case "InstallSnapshot":
					kv.installSnapshot(msg.CommandData)
				}
				continue
			}

			op := msg.Command.(Op)
			DPrintf("kvserver %d start applying command %s at index %d, request id %d, client id %d",
				kv.me, op.Name, msg.CommandIndex, op.RequestId, op.ClientId)
			kv.mu.Lock()
			if kv.isDuplicateRequest(op.ClientId, op.RequestId) {
				kv.mu.Unlock()
				continue
			}
			switch op.Name {
			case "Put":
				kv.db[op.Key] = op.Value
			case "Append":
				kv.db[op.Key] += op.Value
				// Get() does not need to modify db, skip
			}
			kv.lastAppliedRequestId[op.ClientId] = op.RequestId
			kv.appliedRaftLogIndex = msg.CommandIndex

			if ch, ok := kv.dispatcher[msg.CommandIndex]; ok {
				notify := Notification{
					ClientId:  op.ClientId,
					RequestId: op.RequestId,
				}
				ch <- notify
			}

			kv.mu.Unlock()
			DPrintf("kvserver %d applied command %s at index %d, request id %d, client id %d",
				kv.me, op.Name, msg.CommandIndex, op.RequestId, op.ClientId)
		}
	}()

	return kv
}
