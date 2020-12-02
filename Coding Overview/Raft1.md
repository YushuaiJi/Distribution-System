## Raft1的实现

raft1实现了server选举的相关功能，lab1不牵涉到 log。如果有疑问可以转移到[raft1](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/Raft1.md)

Raft 是每个 server 持有一个的结构，作为状态机的存在。每个 server 存在三种状态：

- Follower
- Candidate
- Leader
（整个运行中，只有两种 RPC ----1：请求投票的 RequestVote 2：修改log（也作为心跳包）的 AppendEntries。他们都是通过 sendXXX() 来调用其他 server 的 XXX() 方法。）

除两个 RPC以外，还有两个需要period检查的 timer。(在 timeout时重新选举 Leader 的 electionTimer+在timeout 时发送心跳包的 heartbeatTimer)

## Raft Structure

对于每一个不同状态的 Raft 节点，他们可能会发生如下并发的事件：

## Follower

- AppendEntries
- RequestVote
- electionTimer 超时

## Candidate
- AppendEntries
- RequestVote
- electionTimer 超时
- sendRequestVote

## Leader
- AppendEntries
- RequestVote
- electionTimer 超时
- heartbeatTimer 超时
发送心跳包 sendAppendEntries
避免 race condition 在labA还是比较麻烦的。

lab的rpc 的特性决定了 AppendEntries 和 RequestVote 都是在一个new goroutine 中handle的，这两个函数都需要使用 mutex 保护（我们无需关心如何去给他们安排线程）。

至于两个 timer electionTimer 和 heartbeatTimer，我们可以在构造 Raft 实例时 kickoff 一个 goroutine，在其中利用 select 不断处理这两个 timer 的 timeout 事件。在处理的时候 并行 地调用 sendAppendEntries 和 sendRequestVote。

最好写一个专门的状态转换函数来处理 Raft 节点三种状态的转换。

## Code

1：Raft 增加了必要的 field。在 lab2 A 中需要新加 5 个 fields 就足够了。不需要任何 log 相关的东西。（需要注意的是在读写 currentTerm, votedFor, state 时都需要加锁保护）

```Go
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm    int         // 2A
	votedFor       int         // 2A
	electionTimer  *time.Timer // 2A
	heartbeatTimer *time.Timer // 2A
	state          NodeState   // 2A
}
```

构造 Raft 时，kickoff 一个 goroutine 来处理 timer 相关的事件，注意加锁。

```Go
go func(node *Raft) {
  for {
    select {
    case <-rf.electionTimer.C:
      rf.mu.Lock()
      if rf.state == Follower {
        // rf.startElection() is called in conversion to Candidate
        rf.convertTo(Candidate)
      } else {
        rf.startElection()
      }
      rf.mu.Unlock()

    case <-rf.heartbeatTimer.C:
      rf.mu.Lock()
      if rf.state == Leader {
        rf.broadcastHeartbeat()
        rf.heartbeatTimer.Reset(HeartbeatInterval)
      }
      rf.mu.Unlock()
    }
  }
}(rf)
```

状态转换函数，注意最好由 caller 来加锁避免错误导致死锁。该函数也非常简明，需要注意这里对两个 timer 的处理。
```Go
// should be called with a lock
func (rf *Raft) convertTo(s NodeState) {
	if s == rf.state {
		return
	}
	DPrintf("Term %d: server %d convert from %v to %v\n",
		rf.currentTerm, rf.me, rf.state, s)
	rf.state = s
	switch s {
	case Follower:
		rf.heartbeatTimer.Stop()
		rf.electionTimer.Reset(randTimeDuration(ElectionTimeoutLower, ElectionTimeoutUpper))
		rf.votedFor = -1

	case Candidate:
		rf.startElection()

	case Leader:
		rf.electionTimer.Stop()
		rf.broadcastHeartbeat()
		rf.heartbeatTimer.Reset(HeartbeatInterval)
	}
}
```

以选举为例讲下如何并行 RPC 调用。
```Go
// should be called with lock
func (rf *Raft) startElection() {
	rf.currentTerm += 1
	rf.electionTimer.Reset(randTimeDuration(ElectionTimeoutLower, ElectionTimeoutUpper))

	args := RequestVoteArgs{
		Term:        rf.currentTerm,
		CandidateId: rf.me,
	}

	var voteCount int32

	for i := range rf.peers {
		if i == rf.me {
			rf.votedFor = rf.me
			atomic.AddInt32(&voteCount, 1)
			continue
		}
		go func(server int) {
			var reply RequestVoteReply
			if rf.sendRequestVote(server, &args, &reply) {
				rf.mu.Lock()

				if reply.VoteGranted && rf.state == Candidate {
					atomic.AddInt32(&voteCount, 1)
					if atomic.LoadInt32(&voteCount) > int32(len(rf.peers)/2) {
						rf.convertTo(Leader)
					}
				} else {
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.convertTo(Follower)
					}
				}
				rf.mu.Unlock()
			} else {
				DPrintf("%v send request vote to %d failed", rf, server)
			}
		}(i)
	}
}
```

需要注意的是以下几点：

- 保证在外界加锁后调用 startElection()

- 对其他每个节点开启一个 goroutine 调用 sendRequestVote()，并在处理返回值时候加锁

- 注意我们维护了一个记录投票数的 int32 并在处理请求返回值时候用原子操作进行读写判断，以决定是否升级为 Leader。

