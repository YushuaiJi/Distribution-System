# 1. Introduction
MapReduce是一个主要用于处理大量数据的集的一种programming model，它主要用了map function来处理记录key/value pairs，接着利用reduce function聚合压缩key/value pairs.

主要的文章思路
- 基本思路
- 如何应用
- 一些改进

# 2 基本思路
利用map function来处理最原始数据，让其形成（中间)kv_pairs(key/value pairs)，之后利用reduce function进行“压缩”，最终进行输出。
这里有几点要注意的是map function会将kv_pairs输入到reduce function之中，其中是以不断迭代的过程输入的（这样可以使reduce函数处理一些比内存大的数据）。

论文中有狠多例子，其中word_count例子比较容易理解，as follows:

```
map(String key, String value):
// key: document name
// value: document contents
for each word w in value:
	EmitIntermediate(w, "1");

reduce(String key, Iterator values):
// key: a word
// values: a list of counts
int result = 0;
for each v in values:
	result += ParseInt(v);
Emit(AsString(result));
```
这里的map function 会产生每个word和其occurrence，而reduce函数则会将这些occurence直接相加（比如每一个的word的occurance是“1”时，reduce会把所有的occurrence相加，比如形成最后的结果
emit某一些值，比如“6”。

这里我们有几点要注意的
- 这里的input和output都是以string的形式传出传入的
- map和reduce是函数功能可以as follows理解

```
map    (k1,v2)          ======>list(k2,v2)
reduce (k2, list(v2))   ======>list(v2)
```

论文中还给了很多的例子：
**Distributed Grep**

- 对于map，如果输入的行匹配到相应的pattern，则吐出这行
- 对于reduce，只把map吐出的行拷贝到输出中

**Count of URL Access Frequency**

- 对于map，处理web日志，生成(URL, 1)中间结果
- 对于reduce，计算相同URL的值，生成(URL, total count)结果

**Reverse Web-Link Graph**

- 对于map，吐出(target, source)中间结果，其中target是被source引用的URL
- 对于reduce，聚合相同target的source，吐出(target, list(source))

**Term-Vector per Host**

Term Vector指的是一篇文档中的(word, frequency)K/V对。

- 对于map，吐出(hostname, term vector)中间结果
- 对于reduce，聚合相同hostname的term vector，吐出最终(hostname, term vector)

**Inverted Index**

- 对于map，吐出一系列的(word, document ID)
- 对于reduce，对相同word，按照document ID排序进行聚合，吐出(word, list(document ID))

**Distributed Sort**

- 对于map，吐出(key, record)中间结果
- 对于reduce，把map的中间结果写入到结果文件中，这里不需要显式地排序，因为MapReduce会自动地排序，方便在reduce的时候进行聚合。

# 3. 应用
MapReduce的应用是可以多种多样的，比如可以形成一个share-memory的machine，或者比如可以形成大型的NUMA multi-processor,甚至可以做成大型集群的networked的机器。

论文的处理环境是：

- 双核X86系统+Linux系统+2-4GB内存。
- 100M或1000M带宽网卡
- 集群由大量机器组成（经常发生故障）
- 每台机器使用IDE磁盘，用GFS作为底层存储
- 使用一个调度系统来处理用户的任务


# 主要流程：
- Map function会把输入数据划分成M份，且这些数据是并行的被不同的机器处理。
- Reduce function按照partition function划分数据，例如hash(key) mod R，这里的partition function其实就是哈希表hash的过程，且这个partition function是可以由用户自己定义的。

