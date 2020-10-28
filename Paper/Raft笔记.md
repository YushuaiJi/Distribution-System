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

##为什么是log



