# goroutine：并发不是免费的午餐

你好，我是汪小成。你有没有写过这样的代码：来一个请求就开一个 goroutine，来一万个就开一万个；或者为了“提升性能”，把 for 循环里的工作全部 `go func(){...}()`，结果线上内存飙升、延迟抖动、偶发超时。goroutine 很轻，但**不等于免费**：它需要栈空间、调度、上下文切换，还会放大共享资源的争用。本文用一个“并发任务执行器”的小实验，把 goroutine 的收益与成本讲清楚，并给出可落地的工程写法：什么时候该并发、并发多少、怎么避免 goroutine 泄漏。

## 目录

1. 环境准备 / 前提知识  
2. 核心概念解释（概念 → 示例 → 为什么这么设计）  
3. 完整代码示例（可运行）  
4. 运行效果 + 截图描述  
5. 常见坑 & 解决方案（必写）  
6. 进阶扩展 / 思考题  

## 1. 环境准备 / 前提知识

### 1.1 版本与目录

- Go 1.22+（仓库根目录使用 `go.work`）。
- 本篇目录：`series/19`。
- 示例入口：`series/19/cmd/gorun/main.go`。

### 1.2 运行命令

```bash
# 顺序执行（对照组）
go run ./series/19/cmd/gorun -mode=seq -n=60 -sleep=10ms

# 有界并发（推荐）
go run ./series/19/cmd/gorun -mode=bounded -n=60 -c=10 -sleep=10ms

# 演示“停着不退出”的 goroutine（泄漏感知）
go run ./series/19/cmd/gorun -mode=leak -leak=3000
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/19/cmd/gorun -mode=bounded -n=60 -c=10 -sleep=10ms
```

### 1.3 前置知识

- 了解 `time.Duration`、`time.Sleep`（第 18 篇）。
- 知道 `sync.WaitGroup`、channel 的基本用法（后续章节会更深入，但本篇只用最基础的写法）。
- 理解“并发 vs 并行”的差别：并发是结构，并行是同时执行。

配图建议：
- 一张“并发 vs 并行”示意图（多个任务交错 vs 同时）。
- 一张“有界并发”的示意图（任务队列 → N 个 worker）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 goroutine 到底是什么

**概念**：goroutine 是 Go 运行时调度的轻量执行单元。它不像 OS 线程那样昂贵，但也有成本：启动、调度、栈、同步。  
**示例**：你用 `go f()` 启动一个任务，调度器会把它放到队列里，按需要在多个线程上运行。  
**为什么这么设计**：让你用很低的语法成本表达并发结构，同时把“线程管理”交给运行时处理。

### 2.2 并发不是免费的：成本来自哪里

你常见到的成本主要有三类：

1. **创建与调度**：启动 goroutine、唤醒/挂起、排队，都需要运行时工作。
2. **内存与栈**：每个 goroutine 都有栈（会增长），数量大了就是显著内存。
3. **资源争用**：并发越高，锁竞争、队列竞争、下游限流越明显，延迟会抖。

这也是为什么“开得越多越快”常常是错觉：当你把并发度推到超出 CPU/IO/下游能力时，收益会下降甚至反噬。

### 2.3 什么时候值得用 goroutine

经验上，goroutine 最有价值的场景是：

- **IO 型工作**：网络请求、磁盘 IO、RPC 调用——等待时间长，适合并发隐藏等待。
- **可切分的 CPU 工作**：可以拆成小块并行算，但要注意并发度不要超过有效 CPU（通常与 `GOMAXPROCS` 相关）。
- **流水线/扇入扇出**：多阶段处理用 channel 连接，结构清晰。

不建议用 goroutine 的典型场景：

- 任务极短且数量巨大：调度成本可能比工作本身更大。
- 下游资源严格限流：无界并发只会排队/超时。
- 无退出条件：容易发生 goroutine 泄漏。

### 2.4 “有界并发”是工程默认答案

**概念**：有界并发（bounded concurrency）就是“限制同时运行的 goroutine 数量”。  
**示例**：用带缓冲的 channel 当信号量（semaphore），容量就是并发度。  
**为什么这么设计**：

- 让系统在高负载下保持可控，不会因为突发流量把自己打死。
- 更符合真实世界：CPU 有上限，下游有 QPS 上限，连接池有上限。

