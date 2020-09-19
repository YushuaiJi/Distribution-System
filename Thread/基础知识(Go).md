## Go的threads的特性。


### 1 time.sleep
Code:

```
func main() {
	time.Sleep(1 * time.Second)
	println("started")
	go periodic()
}

func periodic() {
	for {
		println("tick")
		time.Sleep(1 * time.Second)
	}
}
```
- 分析：
1 输出的结果将是”started“。或者是”started“ ”tick“。可能性有多种。

2 因为main func中并不是按照串行的形式运行code的

3 所以main可能等不到periodic运行，就结束了这个进程。

4 解决这个问题，有许多种办法。其一就是加上time.sleep。给periodic一些时间。

如下;
```
func main() {
	time.Sleep(1 * time.Second)
	println("started")
	go periodic()
	time.Sleep(5 * time.Second) // wait for a while so we can observe what ticker does
}

func periodic() {
	for {
		println("tick")
		time.Sleep(1 * time.Second)
	}
}

```

这样就能正常的运行出periodic中的println("tick")。

如果现在再加一段code，println另外一些文字，且同时要保持顺序。

例如想输出：
started
tick
tick
tick
tick
tick
canceled
这段文字。

- 错误示例
code:
```
func main() {
	time.Sleep(1 * time.Second)
	println("started")
	go periodic()
	time.Sleep(5 * time.Second) // wait for a while so we can observe what ticker does
	println("cancelled")
	time.Sleep(3 * time.Second) // observe no output
}

func periodic() {
	for {
		println("tick")
		time.Sleep(1 * time.Second)
	}
}
```
- 这里输出的结果不能保证cancelled就在tick的后面的。
- 如果要保证cancelled就在tick后面，则需要按照下列方式写：

code：
```
var done bool
var mu sync.Mutex

func main() {
	time.Sleep(1 * time.Second)
	println("started")
	go periodic()
	time.Sleep(5 * time.Second) // wait for a while so we can observe what ticker does
	mu.Lock()//不能忘记加锁，心里一定要记住，有数据改动的地方就要加锁
	done = true
	mu.Unlock()//不能忘记解锁
	println("cancelled")
	time.Sleep(3 * time.Second) // observe no output
}

func periodic() {
	for {
		println("tick")
		time.Sleep(1 * time.Second)
		mu.Lock()
		if done {
			return
		}
		mu.Unlock()
	}
}
```
- 这样可以通过一个锁的机制来确保Goroutines的运行中数据不会出现。


## Goroutines的理解
code:
```
func main() {
	var wg sync.WaitGroup
	for i := 0; i < 7; i++ {
		wg.Add(1)
		go func() {
			sendRPC(i)
			wg.Done()
		}()
	}
	wg.Wait()
}

func sendRPC(i int) {
	println(i)
}
```
- 这里就有明显的一个bug，就是go fun子进程时，你会发现输入的i可能已经发生变化了。从而导致输出的i不是按照我们需要的顺序输出来的。

改进方法为:
```
func main() {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(x int) {
			sendRPC(x)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func sendRPC(i int) {
	println(i)
}
```
- 这样就可以达到合理的效果了。
- 这里一定要明白的是sync.WaitGroup的作用是:
A WaitGroup waits for a collection of goroutines to finish. 

The main goroutine calls Add to set the number of goroutines to wait for. 

Then each of the goroutines runs and calls Done when finished. At the same time, Wait can be used to block until all goroutines have finished.

- 主要作用是等待全部的goroutines完成，同时主goroutines会添加一些子goroutines，且同时等待他们完成。这里的wait可以block，当所有的goroutines完成时。


## 锁的机制
- 聊一聊锁，锁的主要作用是在goroutines中要对数据进行read,write的时候确保数据***的作用，防止出现一些****的作用
code:
```
func main() {
	counter := 0
	for i := 0; i < 1000; i++ {
		go func() {
			counter = counter + 1
		}()
	}

	time.Sleep(1 * time.Second)
	println(counter)
}
```
- 如果没有锁的话，就会出现counter的println的结果可能是或大或小的结果。

心中必须要牢记一条，有数据的read和write就要加锁

修改如下
code:
```
func main() {
	counter := 0
	var mu sync.Mutex
	for i := 0; i < 1000; i++ {
		go func() {
			mu.Lock()
			defer mu.Unlock()
			counter = counter + 1
		}()
	}

	time.Sleep(1 * time.Second)
	mu.Lock()
	println(counter)
	mu.Unlock()
}
```
- 这样就可以合理的打出1000这个结果了

再看一个例子：

code：
```
func main() {
	alice := 10000
	bob := 10000
	var mu sync.Mutex

	total := alice + bob

	go func() {
		for i := 0; i < 1000; i++ {
		        //code 1
			mu.Lock()
			alice -= 1
			mu.Unlock()
                        //code 2
			mu.Lock()
			bob += 1
			mu.Unlock()
		}
	}()
	go func() {
		for i := 0; i < 1000; i++ {
			mu.Lock()
			bob -= 1
			mu.Unlock()

			mu.Lock()
			alice += 1
			mu.Unlock()
		}
	}()

	start := time.Now()
	for time.Since(start) < 1*time.Second {
		mu.Lock()
		if alice+bob != total {
			fmt.Printf("observed violation, alice = %v, bob = %v, sum = %v\n", alice, bob, alice+bob)
		}
		mu.Unlock()
	}
}
```

- 结果会不断出现observed violation。。。这段话，因为在运行code1和code2过程中，会有新的goroutines在运行，所以在println的过程中，不一定code1和code2已经运行完成了，所以会出现不相等的状况
code：
```
mu.Lock()
alice -= 1
bob += 1
mu.Unlock()
```

- 改成这样的模式就可以顺利的运行了。

## channel
- channel如何定义的例子
- channel是一种管道，主要连接两个goroutines的，并传递信息的。

- 1 unbuffered channel

```
func main() {
	c := make(chan bool)
	go func() {
		time.Sleep(1 * time.Second)
		<-c
	}()
	start := time.Now()
	c <- true // blocks until other goroutine receives
	fmt.Printf("send took %v\n", time.Since(start))
}
```
- 这个就是unbuffered channel， go func() channel中收到了true的信息，它才继续运行。

- 2 buffered channel

```
func main() {
	c := make(chan bool, 1)
	go func() {
		time.Sleep(1 * time.Second)
		<-c
	}()
	start := time.Now()
	c <- true
	fmt.Printf("send took %v\n", time.Since(start))

	start = time.Now()
	c <- true
	fmt.Printf("send took %v\n", time.Since(start))
}
```
- 这个就是有管道的channel的例子。

首先一定要拒绝deadlock！

```
func main() {
	c := make(chan bool)
	c <- true
	<-c
}
```
- 这个就是deadlock的明显例子，当true传到channel当中的时候，因为是没有储存空间的（无buffer的），所以要等到channel的receiver接收到channel的信息，这个Goroutines才不会继续block

下列写一个正常的code;

```
import "time"
import "math/rand"

func main() {
	c := make(chan int)

	for i := 0; i < 4; i++ {
		go doWork(c)
	}

	for {
		v := <-c
		println(v)
	}
}

func doWork(c chan int) {
	for {
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		c <- rand.Int()
	}
}
```
- 这个就可以应用的channel。

