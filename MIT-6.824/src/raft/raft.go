package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import "labrpc"
import "time"
import "math/rand"
import "sync/atomic"
import "fmt"

import "bytes"
import "labgob"

const (
	HeartbeatInterval    = time.Duration(120) * time.Millisecond
	ElectionTimeoutLower = time.Duration(300) * time.Millisecond
	ElectionTimeoutUpper = time.Duration(400) * time.Millisecond
)

type NodeState uint8

const (
	Follower  = NodeState(1)
	Candidate = NodeState(2)
	Leader    = NodeState(3)
)

func (s NodeState) String() string {
	switch {
	case s == Follower:
		return "Follower"
	case s == Candidate:
		return "Candidate"
	case s == Leader:
		return "Leader"
	}
	return "Unknown"
}

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
		resetTimer(rf.electionTimer,
			randTimeDuration(ElectionTimeoutLower, ElectionTimeoutUpper))
		rf.votedFor = -1

	case Candidate:
		rf.startElection()

	case Leader:
		for i := range rf.nextIndex {
			// initialized to leader last log index + 1
			rf.nextIndex[i] = rf.getAbsoluteLogIndex(len(rf.logs))
		}
		for i := range rf.matchIndex {
			rf.matchIndex[i] = rf.snapshottedIndex
		}

		rf.electionTimer.Stop()
		rf.broadcastHeartbeat()
		resetTimer(rf.heartbeatTimer, HeartbeatInterval)
	}
}

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// to send kv snapshot to kv server
	CommandData []byte // 3B
}

type LogEntry struct {
	Command interface{}
	Term    int
}

//
// A Go object implementing a single Raft peer.
//
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

	logs        []LogEntry    // 2B
	commitIndex int           // 2B
	lastApplied int           // 2B
	nextIndex   []int         // 2B
	matchIndex  []int         // 2B
	applyCh     chan ApplyMsg // 2B

	snapshottedIndex int // 3B
}

func (rf Raft) String() string {
	return fmt.Sprintf("[node(%d), state(%v), term(%d), snapshottedIndex(%d)]",
		rf.me, rf.state, rf.currentTerm, rf.snapshottedIndex)
}

func (rf *Raft) getRelativeLogIndex(index int) int {
	// index of rf.logs
	return index - rf.snapshottedIndex
}

func (rf *Raft) getAbsoluteLogIndex(index int) int {
	// index of log including snapshotted ones
	return index + rf.snapshottedIndex
}

//
// several setters, should be called with a lock
//

func (rf *Raft) setCommitIndex(commitIndex int) {
	rf.commitIndex = commitIndex
	// apply all entries between lastApplied and committed
	// should be called after commitIndex updated
	if rf.commitIndex > rf.lastApplied {
		DPrintf("%v apply from index %d to %d", rf, rf.lastApplied+1, rf.commitIndex)
		entriesToApply := append([]LogEntry{},
			rf.logs[rf.getRelativeLogIndex(rf.lastApplied+1):rf.getRelativeLogIndex(rf.commitIndex+1)]...)

		go func(startIdx int, entries []LogEntry) {
			for idx, entry := range entries {
				var msg ApplyMsg
				msg.CommandValid = true
				msg.Command = entry.Command
				msg.CommandIndex = startIdx + idx
				rf.applyCh <- msg
				// do not forget to update lastApplied index
				// this is another goroutine, so protect it with lock
				rf.mu.Lock()
				if rf.lastApplied < msg.CommandIndex {
					rf.lastApplied = msg.CommandIndex
				}
				rf.mu.Unlock()
			}
		}(rf.lastApplied+1, entriesToApply)
	}
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	var term int
	var isleader bool
	// Your code here (2A).
	term = rf.currentTerm
	isleader = rf.state == Leader
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//

func (rf *Raft) encodeRaftState() []byte {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.snapshottedIndex)
	e.Encode(rf.logs)
	return w.Bytes()
}

func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
	rf.persister.SaveRaftState(rf.encodeRaftState())
}

func (rf *Raft) GetRaftStateSize() int {
	return rf.persister.RaftStateSize()
}