在工程里，你可以把它当成一个默认准则：**除非你能证明无界更好，否则就用有界**。

### 2.5 goroutine 泄漏：它们去哪了

所谓泄漏，不是内存一定不释放，而是 goroutine **长期阻塞在某个点**，永远等不到信号：

- 等 channel 读/写但没有对端
- 等锁但永远拿不到（或死锁）
- 等 timer/ticker 但没 Stop 或没退出条件
- 等 context，但你没传/没取消

本文示例会用一个“park 住的 goroutine”来让你直观看到 goroutine 数量的变化。

配图建议：
- 一张“无界并发 → goroutine 数暴涨 → 内存上涨”的曲线示意。
- 一张“semaphore 控并发”的流程图。

## 3. 完整代码示例（可运行）

示例程序做了三件事：

1. `mode=seq`：顺序执行 N 个“模拟 IO 任务”（sleep）。  
2. `mode=bounded`：用信号量限制并发度（推荐写法）。  
3. `mode=leak`：一次性创建很多 goroutine 并阻塞，观察 goroutine 数与堆内存变化。  

代码路径：`series/19/cmd/gorun/main.go`。

```go
package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type config struct {
	mode        string
	n           int
	concurrency int
	sleep       time.Duration
	leak        int
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== goroutine：并发不是免费的午餐（演示） ===")
	fmt.Printf("GOMAXPROCS=%d\n", runtime.GOMAXPROCS(0))
	printStats("start")

	switch cfg.mode {
	case "seq":
		runSequential(cfg)
	case "unbounded":
		runUnbounded(cfg)
	case "bounded":
		runBounded(cfg)
	case "leak":
		demoLeak(cfg)
	default:
		fmt.Printf("unknown -mode=%q\n", cfg.mode)
	}

	printStats("end")
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", "bounded", "seq|unbounded|bounded|leak")
	flag.IntVar(&cfg.n, "n", 300, "number of tasks")
	flag.IntVar(&cfg.concurrency, "c", 20, "bounded concurrency")
	flag.DurationVar(&cfg.sleep, "sleep", 10*time.Millisecond, "simulated IO duration per task")
	flag.IntVar(&cfg.leak, "leak", 5000, "goroutines to park in leak mode")
	flag.Parse()

	if cfg.n < 0 {
		cfg.n = 0
	}
	if cfg.concurrency < 1 {
		cfg.concurrency = 1
	}
	if cfg.sleep < 0 {
		cfg.sleep = 0
	}
	if cfg.leak < 0 {
		cfg.leak = 0
	}
	return cfg
}

func runSequential(cfg config) {
	fmt.Printf("\n--- mode=seq n=%d sleep=%s ---\n", cfg.n, cfg.sleep)
	start := time.Now()
	for i := 0; i < cfg.n; i++ {
		doTask(i, cfg.sleep)
	}
	fmt.Printf("done in %s\n", time.Since(start))
}

func runUnbounded(cfg config) {
	fmt.Printf("\n--- mode=unbounded n=%d sleep=%s ---\n", cfg.n, cfg.sleep)
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(cfg.n)
	for i := 0; i < cfg.n; i++ {
		i := i
		go func() {
			defer wg.Done()
			doTask(i, cfg.sleep)
		}()
	}
	wg.Wait()
	fmt.Printf("done in %s\n", time.Since(start))
}

func runBounded(cfg config) {
	fmt.Printf("\n--- mode=bounded n=%d sleep=%s c=%d ---\n", cfg.n, cfg.sleep, cfg.concurrency)
	start := time.Now()

	sem := make(chan struct{}, cfg.concurrency)
	var wg sync.WaitGroup
	wg.Add(cfg.n)
	for i := 0; i < cfg.n; i++ {
		i := i
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			doTask(i, cfg.sleep)
		}()
	}
	wg.Wait()
	fmt.Printf("done in %s\n", time.Since(start))
}

func doTask(i int, sleep time.Duration) {
	if i%100 == 0 {
		fmt.Printf("task #%d ...\n", i)
	}
	time.Sleep(sleep)
}

func demoLeak(cfg config) {
	fmt.Printf("\n--- mode=leak leak=%d ---\n", cfg.leak)

	block := make(chan struct{})
	for i := 0; i < cfg.leak; i++ {
		go func() {
			<-block
		}()
	}

	time.Sleep(80 * time.Millisecond)
	printStats("after spawn (parked)")

	close(block)
	time.Sleep(80 * time.Millisecond)
	printStats("after release")
}

func printStats(label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("[%s] goroutines=%d heap_alloc=%s heap_objects=%d num_gc=%d\n",
		label,
		runtime.NumGoroutine(),
		bytes(m.HeapAlloc),
		m.HeapObjects,
		m.NumGC,
	)
}

func bytes(n uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)
	switch {
	case n >= MB:
		return fmt.Sprintf("%.2fMB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.2fKB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
```