![流程图](https://user-images.githubusercontent.com/52951960/92840818-237ae000-f414-11ea-8646-fe16cee71a00.jpg)

1. MapReduce库会把文件划分成范围在16MB-64MB之间的片段（大小可以通过参数调节），然后在culster的机器上开始（cluster的翻译应该是一群？）。

2. master的作用，master会分配任务给worker。总共有M个map的任务和R个reduce任务需要分配，同时master会选取 -->空闲的worker（已经做好原来的工作的worker）或新worker，然后分配一个map任务或者reduce任务。

3. worker在处理map的时候，会将文件进行split，同时生产key—value pairs，然后传递给Map函数，使其缓存在内存中。

4. map任务的中间kv_pairs会被周期性地写入到磁盘中，以partition function来分成R个部分。同时master会得知R个部分的磁盘地址，然后由它转发给
空余或新worker进行reduce。

5. Reduce worker接收到master发送的地址信息时，它会通过RPC来Map worker读取对应的数据。Reduce worker读取到了所有的数据时，他们会进行reduce（它先按照key来排序，方便“压缩”操作）。

6. reduce worker遍历排序好的中间结果，对于相同的key，把其所有数据传入到Reduce函数进行处理，生成最终的结果会被追加到结果文件中。

7. 当所有的map和reduce任务都完成时，master会唤醒用户程序，然后返回到用户程序空间执行用户代码。

这里一定要注意，成功以后，输出的结果还是R个文件，他们不会进行任何合并操作，而是会作为新的MapReduce或者分布式应用的数据。

# master的主要结构

master keep了以下信息

- 对每个map和reduce任务，记录了任务状态(idle,in-progress或completed),同时还记录了worker机器的信息（如果这个worker是空闲的话)
- master相当于一个导管，将map好的信息传递给reduce worker。

# 1 Fault Tolerance

**Worker Failure**

master采用ping的方式检测故障，如果一台worker机器在一定时间内没有响应，master就会标记这个worker已经坏了。

- 对于map worker做好的map任务此时就需要重新执行，因为计算结果是存储在map任务所在机器的本地磁盘上的

- 对于完成了的reduce任务则不需要重新执行，因为结果已经输出到GFS中

当一个map任务开始由A来执行，当A fail以后就由B来执行，做该任务的所有worker都会接受到这个新的变动消息。

MaReduce是可以接受大量的worker failure的，比如每分钟挂掉了80个worker，master还是可以重新分配任务同时启用idle worker来完成这些工作的。

**Master Failure**

通过周期的checkpoint来保存状态，master fail以后，可以回到最近checkpoint的状态。

由于任务master挂掉概率极小，所以没有必要用checkpoint，只需要让应用重试这次操作。

**Semantics in the Presence of Failure**

当用户提供的Map和Reduce函数的执行结果是确定的，那么最终的执行结果就是确定的。

当用户提供的执行结果不是确定的，那么最终结果也是不确定的，但是每个reduce任务产生的结果都是不确定的某次串行执行的结果。

- Locality

由于网络宽带是一种不稳定且稀缺的资源，所以论文利用了GFS来完成数据的储存。这样就可以减少对网络宽带的依赖，做到这一点
我们需要让每个worker尽量启用到各个GFS的副本上(GFS有三个副本)，这样保证了在本地磁盘的运行。如果这个不行的话，则可以找一个replica的机器来
输入相应的数据。

- Task Granularity

map function是分成M份的，reduce任务是分成R份的，假设中，M和R的值应该比worker总量大很多，这样有助于dynamic load balance同时可以加速恢复当一个worker挂掉的时候(当一台机器挂掉后，它的map任务可以分配给很多其他的机器执行)

因为master需要O(M+R)的空间来做schedule decision，需要存储O(M*R)的任务产生的结果位置信息（每个任务产生的结果位置信息大致需要一个字节）。

通常R的数量是自定义的，对M的划分是要保证一个分片的数据量大小大约是16-64M，我们希望R是一个比较小的数，比如M和R的值为 M = 200000，R = 5000，使用2000台worker机器。

- Backup Tasks

在某些时候，某一些机器会出现“stragglers”的现象，即他们完成任务的时间远远超过正常worker完成task的时间，这里的原因有很多

比如由于很多task在一个machine上操作，则由于competition for CPU,memory,,local disk，宽带等等设备有限的原因，导致了超长时间不能完成任务。

MapReduce采用的方案是

