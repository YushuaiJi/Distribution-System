# 分布式系统学习
分布式学习论文以及论文的实现
- 对论文进行了更加容易理解的翻译，对文章进行了更好梳理
- 依托于6.824的课程，对部分论文进行了golang实现
- 对部分6.824的课程重点内容进行了整理
本人非科班出身，如果有错误望指正

channel的作用
Module|Abstract
---|---
[Golang Thread基础知识](https://github.com/YushuaiJi/Distribution-System/blob/master/Thread/基础知识(Go).md)<br> | <ul><li>1:先熟悉golang的threads的特性。 2: 对Goroutines的理解。<li>3:锁的作用. <li> 4:channel的作用。
[Mapreduce论文](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/MapReduce.md)<br> | <ul><li>1: MapReduce是一个主要用于处理大量数据的集的一种programming model。 2: 它主要用了map function来处理记录key/value pairs，接着利用reduce function聚合压缩key/value pairs.<li>3:这篇文章主要通过理解英文版的论文+整合网络资源+自我的理解来描述MapReduce的。<li> 4: 1基本思路+2如何应用+3必要的改进。
[GFS论文](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/GFS.md)<br> | <ul><li>1:GFS是一种scalable的分布式文件系统，其主要用处是管理数据。
[Raft1](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/Raft1.md)<br> | <ul><li> 1 Raft是一致性算法来管理replicated log,它可以产生类似于(multi-）Paxos的结果。 2: 它跟Paxos还是不太一样的，它具有更好的可读性。<li>3:它分开了关键因素比如leader election，lop replication， and safety。<li> 4: 它增加了coherency度，同时减少了state数量。
 
 
 ## 代码实现
- [x] **MapReduce** [MapReduce实现](https://github.com/YushuaiJi/Distribution-System/blob/master/Coding%20Overview/MapReduce.md)

- 1 该篇在结合86.824课程的内容，以其框架内容为依托，用Go实现了MapReduce.
- 2 主要实现了串行，并行实现MapReduce，同时写了如何处理Woker failure的方法。
- 3 同时它在廉价的商用硬件的情况下可以有很好的fault tolerance，并且可以给大量用户提供服务。

- [x] **Raft1实现(Golang)** [Raft1实现](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/Raft1.md)| [Raft2实现](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/Raft2.md)| [Raft3实现](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/Raft3.md)


## 待完成
- [x] **Fault-Tolerant Virtual Machines** [论文](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/Raft1.md)