## 4. 运行效果 + 截图描述

运行顺序模式：

```bash
go run ./series/19/cmd/gorun -mode=seq -n=60 -sleep=10ms
```

你会看到类似输出（节选）：

```
--- mode=seq n=60 sleep=10ms ---
task #0 ...
done in 626ms...
```

运行有界并发模式：

```bash
go run ./series/19/cmd/gorun -mode=bounded -n=60 -c=10 -sleep=10ms
```

典型输出（节选）：

```
--- mode=bounded n=60 sleep=10ms c=10 ---
task #0 ...
done in 62ms...
```

运行 leak 演示：

```bash
go run ./series/19/cmd/gorun -mode=leak -leak=3000
```

典型输出（节选）：

```
[after spawn (parked)] goroutines=3001 heap_alloc=1.85MB ...
[after release] goroutines=1 heap_alloc=1.86MB ...
```

截图建议（每 500 字 1~2 张）：

- 截图 1：seq vs bounded 的耗时对比（强调“并发隐藏等待”）。
- 截图 2：leak 模式 goroutines 从 1 → 3001 的变化（强调“数量上去就是成本”）。
- 截图 3：bounded 的 semaphore 示意图（画一个容量为 c 的桶即可）。

## 5. 常见坑 & 解决方案（必写）

1. **无界并发把系统打爆**：来多少任务开多少 goroutine。解决：默认用有界并发（semaphore/worker pool），并发度与下游能力匹配。
2. **误把并发当并行**：开很多 goroutine 但 `GOMAXPROCS` 很小，CPU 仍然只有那么多。解决：区分 IO/CPU；CPU 型工作控制并发度接近 CPU 核心数。
3. **goroutine 泄漏**：阻塞在 channel/锁/ticker 上，没有退出条件。解决：为每个 goroutine 设计退出信号（context/close channel），并在循环里检查。
4. **忘记回收资源**：ticker 不 Stop、连接不 Close，goroutine 等待永不返回。解决：把 Stop/Close 放在创建点附近（defer），并保证函数一定走到。
5. **并发度过高导致下游更慢**：连接池打满、队列变长、重试风暴。解决：限制并发 + 限流 + 重试退避；把“快失败”策略写清楚。
6. **共享数据没保护**：多个 goroutine 写同一 map/slice 导致 data race。解决：用 Mutex/Channel 保护，或做数据分片。
7. **在循环里捕获变量**：`for i := ... { go func(){ use i }() }` 导致逻辑错误。解决：在循环内 `i := i` 拷贝或传参。
8. **用 time.Sleep 当同步工具**：靠“睡一会儿应该好了”。解决：用 WaitGroup/channel/ctx 明确同步点。

配图建议：
- “泄漏来源清单”图：channel/lock/ticker/context。
- “并发度与延迟”示意图：并发提升到某阈值后收益下降。

## 6. 进阶扩展 / 思考题

- 把 `bounded` 改成真正的 worker pool：一个任务 channel + N 个 worker，比较与 semaphore 的差异。
- 给任务加错误返回与统计：成功/失败/超时，思考错误传播策略（集中收集还是即时失败）。
- 把 `doTask` 改成 CPU 计算（例如 hash），观察并发度超过 `GOMAXPROCS` 后的收益变化。
- 为 leak 模式加 context 超时：让“停住的 goroutine”能自动退出。
- 思考题：你服务的下游（DB/Redis/HTTP）各自的最佳并发度怎么定？你会用哪些指标验证？

---

goroutine 的价值是让并发结构表达得很简单，但它的成本也真实存在：调度、栈、争用、以及泄漏风险。工程上，把“有界并发”当默认，把退出条件当必选，把并发度当可配置并用指标验证，你就能既吃到并发收益，又避免免费午餐幻觉带来的事故。
