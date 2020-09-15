# 分布式系统学习
分布式学习论文以及论文的实现
- 对论文进行了更加容易理解的翻译，对文章进行了更好梳理
- 依托于6.824的课程，对部分论文进行了golang实现
- 对部分6.824的课程重点内容进行了整理
本人非科班出身，如果有错误望指正

## 目录
- [x] **Golang Thread基础知识** [理论与代码实例](https://github.com/YushuaiJi/Distribution-System/blob/master/Thread/基础知识(Go).md)

先熟悉golang的threads的特性。

对Goroutines的理解。

锁的作用

channel的作用

- [x] **Mapreduce论文** [论文](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/MapReduce.md)

MapReduce是一个主要用于处理大量数据的集的一种programming model。

它主要用了map function来处理记录key/value pairs，接着利用reduce function聚合压缩key/value pairs.

这篇文章主要通过理解英文版的论文+整合网络资源+自我的理解来描述MapReduce的：

1基本思路
2如何应用
3必要的改进

- [x] **Mapreduce实现(Golang)** [MapReduce实现](https://github.com/YushuaiJi/Distribution-System/blob/master/Coding%20Overview/MapReduce.md)

该篇在结合86.824课程的内容，以其框架内容为依托，用Go实现了MapReduce，主要实现了串行，并行实现MapReduce，同时写了如何处理Woker failure的方法。

- [x] **GFS论文** [论文](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/GFS.md)

GFS是一种scalable的分布式文件系统，其主要用处是管理数据。

同时它在廉价的商用硬件的情况下可以有很好的fault tolerance，并且可以给大量用户提供服务。
- [x] **Fault-Tolerant Virtual Machines** [论文](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/Raft1.md)
- [x] **Raft1** [论文1](https://github.com/YushuaiJi/Distribution-System/blob/master/Paper/Raft1.md)|[论文2](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/MapReduce)|[论文3](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/MapReduce)
- [x] **Raft1实现(Golang)** [Raft1实现](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/Raft1.md)
- [x] **Raft2实现(Golang)** [Raft2实现](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/Raft2.md)
- [x] **Raft3实现(Golang)** [Raft3实现](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/Raft3.md)