func (rf *Raft) ReplaceLogWithSnapshot(appliedIndex int, kvSnapshot []byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if appliedIndex <= rf.snapshottedIndex {
		return
	}
	// truncate log, keep snapshottedIndex as a guard at rf.logs[0]
	// because it must be committed and applied
	rf.logs = rf.logs[rf.getRelativeLogIndex(appliedIndex):]
	rf.snapshottedIndex = appliedIndex
	rf.persister.SaveStateAndSnapshot(rf.encodeRaftState(), kvSnapshot)

	// update for other nodes
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go rf.syncSnapshotWith(i)
	}
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }

	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm, votedFor, snapshottedIndex int
	var logs []LogEntry
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&snapshottedIndex) != nil ||
		d.Decode(&logs) != nil {
		DPrintf("%v fails to recover from persist", rf)
		return
	}

	rf.currentTerm = currentTerm
	rf.votedFor = votedFor
	rf.snapshottedIndex = snapshottedIndex
	rf.logs = logs

	// for lab 3b, we need to set them at the first index
	// i.e., 0 if snapshot is disabled
	rf.commitIndex = snapshottedIndex
	rf.lastApplied = snapshottedIndex
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term        int // 2A
	CandidateId int // 2A

	LastLogIndex int // 2B
	LastLogTerm  int // 2B
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int  // 2A
	VoteGranted bool // 2A
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist() // execute before rf.mu.Unlock()
	// Your code here (2A, 2B).
	if args.Term < rf.currentTerm ||
		(args.Term == rf.currentTerm && rf.votedFor != -1 && rf.votedFor != args.CandidateId) {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.convertTo(Follower)
		// do not return here.
	}

	// 2B: candidate's vote should be at least up-to-date as receiver's log
	// "up-to-date" is defined in thesis 5.4.1
	lastLogIndex := len(rf.logs) - 1
	if args.LastLogTerm < rf.logs[lastLogIndex].Term ||
		(args.LastLogTerm == rf.logs[lastLogIndex].Term &&
			args.LastLogIndex < rf.getAbsoluteLogIndex(lastLogIndex)) {
		// Receiver is more up-to-date, does not grant vote
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	rf.votedFor = args.CandidateId
	reply.Term = rf.currentTerm // not used, for better logging
	reply.VoteGranted = true
	// reset timer after grant vote
	resetTimer(rf.electionTimer,
		randTimeDuration(ElectionTimeoutLower, ElectionTimeoutUpper))
}

type AppendEntriesArgs struct {
	Term     int // 2A
	LeaderId int // 2A

	PrevLogIndex int        // 2B
	PrevLogTerm  int        // 2B
	LogEntries   []LogEntry // 2B
	LeaderCommit int        // 2B
}

