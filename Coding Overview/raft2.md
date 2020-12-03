## Log Replication

log的复制原理我们已经在[raft2](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/Raft1.md)中讲过了。

首先在讲这个之前，我们得先说明下committed是怎么做到的

- 当一个 log entry 被 Leader 复制到了 大部分的 servers 上之后， 这个 log entry 就被称为 committed。

```Go
func (rf *Raft) setCommitIndex(commitIndex int) {
	rf.commitIndex = commitIndex
	// apply all entries between lastApplied and committed
	// should be called after commitIndex updated
	if rf.commitIndex > rf.lastApplied {//上面一个已经committed的到现在committed的中间这个部分需要committed。
		DPrintf("%v apply from index %d to %d", rf, rf.lastApplied+1, rf.commitIndex)
		entriesToApply := append([]LogEntry{},
			rf.logs[rf.getRelativeLogIndex(rf.lastApplied+1):rf.getRelativeLogIndex(rf.commitIndex+1)]...)//getRelativeLogIndex这个func主要是在snapshot到现在这个
      //index之间的logentry都append到committed的行列里面去。
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
```
- Election restriction

在所有 Leader-Follower 一致性算法中，Leader 最终都必须存储所有的 committed entries。 Raft 保证了 所有来自过去的 term 的 committed entries 在新的 Leader 选出的时刻就已经存在与 Leader 的 logs 中。 其机制为在选举阶段，如果一个 Candidate 的 log 不 “up-to-date”，则不会当选。

所谓 ”up-to-date“ 是指比大部分的节点的 log 更新，这就保证其包含所有的 committed entries。 Raft 通过比较最后一个 entry 的 index 和 term 来判断谁更新。

- 谁的 term 更大谁更新。
- term 一致时，谁的 log 更长谁更新。

首先上一个request vote的code：

```Go
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
```
一个candidate对于另外一个candidate，要求他选票的时候，如果其term小于另一个的term，那么选票直接失败。或者另外那个candidate要么直接变成不能选票（有两种情况）

这里一定要明白如果，requestvote的rf如果term没有args的新的话，则需要转换成follower的形态，同时此时要resettimer，且reply.Term = rf.currentTerm。（5.1）.

其他的详细代码请参考raft里面的结构。
