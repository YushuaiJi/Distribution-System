## Abstract

- Raft是一致性算法来管理replicated log,它可以产生类似于(multi-）Paxos的结果。但是它跟Paxos还是不太一样的，它具有更好的可读性。

为了这种可读性的增加，它分开了关键因素比如leader election，lop replication， and safety。

同时它增加了coherency度，同时减少了state数量。

它有几个关键的创新：

1：Strong Leader：raft使用了stronger的领导方式，比如log只能从leader到其他的servers（例如follower），这大大的简化了副本log的管理，

而且让raft更加容易的去理解。

2: Leader election: raft 使用了随机timers去选举leader，只有一小部分机制会使用heartbeat（为了保证一致性算法），这些很容易的解决了矛盾

3：Membership changes: Raft机制利用了joint consensus方法来配置更新每个servers的信息和作用。

## 1 Replicated State Machines
![IMG_0142(20200916-090113)](https://user-images.githubusercontent.com/52951960/93280160-8517c180-f7fb-11ea-8f3d-e02cff0cb943.PNG)

Replicated State Machines是多个state machine可以在同一个状态，即使某些服务器已经down机了。

RSM可以在分布式系统当中解决很多fault tolerance的问题。比如GFS,HDFS,以及RAMCloud。

1：每个server都会接受来自于client的信息，储存log（包含了一系列的命令，且这些命令是拥有相同顺序的）。所以每个state machine可以执行相同过程。

2：每个一致性module在服务器上为了保持一致性的性质，保持一样的order，他们会和其他的服务器进行交流，这样即使有一些servers坏了也不会破坏一致性结果。

3：一致性的算法的性质。

- 1 他们保证了safety，在所有non-Byzantine的情况下。包括network delay，partitions，和packet loss，duplication，和recording。

- 2 他们可以保证functional，只要大部分的机器还是可以和别的机器交流，且能和client交流的，例如五个cluster中有三个ok就能干活，servers可能会停止

工作而fail，但是他们会恢复并且参与到工作中。

- 3: 他们不依赖于时钟来保持log的一致性，因为错误的时钟或者极端消极的时钟可能会导致很多可用性的问题。

- 4：一个命令被算作完成，只要大部分的cluster回复RPC。小部分慢的RPC对全局没有影响。

2: What's wrong with Paxos 

- 难以理解，缺少细节，难以实现。

- Designing for understandability:算法分成leader election，lop replication, and safety + 日志不允许有空洞。

## Raft Basic 

一个leader管理log副本（log entry只能从leader到follower）

1 leader election：原来的leader挂掉以后可以选一个新leader+leader是不需要咨询其他server的决定而可以独自决定把log entry放在哪里。

2：log replication：leader从client中接受log，

3：Safety：任意一个servers将log entry放回到state machine中去，其他的server都会把相同的log entry放回去。

Raft允许多个server，五个是合理的server数量，因为挂了两个还可以继续工作。任何一个时间段server都处于以下三种状态：

1：Leader：leader可以处理所有来自client的请求（是client给followed联系，之后follower把client的请求发给leader）。

2：Follower：follower是消极的，他们有什么自己产生的request，仅仅作为leader和candidate的一种纽带。

3: candidate:被用作于选一个新的leader。

![IMG_0143(20200917-094645)](https://user-images.githubusercontent.com/52951960/93410127-fe321a00-f8ca-11ea-89f6-e72f8ebf6691.PNG)

图片展示了大体过程。

![IMG_0143(20200917-094645)](https://user-images.githubusercontent.com/52951960/93410427-8e705f00-f8cb-11ea-9248-fd071c111ae5.PNG)

时间会被分成一段一段的，每个term都会开始选举，一个single leader会管理cluster直到这个term结束位置。一些election fail了，那么这个term就会没有leaders的结束。

这种term的transition可以在不同的时间不同的servers上观察到。

- Raft会把时间分割成一段一段term，每个term会开始election，而且大多数的candidate都尝试着变成leader。且如果一个candidate变成了leader，那么接下来的整个term它

都会是leaders。某一些时候election可能会出现没有leader的情况。这样会马上有一个新的term出现，raft要保证这里至少有leader在一个term里面。

- 不同的server会观察到transition在term之间，在不同的时期。当然也有可能某一些情况下servers不会观察到选举，在整个term之间。

- term在raft中扮演逻辑块，就是他们允许servers去观察删除一些老旧的信息，比如stale leader。

- 每个servers会储存current term number，而且这个是单调递增的。

- current term会交换在servers交流的时候。如果current term在交流信息的时候发现它的term比其他的term小，它就会更新自己的term变成更大的value。

- 如果一个candidate或者leader发现一个term已经out of date了，它会迅速的变成follower的状态。

- 如果一个servers接受到来自于一个stale term number，则它会拒绝这个请求。

- Raft servers会通过RPC来完成交流，一致性算法主要通过两种RPC来完成，第一种是RequestVote RPCs，第二种是AppendEntries RPCs。

## Leader Election

- Raft主要通过心跳来引发选主的。

- 当server启动的时候，状态是follower。当server从leader或者candidate接受到合法的RPC信息时，它会一直保持follower的状态（leader是通过周期的

发送心跳来证明是leader）但是follower当在选举timeout的时候还没有收到通知，这时候它就开始参与选主了。

Election的具体步骤

- 增加current term

- 转换成candidate state

-选自己为leader，然后保持选主的RPC并且并行的发送信息给其他的server

- candidate状态会持续下去直到有leader进行出来（或者在一定的时间内没有leader）

其中选出leader有自己成为leader，和其他server成为leaders。

以下开始讨论这三种具体情况

1：自己成为了leader。

在cluster的election当中，如果这个server成为了leader，那么它会通过心跳的方式告诉其他的servers，这样可以防止新的选举。

这里我们要明白选举的过程是一个servers只能选一个server成为leader，从而使得最后只有一个candidate变成leader。

2：其他的server变成了leader

如果在选举过程中，candidate接受来自别的servers要成为了leader的RPC，那么这时候还是情况处理。

- leader的term大于等于自身的term，那么candidate会转变成follower。。

- 反之，他会拒绝该leader，并保持自身的candidate的状态。

3：选举一段时间后没有leader。

- 如果出现很多follower同时变成candidate，可能导致没有一个candidate获得了大多数的选举，从而没法选出leader。这个时候，每一个candidate都会timeout。

然后增加一个新的term同时会进行新的一轮election通过增加term和初始化RequestVote RPCs。但是这里要明白，如果没有手段去处理，可能会导致不断地重复选主这种情况。

- Raft利用了Randomized election timout来保证这种split vote出现的情况比较少且出现了也可以进行解决。每个candidate选择一个fixed interval（150ms-300ms）之间，采用这种机制

一般只有一个server变成candidate，然后获取大部分server的election，最后win且变成leader。每一个candidate在收到leader的心跳以后会重新启动定时器，这样可以有效防止有leader的情况

下还发生选举。

-








