## raft笔记

对于fault tolerant的系统：

1 Mapreduce的系统是复制估算通过了单独的master来组织的。

2 GFS是复制数据，且也是通过单独的master来组织选取优先级的

3 VMware FT复制serves但是是以来于test-and-set来选取优先级的，而且也都是全部依赖于单独项来决定重要决策的。

## 为什么split大脑会出现，他有什么坏处。

[C1,C2,S1,S2]

当C1只和S1联系的时候，可能有两种原因：

1：一个是S2 crash了，所以C1已经没有办法跟S2联系了。

2：一个是网络出了问题，所以C1是没有办法跟S2联系了。

这样也出现了新的问题，计算机无法区别出servers crash和network broken这两个问题，因为这两个问题导致的结果是一摸一样的。

这个解决的办法就是用人工去机房检查到底是什么东西出现了问题。

#新的解决办法：大多数投票。

1 需要奇数个.

2 2f+1，最多能容忍f个出现crash。

## raft基本的Overview

raft的基本构架（举例）: [clients ---> 3 replicas，k/v layers ---> state，raft layers + logs]

client指令的时间线：[C,L,F1,F2]

1： client会发出put/get的指令到leader的k/v layer。

2： leader把指令添加到log当中。

3： leader发出AppendEntries RPCs到followers。

4： followers会添加command到log当中。

5：Leader会等待“大部分”的回应（包括它自己）

项会“committed”如果大多数的服务器把指令项committed。committed意味着即使大部分服务器的这个指令项操作失败，它也不会被遗忘。

而是在下一个vote requests的时候，leader会进行log中的一系列操作，这样其他的服务器也会如此操作，所以这个指令即使以前失败了，但是后面还是会重新可以操作。

## 为什么是log

- 服务器为什么需要保持状态机，比如为什么 key/value DB是不够的？

- log是指令的顺序。

让每个服务器都承认单一的执行顺序+leader这样做可以保证每个follower都有一个统一的log。

- log储存指令直到committed为止。

- 储蓄指令是是为了以后leader要重新给follower发送信息准备的

- log储蓄是为了可持续的回复再重建以后。

serves的log是一致的吗？

不是，很多服务器有可能发生lag， 同时他们有些时候会有不一样的entry。

好消息是他们最终会一致的，raft的机制导致只有稳定的entry才可以被执行。

## raft的互动

rf.Start(command)(index，term，isleader）

Start()只有在leader的上面才有用。

Starts开始一个新的log entry增加到leader的log上。

leader发送AppendEntries RPCs

Starts() 回复 w/o 等待RPCs回复。

k/v 层的put()/get()必须等待commit，在applych上可能会坏掉如果服务器失去了leader的位置再committed之前指令可能就丢失。

client要重新发送 isleader：

如果这个服务器不是leader的话，client就要试一试别的term了。

- 为什么需要一个leader

保证所有的指令都是统一行径的。

- raft会给leader的顺序进行数字编号。

new leader --> 新的term

一个term最多一个leader，或者是没有leader。

这个编号可以给服务器跟随最新的leader，不会停止leader。

- 为什么raft同辈开始一个leader的选举。

当raft不能听见current leader(未完待续）