type AppendEntriesReply struct {
	Term    int  // 2A
	Success bool // 2A

	// OPTIMIZE: see thesis section 5.3
	ConflictTerm  int // 2C
	ConflictIndex int // 2C
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist() // execute before rf.mu.Unlock()
	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.convertTo(Follower)
		// do not return here.
	}

	// reset election timer even log does not match
	// args.LeaderId is the current term's Leader
	resetTimer(rf.electionTimer,
		randTimeDuration(ElectionTimeoutLower, ElectionTimeoutUpper))

	if args.PrevLogIndex <= rf.snapshottedIndex {
		reply.Success = true

		// sync log if needed
		if args.PrevLogIndex+len(args.LogEntries) > rf.snapshottedIndex {
			// if snapshottedIndex == prevLogIndex, all log entries should be added.
			startIdx := rf.snapshottedIndex - args.PrevLogIndex
			// only keep the last snapshotted one
			rf.logs = rf.logs[:1]
			rf.logs = append(rf.logs, args.LogEntries[startIdx:]...)
		}

		return
	}

	// entries before args.PrevLogIndex might be unmatch
	// return false and ask Leader to decrement PrevLogIndex
	absoluteLastLogIndex := rf.getAbsoluteLogIndex(len(rf.logs) - 1)
	if absoluteLastLogIndex < args.PrevLogIndex {
		reply.Success = false
		reply.Term = rf.currentTerm
		// optimistically thinks receiver's log matches with Leader's as a subset
		reply.ConflictIndex = absoluteLastLogIndex + 1
		// no conflict term
		reply.ConflictTerm = -1
		return
	}

	if rf.logs[rf.getRelativeLogIndex(args.PrevLogIndex)].Term != args.PrevLogTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		// receiver's log in certain term unmatches Leader's log
		reply.ConflictTerm = rf.logs[rf.getRelativeLogIndex(args.PrevLogIndex)].Term

		// expecting Leader to check the former term
		// so set ConflictIndex to the first one of entries in ConflictTerm
		conflictIndex := args.PrevLogIndex
		// apparently, since rf.logs[0] are ensured to match among all servers
		// ConflictIndex must be > 0, safe to minus 1
		for rf.logs[rf.getRelativeLogIndex(conflictIndex-1)].Term == reply.ConflictTerm {
			conflictIndex--
			if conflictIndex == rf.snapshottedIndex+1 {
				// this may happen after snapshot,
				// because the term of the first log may be the current term
				// before lab 3b this is not going to happen, since rf.logs[0].Term = 0
				break
			}
		}
		reply.ConflictIndex = conflictIndex
		return
	}

	// compare from rf.logs[args.PrevLogIndex + 1]
	unmatch_idx := -1
	for idx := range args.LogEntries {
		if len(rf.logs) < rf.getRelativeLogIndex(args.PrevLogIndex+2+idx) ||
			rf.logs[rf.getRelativeLogIndex(args.PrevLogIndex+1+idx)].Term != args.LogEntries[idx].Term {
			// unmatch log found
			unmatch_idx = idx
			break
		}
	}

	if unmatch_idx != -1 {
		// there are unmatch entries
		// truncate unmatch Follower entries, and apply Leader entries
		rf.logs = rf.logs[:rf.getRelativeLogIndex(args.PrevLogIndex+1+unmatch_idx)]
		rf.logs = append(rf.logs, args.LogEntries[unmatch_idx:]...)
	}

	// Leader guarantee to have all committed entries
	// TODO: Is that possible for lastLogIndex < args.LeaderCommit?
	if args.LeaderCommit > rf.commitIndex {
		absoluteLastLogIndex := rf.getAbsoluteLogIndex(len(rf.logs) - 1)
		if args.LeaderCommit <= absoluteLastLogIndex {
			rf.setCommitIndex(args.LeaderCommit)
		} else {
			rf.setCommitIndex(absoluteLastLogIndex)
		}
	}

	reply.Success = true
}

// should be called with lock
func (rf *Raft) broadcastHeartbeat() {
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(server int) {
			rf.mu.Lock()
			if rf.state != Leader {
				rf.mu.Unlock()
				return
			}

			prevLogIndex := rf.nextIndex[server] - 1

			if prevLogIndex < rf.snapshottedIndex {
				// leader has discarded log entries the follower needs
				// send snapshot to follower and retry later
				rf.mu.Unlock()
				rf.syncSnapshotWith(server)
				return
			}
			// use deep copy to avoid race condition
			// when override log in AppendEntries()
			entries := make([]LogEntry, len(rf.logs[rf.getRelativeLogIndex(prevLogIndex+1):]))
			copy(entries, rf.logs[rf.getRelativeLogIndex(prevLogIndex+1):])

			args := AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  rf.logs[rf.getRelativeLogIndex(prevLogIndex)].Term,
				LogEntries:   entries,
				LeaderCommit: rf.commitIndex,
			}
			rf.mu.Unlock()

			var reply AppendEntriesReply
			if rf.sendAppendEntries(server, &args, &reply) {
				rf.mu.Lock()
				if rf.state != Leader {
					rf.mu.Unlock()
					return
				}
				if reply.Success {
					// successfully replicated args.LogEntries
					rf.matchIndex[server] = args.PrevLogIndex + len(args.LogEntries)
					rf.nextIndex[server] = rf.matchIndex[server] + 1

					// check if we need to update commitIndex
					// from the last log entry to committed one
					for i := rf.getAbsoluteLogIndex(len(rf.logs) - 1); i > rf.commitIndex; i-- {
						count := 0
						for _, matchIndex := range rf.matchIndex {
							if matchIndex >= i {
								count += 1
							}
						}

						if count > len(rf.peers)/2 {
							// most of nodes agreed on rf.logs[i]
							rf.setCommitIndex(i)
							break
						}
					}

				} else {
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.convertTo(Follower)
						rf.persist()
					} else {
						// log unmatch, update nextIndex[server] for the next trial
						rf.nextIndex[server] = reply.ConflictIndex

						// if term found, override it to
						// the first entry after entries in ConflictTerm
						if reply.ConflictTerm != -1 {
							DPrintf("%v conflict with server %d, prevLogIndex %d, log length = %d",
								rf, server, args.PrevLogIndex, len(rf.logs))
							for i := args.PrevLogIndex; i >= rf.snapshottedIndex+1; i-- {
								if rf.logs[rf.getRelativeLogIndex(i-1)].Term == reply.ConflictTerm {
									// in next trial, check if log entries in ConflictTerm matches
									rf.nextIndex[server] = i
									break
								}
							}
						}

						// TODO: retry now or in next RPC?
					}
				}
				rf.mu.Unlock()
			}
		}(i)
	}
}

