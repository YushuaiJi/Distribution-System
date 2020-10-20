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