- MapReduce操作快执行完成的时候，master会对正在进行的任务的进行备份，从而产生备份任务。备份任务和源任务做的是同样的事情，只要其中一个任务执行完成，就认为该任务执行完成。

该机制在占有很少的计算资源的情况下，大大缩短了任务的执行时间，论文5.4中的program就列举了，当我们不利用“backup”任务的时候，我么比用“备份”任务的program多出44%的时间。


# 4. Refinements
本章节描述了一些原油基础上的优化和改进。

- Partitioning Function

map任务的中间结果按照partitioning function分成了R个部分，通常，默认的函数`hash(key) mod R`可以提供相对均衡的划分。但有时应用需要按照自己的需求的来划分，比如，当Key是URL时，用户可能希望相同host的URL划分到一起，方便处理。这时候，用户可以自己提供partitioning function，例如`hash(Hostname(url))`。

- Ordering Guarantees

对于reduce任务生成的结果，MapReduce保证其是按照Key排序的，方便reduce worker聚合结果，并且还有两个好处

- 按照key随机读性能较好
- 用户程序需要排序时会比较方便

- Combiner Function

在有些情况下，map任务生成的中间结果中key的重复度很高，会造成对应的reduce任务通信量比较大。例如，word count程序中，可能和the相关的单词量特别大，组成了很多的(the, 1)K/V对，这些都会推送到某个reduce任务，会造成该reduce任务通信量和计算量高于其他的reduce任务。解决的方法是

- 在map任务将数据发送到网络前，通过提供一个`combiner`函数，先把数据做聚合，以减少数据在网络上的传输量


- Input and Output Types

MapReduce提供多种读写格式的支持，例如，文件中的偏移和行内容组成K/V对。

用户也可以自定义读写格式的解析，实现对应的接口即可。

- Side-effects

MapReduce允许用户程序生成辅助的输出文件，其原子性依赖于应用的实现。

- Skipping Bad Records

有时候，可能用户程序有bug，导致任务在解析某些记录的时候会崩溃。普通的做法是修复用户程序的bug，但有时候，bug是来自第三方的库，无法修改源码。

MapReduce的做法是通过监控任务进程的segementation violation和bus error信号，一旦发生，把响应的记录发送到master，如果master发现某条记录失败次数大于1，它就会在下次执行的时候跳过该条记录。

- Local Execution

因为Map和Reduce任务是在分布式环境下执行的，要调试它们是非常困难的。MapReduce提供在本机串行化执行MapReduce的接口，方便用户调试。

- Status Information

master把内部的状态通过网页的方式展示出来，例如，计算的进度，包括，多少任务完成了，多少正在执行，输入的字节数，输出的中间结果，最终输出的字节数等；网页还包括每个任务的错误输出和标准输出，用户可以通过这些来判断计算需要的时间等；除此之外，还有worker失败的信息，方便排查问题。

- Counters

MapReduce libaray提供一个counter接口来记录各种事件发生的次数。

例如，word count用户想知道总共处理了多少大写单词，可以按照如下方式统计

```
Counter* uppercase;
uppercase = GetCounter("uppercase");

map(String name, String contents):
	for each word w in contents:
		if (IsCapitalized(w)):
			uppercase->Increment();
		EmitIntermediate(w, "1");
```

master通过ping-pong消息来拉取worker的count信息，当MapReduce操作完成时，count值会返回给用户程序，需要注意的是，重复执行的任务的count只会统计一次。

有些counter是MapReduce libaray内部自动维护的，例如，输入的K/V对数量，输出的K/V对数量等。

Counter机制有些时候非常有用，例如我们希望输入和输出的K/V数量是完全相同的，counter就是个非常好的选择。

# 引用内容：
1:https://github.com/chaozh/MIT-6.824
3:https://github.com/double-free/MIT6.824-2017-Chinese
3:https://github.com/Charles0429
4:https://static.googleusercontent.com/media/research.google.com/zh-CN//archive/mapreduce-osdi04.pdf



