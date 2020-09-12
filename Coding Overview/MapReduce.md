MapReduce的golang实现版依托于MIT-6.824课程[网址](https://pdos.csail.mit.edu/6.824/schedule.html)
并且吸取了很多原有人对MIT-6.824的理解写出了此篇MapReduce Overview。

# MapReduce简介
MapReduce是一个主要用于处理大量数据的集的一种programming model，它主要用了map function来处理记录key/value pairs，接着利用reduce function聚合压缩key/value pairs.
其理论内容可以参考[link](https://github.com/YushuaiJi/DIstribution-System/blob/master/Paper/MapReduce.md)


## MapReduce实现基本过程

### 1串行实现MapReduce

MapReduce有串行和并行运行的两种方式，其中实际运行过程是以并行的形式进行了，运用了Golang轻线程的方式进行MapReduce，但在debug过程当中串行能发挥非常好的作用。

上一个串行的运行方式的coding

```Go
func Sequential(jobName string, files []string, nreduce int,
	mapF func(string, string) []KeyValue,
	reduceF func(string, []string) string,
) (mr *Master) {
	mr = newMaster("master")
	go mr.run(jobName, files, nreduce, func(phase jobPhase) {
		switch phase {
		case mapPhase:
			for i, f := range mr.files {
				doMap(mr.jobName, i, f, mr.nReduce, mapF)
			}
		case reducePhase:
			for i := 0; i < mr.nReduce; i++ {
				doReduce(mr.jobName, i, mergeName(mr.jobName, i), len(mr.files), reduceF)
			}
		}
	}, func() {
		mr.stats = []int{len(files) + nreduce}
	})
	return
}
```
其主要过程是: 建立master ---> 使用迭代器进行map和reduce ---> 输出结果

这里需要detail一点的是domap和doReduce两个函数
- 常用变量定义
Map和reduce之前看一眼常用的变量定义:[变量定义](https://github.com/YushuaiJi/DIstribution-System/blob/master/MIT-6.824/src/mapreduce/common.go)
特别是KeyValue，reduceName和mergeName，这三个变量的定义。

- Map的code:
```Go
// doMap 会读取其中一个需要map的文件(inFile), 且会根据用户定义的map function进行map
//会把partition结果同时产生nReduce中间文件.

func doMap(
	jobName string, // the name of the MapReduce job
	mapTaskNumber int, // which map task this is
	inFile string,
	nReduce int, // the number of reduce task that will be run ("R" in the paper)
	mapF func(file string, contents string) []KeyValue,
) {
//要看一看common.go KeyValue的定义。

//中间文件会储存在不同的文件夹中，后面的流程中每一个中间文件都要作为Reduce的任务。
  
//这里要注意的是文件名 其由map task number+the reduce task number一起产生
  
//(中间文件名是由jobName, mapTaskNumber, r) 作为reduce文件，且可以call ihash()。
  
//jobName string在map function也用不到。
  
//mapF() return一个slice，包含着key/value pairs

//Golang的ioutil and os packages可以读写文件  

// A data structure as a JSON string to a file using the commented
// code below. The corresponding decoding functions can be found in
// common_reduce.go.
//   enc := json.NewEncoder(file)
//   for _, kv := ... {
//     err := enc.Encode(&kv)	
  
  fmt.Printf("Map: job name = %s, input file = %s, map task id = %d, nReduce = %d\n",
		 jobName, inFile, mapTaskNumber, nReduce);
	*/
	// 读取输入文件
	bytes, err := ioutil.ReadFile(inFile)//ioutil 读这个文件的内容
	if (err != nil) {
		// log.Fatal() 打印输出并调用 exit(1)
		log.Fatal("Unable to read file: ", inFile)
	}

	// 解析输入文件为 {key,val} slice,且命名为kv_pairs
	kv_pairs := mapF(inFile, string(bytes))//key是 infile，val是infile 的content的string的slice，这里要明白内容是分开的
	//func ReadFile(filename string) ([]byte, error) 返回来的是一个slice

	// 生成一组 encoder的slice 用来将 {key,val} 保存至对应文件
	encoders := make([]*json.Encoder, nReduce);//长度为nReduce的slice
	for reduceTaskNumber := 0; reduceTaskNumber < nReduce; reduceTaskNumber++ {
		filename := reduceName(jobName, mapTaskNumber, reduceTaskNumber)
		// Create() 默认权限 0666
		file_ptr, err := os.Create(filename)
		if (err != nil) {
			log.Fatal("Unable to create file: ", filename)
		}
		defer file_ptr.Close()
		encoders[reduceTaskNumber] = json.NewEncoder(file_ptr);//每一个encoder都加上对应的文件夹
	}

	// 利用 encoder 将 {key,val} 写入对应的文件
	for _, key_val := range kv_pairs {
		key := key_val.Key
		reduce_idx := ihash(key) % nReduce//是把string转换成数字的大小
		err := encoders[reduce_idx].Encode(key_val)
		if (err != nil) {
			log.Fatal("Unable to write to file")
		}
	}
}
```

- doMap的函数
如何把Map函数利用好，以wordcount为例:
```Go
func mapF(filename string, contents string) []mapreduce.KeyValue {
	// TODO: you have to write this function
	// words := strings.Fields(contents)
	f := func(c rune) bool {
		return !unicode.IsLetter(c)
	}
	words := strings.FieldsFunc(contents, f)
	kv_slice := make([]mapreduce.KeyValue, 0, len(words))
	for _,word := range words {
		kv_slice = append(kv_slice, mapreduce.KeyValue{word, "1"})
	}
	return kv_slice
}
```

- doReduce函数
```Go
//doReduce会处理中间key/value对，之后按照key对pairs进行排序。
//根据doReduce函数进行reduce，之后储存到相应的文件中

func doReduce(
	jobName string, // the name of the whole MapReduce job
	reduceTaskNumber int, // which reduce task this is
	outFile string, // write the output here
	nMap int, // the number of map tasks that were run ("M" in the paper)
	reduceF func(key string, values []string) string,
) {
	
  //对于每一个key，我们都要利用reduceF()对其进行“聚合”操作
	// JSON -- 一种marshalling format.
	// enc := json.NewEncoder(file)
	// for key := ... {
	// 	enc.Encode(KeyValue{key, reduceF(...)})
	// }
	// file.Close()
	//

	// 查看参数
	/*
	fmt.Printf("Reduce: job name = %s, output file = %s, reduce task id = %d, nMap = %d\n",
		 jobName, outFile, reduceTaskNumber, nMap);
	*/

	kv_map := make(map[string]([]string))//hashmap储存的key为string，value是slice[]

	for mapTaskNumber := 0; mapTaskNumber < nMap; mapTaskNumber++ {
		filename := reduceName(jobName, mapTaskNumber, reduceTaskNumber)//需要reduce的file，已经在map里面create了
		f, err := os.Open(filename)
		if (err != nil) {
			log.Fatal("Unable to read from: ", filename)
		}
		defer f.Close()

		decoder := json.NewDecoder(f)
		var kv KeyValue
		for ; decoder.More(); {//就是一个loop
			err := decoder.Decode(&kv)
			if (err != nil) {
				log.Fatal("Json decode failed, ", err)
			}
			kv_map[kv.Key] = append(kv_map[kv.Key], kv.Value)//把数据放到hashmap当中
		}
	}

	keys := make([]string, 0,len(kv_map))//0定义len，len(kv_map)定义cap
	for k,_ := range kv_map {
		keys = append(keys, k)
	}
	sort.Strings(keys)//做了一个升序排序

	outf, err := os.Create(outFile)
	if (err != nil) {
		log.Fatal("Unable to create file: ", outFile)
	}
	defer outf.Close()
	encoder := json.NewEncoder(outf)
	for _,k := range keys {
		encoder.Encode(KeyValue{k, reduceF(k, kv_map[k])})//把每一个key的多少个（就是value）找出来。并reduce一下。
	}
}
```

- reduceF的code

```Go
func reduceF(key string, values []string) string {
	// TODO: you also have to write this function
	var sum int
	for _,str := range values {
		i, err := strconv.Atoi(str)
		if (err != nil) {
			log.Fatal("Unable to convert ", str, " to int")
		}
		sum += i
	}
	return strconv.Itoa(sum)
}
```
这样就很好的实现了一个串行的MapReduce。


### 2 并行的MapReduce

首先做并行进行分布式的MapReduce，一定要了解携程，线程，轻线程等一些问题的处理.
可以看看我推荐的一些内容[thread](https://github.com/YushuaiJi/DIstribution-System/blob/master/Thread/基础知识(Go).md)

首先直接上整个过程是如何实现的
```Go
func Distributed(jobName string, files []string, nreduce int, master string) (mr *Master) {
	mr = newMaster(master)
	mr.startRPCServer()
	go mr.run(jobName, files, nreduce,
		func(phase jobPhase) {
			ch := make(chan string)
			go mr.forwardRegistrations(ch)
			schedule(mr.jobName, mr.files, mr.nReduce, phase, ch)
		},
		func() {
			mr.stats = mr.killWorkers()
			mr.stopRPCServer()
		})
	return
}
```
大体过程就是 建立一个master --->开始启用RPC ---> 进行并行的map和reduce ---> 最后把worker程序都停止了 ---> 停止RPC

如果不理解RPC基本机制，可以阅读[RPC基本原理](https://golang.org/pkg/net/rpc/)

mr.run其实就是schedule函数外面加一个套层，这里每次任务都会schedule两次，一次是用于map，另一次是用于reduce。

`schedule()` 通过 `registerChan` 参数获取 Workers 信息，它会生成一个包含 Worker 的 RPC 地址的 string，
有些 Worker 在调用 `schedule()` 之前就存在了，有的在调用的时候产生，他们都会出现在 `registerChan` 中。

`schedule()` 通过发送 `Worker.DoTask` RPC 调度 Worker 执行任务，可以用 `mapreduce/common_rpc.go` 中的 `call()` 函数发送。
`call()` 的第一个参数是 Worker 的地址，可以从 `registerChan` 获取，
第二个参数是 `"Worker.DoTask"` 字符串，第三个参数是 `DoTaskArgs` 结构体的指针，最后一个参数为 `nil`。
run的code:
```Go
func (mr *Master) run(jobName string, files []string, nreduce int,
	schedule func(phase jobPhase),
	finish func(),
) {
	mr.jobName = jobName
	mr.files = files
	mr.nReduce = nreduce

	fmt.Printf("%s: Starting Map/Reduce task %s\n", mr.address, mr.jobName)

	schedule(mapPhase)
	schedule(reducePhase)
	finish()
	mr.merge()

	fmt.Printf("%s: Map/Reduce task completed\n", mr.address)

	mr.doneChannel <- true
}
```
schedule的code;
```Go
func schedule(jobName string, mapFiles []string, nReduce int, phase jobPhase, registerChan chan string) {
	var ntasks int
	var n_other int // number of inputs (for reduce) or outputs (for map)
	switch phase {
	case mapPhase:
		ntasks = len(mapFiles)
		n_other = nReduce
	case reducePhase:
		ntasks = nReduce
		n_other = len(mapFiles)
	}
  //选择是进行map还是进行reduce
	fmt.Printf("Schedule: %v %v tasks (%d I/Os)\n", ntasks, phase, n_other)

	// All ntasks tasks have to be scheduled on workers, and only once all of
	// them have been completed successfully should the function return.
	// Remember that workers may fail, and that any given worker may finish
	// multiple tasks.
	//
	// TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO
	//

	// Part III code
	var wait_group sync.WaitGroup;

	for i := 0; i < ntasks; i++ {
		wait_group.Add(1)
		var taskArgs DoTaskArgs
		taskArgs.JobName = jobName
		taskArgs.Phase = phase
		taskArgs.NumOtherPhase = n_other
		taskArgs.TaskNumber = i
		if (phase == mapPhase) {
			taskArgs.File = mapFiles[i]
		}
		go func() {
			// fmt.Printf("Now: %dth task\n", task_id)
			defer wait_group.Done()
			worker := <-registerChan
			if (call(worker, "Worker.DoTask", &taskArgs, nil) != true) {
				log.Fatal("RPC call error, exit")
			}
			// 将 worker 放回再利用
			go func() {registerChan <- worker}()
		}()
	}
	wait_group.Wait()
	fmt.Printf("Schedule: %v phase done\n", phase)
}
```

 ## worker failures
 
- 如果一个 Worker fails，Master 交给它的任何任务都会失败。
- Master 可以把任务交给另一个 Worker。
- 一个 RPC 出错并不一定表示 Worker 没有执行任务，有可能只是 reply 丢失了，或是 Master 的 RPC 超时了。
- 因此，有可能两个 Worker 都完成了同一个任务。同样的任务会生成同样的结果，所以这样并不会引发什么问题。
- 在该 lab 中，每个 Task 都是序列执行的，这就保证了结果的整体性。
- 加入无限循环使得在 call 返回 false 的时候另选一个 worker 重试，返回 true 的时候将 worker 放回 ch，跳出循环。
```Go
...
		go func() {
			// fmt.Printf("Now: %dth task\n", task_id)
			defer wait_group.Done()
			// 加入无限循环，只要任务没完成，就换个 worker 执行
			for {
				worker := <-registerChan
				if (call(worker, "Worker.DoTask", &taskArgs, nil) == true) {
					// 非常关键，完成后再将 worker 放回
					go func() {registerChan <- worker}()
					break
				}
			}
		}()
...
```
这样就完成了基本的MapReduce