// should be called with lock
func (rf *Raft) startElection() {
	defer rf.persist()

	rf.currentTerm += 1

	lastLogIndex := len(rf.logs) - 1
	args := RequestVoteArgs{
		Term:         rf.currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: rf.getAbsoluteLogIndex(lastLogIndex),
		LastLogTerm:  rf.logs[lastLogIndex].Term,
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
				DPrintf("%v got RequestVote response from node %d, VoteGranted=%v, Term=%d",
					rf, server, reply.VoteGranted, reply.Term)
				if reply.VoteGranted && rf.state == Candidate {
					atomic.AddInt32(&voteCount, 1)
					if atomic.LoadInt32(&voteCount) > int32(len(rf.peers)/2) {
						rf.convertTo(Leader)
					}
				} else {
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.convertTo(Follower)
						rf.persist()
					}
				}
				rf.mu.Unlock()
			}
		}(i)
	}
}

type InstallSnapshotArgs struct {
	// do not need to implement "chunk"
	// remove "offset" and "done"
	Term              int    // 3B
	LeaderId          int    // 3B
	LastIncludedIndex int    // 3B
	LastIncludedTerm  int    // 3B
	Data              []byte // 3B
}

type InstallSnapshotReply struct {
	Term int // 3B
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// we do not need to call rf.persist() in this function
	// because rf.persister.SaveStateAndSnapshot() is called

	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm || args.LastIncludedIndex < rf.snapshottedIndex {
		return
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.convertTo(Follower)
		// do not return here.
	}

	// step 2, 3, 4 is skipped because we simplify the "offset"

	// 6. if existing log entry has same index and term with
	//    last log entry in snapshot, retain log entries following it
	lastIncludedRelativeIndex := rf.getRelativeLogIndex(args.LastIncludedIndex)
	if len(rf.logs) > lastIncludedRelativeIndex &&
		rf.logs[lastIncludedRelativeIndex].Term == args.LastIncludedTerm {
		rf.logs = rf.logs[lastIncludedRelativeIndex:]
	} else {
		// 7. discard entire log
		rf.logs = []LogEntry{{Term: args.LastIncludedTerm, Command: nil}}
	}
	// 5. save snapshot file, discard any existing snapshot
	rf.snapshottedIndex = args.LastIncludedIndex
	// IMPORTANT: update commitIndex and lastApplied because after sync snapshot,
	// 						it has at least applied all logs before snapshottedIndex
	if rf.commitIndex < rf.snapshottedIndex {
		rf.commitIndex = rf.snapshottedIndex
	}
	if rf.lastApplied < rf.snapshottedIndex {
		rf.lastApplied = rf.snapshottedIndex
	}

	rf.persister.SaveStateAndSnapshot(rf.encodeRaftState(), args.Data)

	if rf.lastApplied > rf.snapshottedIndex {
		// snapshot is elder than kv's db
		// if we install snapshot on kvserver, linearizability will break
		return
	}

	installSnapshotCommand := ApplyMsg{
		CommandIndex: rf.snapshottedIndex,
		Command:      "InstallSnapshot",
		CommandValid: false,
		CommandData:  rf.persister.ReadSnapshot(),
	}
	go func(msg ApplyMsg) {
		rf.applyCh <- msg
	}(installSnapshotCommand)
}

