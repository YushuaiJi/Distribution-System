## Safety
前面的章节已经描述了raft如何选举且如何复制log entries。

但是这不能保证每一个machine都是可以都有一样的指令，同时这些指令也有一样的order。

举个例子，一个follower可能unavailable当leader commit几个log entry的时候，接着这个follower可能被选举为新的leader，

这个时候他就有可能overwrite这些entry with new one。

所以这个章节添加了一些限制条件，这些限制保证了leader拥有原来所有committed的entries。

## 1 Electionaries Restriction


- 在任何leader-based一致性算法当中，leader肯定最后有所有的committed log entries。但在某一些算法当中。

比如Viewstamped Rexplication，leader可以不拥有所有的entries，也可以变成一个合法的leader。

但是在这种情况下，算法就要用额外的精力去把这些丢掉的entries“给”leader.Raft用了一个更加简单的方法来保证所有的entries

都在新leader当中（不用transfer）。这就是说log entries只会从leader到follower，且leader不会overwrite他们已经存在的

log。

- Raft利用vote process来确保那些没有“所有”committed entries的候选者无法参与竞选。


文中“所有”的意思是要有大部分entries就行了（很奇怪的描述）。


好的表达应该是选举这个candidate的servers的至少有一个是包含所有的log entries的。


这里candidate的log entries要和follower一样新，这样才能保证leader有所有的committed的log entries。


这里的操作是用RequestriaVote RPC实现的。 RPC包含candidate log的所有信息，这样投票的voter不会投给那些


太“旧”的candidate。


- Figure 8用一系列操作来表明在一系列操作以后，有一些老的entries及时被储存在大多数的servers中，也会被overwrite。

![IMG_0261](https://user-images.githubusercontent.com/52951960/96208071-4c583d00-0f9f-11eb-819f-afb6e2d11639.jpg)

1 s1是一个leader，复制了log entries在index2的时候


2 s1凉了，s5被选作leader(因为自己投票和s3,s4的投票),同时有了不一样的entry log


3 s5坏了，s1重新开始变为leader，继续复制。这时候term2已经复制给了大多数servers(3个了）,但是term2还没有committed。之后s1在(d)的时候


坏了，s5可以被作为leader(s2,s3,s4会投给他),这时候它会overwrite entry根据它自己的term(term3去覆盖别的server的不一样的term）


4 但是，如果s1在坏之前可以复制entry到其他的server，就像(e)一样，这样s5就不能获得选举，且变为leader。同时所有的preceding entries都也会committed。

## 2

一个leader是可以知道它的current term 是被committed的，当entry被储存在大多数的servers中。如果leader在committed这个entry之前坏了，未来的leader会尝试


复制好这个entry（即使它不能立刻意识到这个entry是被committed的且储存在大部分的servers当中)。为了排除这个问题（就是上吗Figure8的问题)


为了排除这个问题，raft不允许老的一些entries，成为committed entries通过“数数”replicas的方式（就是figure8的方式）; 反而“最新”的entries是可以通过通过

“数数”的方式来committed的，这样原来的一些entries也可以立马committed（由于log matching的性质，原来提过了）

## Safety Argument

我们利用反证法证明一个committed的entries会在更换leader之后还是继续存在的。


上图：
![IMG_0269](https://user-images.githubusercontent.com/52951960/96336208-aac11080-10b0-11eb-9049-768027ca7328.jpg)


那么我们假设一个entry AE被存储在term T当中，且同时已经被committed过了。但是在未来的一个leader当中，比如leaderU是没有的。（这里的U>T）的


1:这个AE一定要在leader U选举的时候是缺失的(leader从来不删除和重写entries）


2:leader T肯定是把这个entries复制到了许多的servers上，同时leader U收到的投票的servers中，至少有一个接受了这个entries。这个是导致矛盾的一个关键点


3:这个voter的servers必须是有这个committed entry的，如果它先投U的话，它是会拒绝AppendEntries(来自于leader T)（你要知道这样U > T,所以这个voter的current term也是新于

T的）


4:假设voter一直储存着这个entry。因为，假设中U是最小的不存在此log entry的leader，那么[T,U)之间的leader不会删除和覆盖自己的log entry且follower只会删除和leader冲突的log entry；


5: voter会保证他投向U，所以U会一致跟voter保持“最新”的状态。这样就会有两个矛盾。


6: vote和leader U肯定是share最后一个log term的，即最后一个term是一样的，这样U至少要跟voter一样长，所以它一定要包含voter的log，这就有了第一个矛盾。


7:如果最后一个term是不一样的，那么U的必定大于voter的，这就可以推算出leader U必须包含term T的所有日志，因为U > T；


8: 所以就可以利用反证法证明了结论，committed的entries会在更换leader之后还是继续存在的。

## Follower and candidate creashes


- follower崩溃掉后，会按如下处理


1 当发送的RequestVote和AppendEntries失败的时候，Raft会不断的重新尝试，直到成功。


2:如果servers crash的时候是已经完成RPC但还没有回复的时候，它会收到一样的RPC，但是它不会采取接受到的操作。


## Timing and availability

(未完待续）


