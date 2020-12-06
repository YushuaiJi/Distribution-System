## raft3

Raft 的持久化(如果仅将状态保存在内存中，如果 server 挂了那就凉了)。 (在实际的系统中，每次改变 Raft 状态后我们会将其存在硬盘上，并且在重启时读取。)

lab中，Persister 结构体代替硬盘，调用 Raft.Maker() 时需要提供一个 Persister， Raft 会利用它进行初始化(Persister.ReadRaftState())。

并在状态改变时保存状态(Persister.SaveRaftState())。

做的主要两个点：

- 完成持久化相关的内容

- 完成log同步的加速

## 如何做到持久化

readPersist() 和 persist() 两个func基本按照6.824的exmaple就成。 
 
值得关注的是log， votedFor 和 currentTerm三个参数。

调用 readPersist() 的地方就在 Make() 里，已经写好了。但是注意需要加个锁。

调用 persist() 的地方需要认真思考。这里有两种方案:(在课程的疑难杂症里面写的挺清楚的）

- 为上述三个变量加一个 setter，并在 setter 中调用 persist() ----->简单无脑，但是会多meanless的 persist()，在实际系统中由于硬盘速度慢，会极大影响 Raft 性能

## 哪里需要加哪里 ----->(这样就更好!)

- 向 Leader 添加一个新的需要同步的 LogEntry 时

- Leader 处理 AppendEntries 的回复，并且需要改变自身 term 时

- Candidate 处理 RequestVote 的回复，并且需要改变自身 term 时

- Receiver 处理完 AppendEntries 或者 RequestVote 时

# 加速 log 同步怎么做

这部分建议参考 Raft students guide。 没做这个优化前，每个 AppendEntries RPC 只能检查一个 index。做了之后可以检查一个 term。

这个优化的核心思想就是让 AppendEntries 的 reply 能提供一些有用的信息给 Leader， 这样 Leader 就不用以最保守的方式（每次 decrement prevLogIndex）来尝试下一次同步 log(这个优化在

我的[raft3](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/Raft3.md))写了。

=为 reply 增加两个信息。

```Go
type AppendEntriesReply struct {
	Term    int  // 2A
	Success bool // 2A

	// OPTIMIZE: see thesis section 5.3
	ConflictTerm  int // 2C
	ConflictIndex int // 2C
}
```

### ConflictTerm: Follower 与 Leader 的不一致 log 的 term
- 用于当 Leader 和 Follower 都拥有属于 ConflictTerm 的 LogEntry 时
- Leader 会根据 ConflictTerm 来推算下一次请求使用的 prevLogIndex
### ConflictIndex: Follower 希望 Leader 重新发送的 LogIndex
- 用于当 Leader 没有属于 ConflictTerm 的 LogEntry 时
- 不是简单的不一致的 log 的 index
- 下一次请求的 prevLogIndex = ConflictIndex - 1

### followers的问题有两种：
- 1 Follower 的 log 不够新，prevLogIndex 已经超出 log 长度	-1（本次检查并没有发现冲突的 term），那么下一次利用len(rf.logs) (希望从 Follower 的最后一个 index 开始下一次检查)。

- 2	Follower prevLogIndex 处存在 log	rf.logs[args.PrevLogIndex]. Term	Follower log 中属于ConflictTerm的第一个 index (期望下一次能检查之前的)。

```Go
// entries before args.PrevLogIndex might be unmatch
// return false and ask Leader to decrement PrevLogIndex
if len(rf.logs) < args.PrevLogIndex + 1 {
  reply.Success = false
  reply.Term = rf.currentTerm
  // optimistically thinks receiver's log matches with Leader's as a subset
  reply.ConflictIndex = len(rf.logs)
  // no conflict term
  reply.ConflictTerm = -1
  return
}

if rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
  reply.Success = false
  reply.Term = rf.currentTerm
  // receiver's log in certain term unmatches Leader's log
  reply.ConflictTerm = rf.logs[args.PrevLogIndex].Term

  // expecting Leader to check the former term
  // so set ConflictIndex to the first one of entries in ConflictTerm
  conflictIndex := args.PrevLogIndex
  // apparently, since rf.logs[0] are ensured to match among all servers
  // ConflictIndex must be > 0, safe to minus 1
  for rf.logs[conflictIndex - 1].Term == reply.ConflictTerm {
     conflictIndex--
  }
  reply.ConflictIndex = ConflictIndex
  return
}
```
对于 Leader 接收到请求的结果，也要分情况处理

Case	Description	nextIndex:
-1	Leader 的 log 中并不存在 ConflictTerm (包括 ConflictTerm = -1 的情况)	---> nextIndex:ConflictIndex
-2	Leader 的 log 中存在 ConflictTerm	--->nextIndex:Leader log 中属于 ConflictTerm 之后 的第一个 index（Follower 期望能检查 ConflictTerm 和 Leader 是否匹配).

```Go
// log unmatch, update nextIndex[server] for the next trial
rf.nextIndex[server] = reply.ConflictIndex

// if term found, override it to
// the first entry after entries in ConflictTerm
if reply.ConflictTerm != -1 {
  for i := args.PrevLogIndex; i >= 1; i-- {
    if rf.logs[i-1].Term == reply.ConflictTerm {
      // in next trial, check if log entries in ConflictTerm matches
      rf.nextIndex[server] = i
      break
    }
  }
}
```
这部分最容易纠结的点就是到底应该如何设置 nextIndex。归纳起来就是：

1: 优化以后，理想情况是一个 RPC 能够至少检验一个 Term 的 log。

2: Follower 在 prevLogIndex 处发现不匹配，设置好 ConflictTerm，同时 ConflictIndex 被设置为这个 ConflictTerm 的 第一个 log entry。

如果 Leader 中不存在 ConflictTerm 则会使用 ConflictIndex 来跳过这整个 ConflictTerm 的所有 log。

3:如果 Follower 返回的 ConflictTerm 在 Leader 的 log 中找不到， 说明这个 ConflictTerm 不会存在需要 replication 的 log。

对下一个 RPC 中的 prevLogIndex 的最好的猜测就是将 nextIndex 设置为 ConflictIndex，直接跳过 ConflictIndex。

4:如果 Follower 返回的 ConflictTerm 在 Leader 的 log 中找到， 说明我们还需要 replicate ConflictTerm 的某些 log，此时就不能使用 ConflictIndex 跳过，

而是将 nextIndex 设置为 Leader 属于 ConflictTerm 的 log 之后的 第一个 log， 这样使下一轮 prevLogIndex 能够从正确的 log 开始。