// invoke by Leader to sync snapshot with one follower
func (rf *Raft) syncSnapshotWith(server int) {
	rf.mu.Lock()
	if rf.state != Leader {
		rf.mu.Unlock()
		return
	}
	args := InstallSnapshotArgs{
		Term:              rf.currentTerm,
		LeaderId:          rf.me,
		LastIncludedIndex: rf.snapshottedIndex,
		LastIncludedTerm:  rf.logs[0].Term,
		Data:              rf.persister.ReadSnapshot(),
	}
	DPrintf("%v sync snapshot with server %d for index %d, last snapshotted = %d",
		rf, server, args.LastIncludedIndex, rf.snapshottedIndex)
	rf.mu.Unlock()

	var reply InstallSnapshotReply

	if rf.sendInstallSnapshot(server, &args, &reply) {
		rf.mu.Lock()
		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.convertTo(Follower)
			rf.persist()
		} else {
			if rf.matchIndex[server] < args.LastIncludedIndex {
				rf.matchIndex[server] = args.LastIncludedIndex
			}
			rf.nextIndex[server] = rf.matchIndex[server] + 1
		}
		rf.mu.Unlock()
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	term, isLeader = rf.GetState()
	if isLeader {
		rf.mu.Lock()
		index = rf.getAbsoluteLogIndex(len(rf.logs))
		rf.logs = append(rf.logs, LogEntry{Command: command, Term: term})
		rf.matchIndex[rf.me] = index
		rf.nextIndex[rf.me] = index + 1
		rf.persist()
		// start agreement now, note that this does not take too long
		// because every single RPC is in other goroutine
		rf.broadcastHeartbeat()
		rf.mu.Unlock()
	}

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.currentTerm = 0
	rf.votedFor = -1 // voted for no one
	rf.heartbeatTimer = time.NewTimer(HeartbeatInterval)
	rf.electionTimer = time.NewTimer(randTimeDuration(ElectionTimeoutLower, ElectionTimeoutUpper))
	rf.state = Follower

	rf.applyCh = applyCh
	rf.logs = make([]LogEntry, 1) // start from index 1

	// initialize from state persisted before a crash
	rf.mu.Lock()
	rf.readPersist(persister.ReadRaftState())
	rf.mu.Unlock()

	rf.nextIndex = make([]int, len(rf.peers))
	for i := range rf.nextIndex {
		// initialized to leader last log index + 1
		rf.nextIndex[i] = len(rf.logs)
	}
	rf.matchIndex = make([]int, len(rf.peers))

	go func(node *Raft) {
		for {
			select {
			case <-node.electionTimer.C:
				node.mu.Lock()
				// we know the timer has stopped now
				// no need to call Stop()
				node.electionTimer.Reset(randTimeDuration(ElectionTimeoutLower, ElectionTimeoutUpper))
				if node.state == Follower {
					// Raft::startElection() is called in conversion to Candidate
					node.convertTo(Candidate)
				} else {
					node.startElection()
				}
				node.mu.Unlock()

			case <-node.heartbeatTimer.C:
				node.mu.Lock()
				if node.state == Leader {
					node.broadcastHeartbeat()
					// we know the timer has stopped now
					// no need to call Stop()
					node.heartbeatTimer.Reset(HeartbeatInterval)
				}
				node.mu.Unlock()
			}
		}
	}(rf)

	return rf
}

func randTimeDuration(lower, upper time.Duration) time.Duration {
	num := rand.Int63n(upper.Nanoseconds()-lower.Nanoseconds()) + lower.Nanoseconds()
	return time.Duration(num) * time.Nanosecond
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C: //try to drain from the channel
		default:
		}
	}
	timer.Reset(d)
}
