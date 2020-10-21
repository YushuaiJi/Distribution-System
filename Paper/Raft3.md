## Log compaction
- log会随着正常操作会持续不断的增加，但是实际过程中log是不可能无限的增加的（因为会需要更多的空间和时间），这个会造成available problem，如果不抛弃这些过时的log的话。


- 快照是压缩log最简单的一个办法。在快照当中，整个系统都会被写到快照中，从而可以储蓄在稳定的storage当中。这时候到在快照中所有的log都会被抛弃，快照可以使用在chubby和zookeeper当中。

![IMG_0272](https://user-images.githubusercontent.com/52951960/96593835-88332f80-131c-11eb-8902-9fb5d79413e5.jpg)

- Figure12 就展示了快照在raft当中的作用。每个server都会独自的采取快照，大部分的工作就是state machine写他的current state到快照当中。raft也包括小部分的元数据也会储蓄到快照当中：


- 最后一个log entry的last included index和last included term。这样可以保证了AppendEntries的一致性检查，对于下一个snapchat的的第一个log entry而言（这是由于每一个entry需要


原来的log的index和term。


- 为了支持集群成员变更，快照中保存的元数据还会存储集群最新的配置信息，且当它完成了一个快照以后，它会删掉所有log entries直到最新的一个last included index，以及之前的快照。


- servers是独立快照的，但是leader也会偶尔的发送快照给follower(有些follower可能已经出现log延迟了(我的理解是没有同步快照操作，比leader慢))（不过这个不是正常的操作,大部分时候follower还是会


跟着leader一起同步操作的）但是，有些新加入的server或者比较慢的follower可能就需要这个操作。这种时候只能通过快照来发送。


- leader使用了new RPC 叫做installSnapshot去把snapchats发送给followers。


当一个follower接收到快照的时候，快照经常包含着接受者没有的信息，这个时候follower就会抛弃它的整个log。而且它可能有一些没有committed的entries会跟快照发生冲突

且同时会被快照所取代。


如果快照信息如果由于一些错误而发送给了follower，这个时候被covered掉的log entries就会被删除，但是剩余的log entries还是会有效的。


对于Raft快照，两个issue是需要考虑的：

-  server何时做快照，太频繁地做快照会浪费带宽和能量；如果过于不频繁会导致server down掉后在restart的时间增加，可行的方案为当日志大小到一定空间时，开始快照。备注：如果所有server做快照的阈值空间都是一样的，但是快照点


不一定相同，因为，当server检测到日志超过大小，到其真正开始做快照中间还存在时间间隔，每个server的间隔可能不一样。



- 写快照花费的时间很长，不能让其delay正常的操作。可以采用copy-on-write操作（例如linux的fork）


## Client Interaction

client会发送他们的请求给leader，client刚开始的时候它会随机连接一个server。如果client第一个选择如果不是leader，server会拒绝client的请求且会把这个得到的请求会发送给最近的一个

leader(AppendEntries的请求会包括leader的地址）如果leader crash了的话，client会又开始重新随机选择servers。


raft的目标就是linearizable semantics。


- 每一个操作都是毫秒级别的，举个例子，如果leader crash了在commit log entry和回复给client之间，这个时候client可能会叫新的leader重新尝试，会造成一个指令的两次实行。


- 解决的办法也不难，每一个client会给每一个指令独特的序列号。进而每一个state machine会检验指令是不是最新的序列号。如果接收者接到的信号是已经实行过的，那它不会execute这个请求。


- 只读的请求可以不写进log就能执行，但是它有可能返回stale的数据，因为老的leader挂掉了，但它自身意识到自己已经不是leader了，于是client发读请求到该server时，可能获得的是老数据


- 这里raft做了如下两个操作来避免以上可能发生的错误：


1:leader完备性特征要求所有的log entry是已经committed的了。在正常情况下，leader一直是有已committed log entry的。但是，在leader刚当选的时候，他是不知道哪些是哪些不是的，这里通过提交一个


no-op(应该是no operation)的log entry来获取已提交过的log entry（为了避免commiting from previous leader).


2: leader在执行只读请求时，需要确定自己是否还是leader，通过和大多数的server发送heartbeat消息，来确定自己是leader，然后再决定是否执行该请求.

## Implementation and Evalution

![IMG_0273](https://user-images.githubusercontent.com/52951960/96734596-48884880-13ed-11eb-80a0-ea82e6bab73f.jpg)

- 从上图的第一幅图，可以看出：

5ms的随机范围，可以将downtime减少到平均283ms。


50ms的随机范围，可以将最坏的downtime减少到513ms。


- 从第二幅图，可以看出：


可以通过减少electionTimeout来减少downtime。


过小的electionTimeout可能会造成leader的心跳没有发给其他server前，其他server就开始选举了，造成不必要的leader的切换，一般建议范围为[150ms-300ms]。
