# WaitGroup 与 Mutex：共享数据的正确方式

你好，我是汪小成。你有没有遇到过这种并发“疑难杂症”：压测时计数器偶尔少了几千；日志里某个状态有时是 1、有时是 0；甚至线上偶发崩溃，但本地怎么都复现不了。很多并发 bug 的根因其实很朴素：**多个 goroutine 同时读写同一份共享数据**，你以为“++ 很简单”，但它并不是原子操作。Go 给了两把最常用的工程工具：`sync.WaitGroup` 用来“等大家做完”，`sync.Mutex` 用来“同一时刻只让一个人改”。本文会从痛点场景出发，解释 WaitGroup/Mutex 的核心设计，再给出完整可运行示例、运行效果、常见坑与解决方案。

## 目录

1. 环境准备 / 前提知识  
2. 核心概念解释（概念 → 示例 → 为什么这么设计）  
3. 完整代码示例（可运行）  
4. 运行效果 + 截图描述  
5. 常见坑 & 解决方案（必写）  
6. 进阶扩展 / 思考题  

## 1. 环境准备 / 前提知识

### 1.1 版本与目录

- Go 1.22+（仓库根目录用 `go.work` 管理多模块）。
- 本篇目录：`series/20`。
- 示例入口：`series/20/cmd/counter/main.go`。

### 1.2 运行命令

```bash
# 正确写法：Mutex 保护共享计数器
go run ./series/20/cmd/counter -mode=mutex -workers=20 -loops=20000

# 错误写法：无锁并发写（结果会随机偏小）
go run ./series/20/cmd/counter -mode=unsafe -workers=20 -loops=20000

# 加分项：race detector（需要 Go 安装支持）
go run -race ./series/20/cmd/counter -mode=unsafe -workers=20 -loops=20000
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/20/cmd/counter -mode=mutex -workers=20 -loops=20000
```

### 1.3 前置知识

- goroutine 基础（第 19 篇）。
- 了解“数据竞争（data race）”的直觉：两个 goroutine 同时写同一变量，结果不可预测。

配图建议：
- 一张“counter++ 的拆解图”（读→加→写三步，说明为什么不是原子）。
- 一张“WaitGroup 等待所有 goroutine 完成”的流程图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 WaitGroup：负责“等待”，不负责“互斥”

**概念**：`sync.WaitGroup` 用来等待一组 goroutine 结束。它解决的是“什么时候所有工作都做完”，不是“怎么保护共享数据”。  
**示例**：主 goroutine `wg.Add(n)` 后启动 n 个 worker，每个 worker `defer wg.Done()`，主 goroutine 最后 `wg.Wait()`。  
**为什么这么设计**：把职责分清：等待与互斥是两类问题。等待是流程控制，互斥是数据一致性。

一个常见误解是：只要用了 WaitGroup 就“线程安全”了。不是的。WaitGroup 只保证“结束时刻”，不保证“过程中不会打架”。

### 2.2 Mutex：负责“互斥”，让临界区一次只进一个

**概念**：`sync.Mutex` 用于保护临界区：那段“必须串行执行”的代码。  
**示例**：对共享 `counter` 做 `mu.Lock(); counter++; mu.Unlock()`。  
**为什么这么设计**：在共享内存模型里，互斥锁是最直接、最通用的正确性工具。它让你显式标记“这里不能并发”。

注意：锁不是为了“变慢”，锁是为了“变对”。没有正确性，性能指标没有意义。

### 2.3 `counter++` 为什么会丢数据

**概念**：`counter++` 在机器层面不是一个不可分割的动作，它至少包含：

1. 从内存读出 counter 到寄存器  
2. 寄存器 +1  
3. 写回内存  

两个 goroutine 交错执行时，可能都读到同一个旧值，最终只写回一次 +1。  
**为什么这么设计**：Go 语言不帮你把所有写操作都变成原子——因为原子化有成本，也会限制优化。正确做法是由你选择：用 Mutex、用 atomic，还是改成无共享（channel）。

### 2.4 有界并发 + 互斥：工程里的常见组合

你在第 19 篇看到“有界并发”，这一篇你会发现它常常和 Mutex 搭配出现：

- 用有界并发控制 goroutine 数量，避免把系统打爆。
- 用 Mutex 保护共享状态（计数器、map、缓存、统计）。

工程上经常是这套组合拳：**控并发 + 保一致**。

### 2.5 race detector：并发 bug 的照妖镜

`-race` 会在运行时检测数据竞争，把“偶发错”变成“稳定报错”。它通常会让程序变慢、占用更多内存，但在开发/测试阶段极其有价值。  
本篇示例在 `unsafe` 模式下非常适合配合 `-race` 观察。

配图建议：
- 一张“WaitGroup vs Mutex 职责对比表”（WaitGroup=等待；Mutex=保护）。
- 一张“race detector 报错示意图”（截图建议即可，不必真实贴栈）。

## 3. 完整代码示例（可运行）

示例分两种模式：

- `mode=unsafe`：不加锁并发 `counter++`，结果通常偏小。  
- `mode=mutex`：用 Mutex 包住 `counter++`，结果稳定正确。  

代码路径：`series/20/cmd/counter/main.go`。

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
	mode      string
	workers   int
	loops     int
	showEvery int
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== WaitGroup 与 Mutex：共享数据的正确方式 ===")
	fmt.Printf("mode=%s workers=%d loops=%d GOMAXPROCS=%d\n",
		cfg.mode, cfg.workers, cfg.loops, runtime.GOMAXPROCS(0))

	start := time.Now()
	var got int
	var expected int

	switch cfg.mode {
	case "unsafe":
		got, expected = runUnsafe(cfg)
	case "mutex":
		got, expected = runWithMutex(cfg)
	default:
		fmt.Printf("unknown -mode=%q (use unsafe|mutex)\n", cfg.mode)
		return
	}

	fmt.Printf("expected=%d got=%d cost=%s\n", expected, got, time.Since(start))
	if got != expected {
		fmt.Println("WARNING: result mismatch (likely data race). Try: go run -race ./series/20/cmd/counter -mode=unsafe")
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", "mutex", "unsafe|mutex")
	flag.IntVar(&cfg.workers, "workers", 40, "number of goroutines")
	flag.IntVar(&cfg.loops, "loops", 50_000, "increments per goroutine")
	flag.IntVar(&cfg.showEvery, "show-every", 0, "print progress every N loops (0 to disable)")
	flag.Parse()

	if cfg.workers < 1 {
		cfg.workers = 1
	}
	if cfg.loops < 0 {
		cfg.loops = 0
	}
	return cfg
}

func runUnsafe(cfg config) (got int, expected int) {
	fmt.Println("\n--- unsafe: no mutex ---")
	expected = cfg.workers * cfg.loops

	counter := 0
	var wg sync.WaitGroup
	wg.Add(cfg.workers)

	for w := 0; w < cfg.workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < cfg.loops; i++ {
				counter++
				if cfg.showEvery > 0 && i%cfg.showEvery == 0 && w == 0 {
					fmt.Printf("  progress i=%d counter=%d\n", i, counter)
				}
			}
		}()
	}

	wg.Wait()
	return counter, expected
}

func runWithMutex(cfg config) (got int, expected int) {
	fmt.Println("\n--- mutex: protect shared counter ---")
	expected = cfg.workers * cfg.loops

	counter := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(cfg.workers)

	for w := 0; w < cfg.workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < cfg.loops; i++ {
				mu.Lock()
				counter++
				if cfg.showEvery > 0 && i%cfg.showEvery == 0 && w == 0 {
					fmt.Printf("  progress i=%d counter=%d\n", i, counter)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return counter, expected
}
```

## 4. 运行效果 + 截图描述

运行正确版本（mutex）：

```bash
go run ./series/20/cmd/counter -mode=mutex -workers=20 -loops=20000
```

典型输出（节选）：

```
--- mutex: protect shared counter ---
expected=400000 got=400000 cost=31ms...
```

运行错误版本（unsafe）：

```bash
go run ./series/20/cmd/counter -mode=unsafe -workers=20 -loops=20000
```

典型输出（节选，got 往往偏小）：

```
--- unsafe: no mutex ---
expected=400000 got=261432 cost=1ms...
WARNING: result mismatch (likely data race).
```

截图建议（每 500 字 1~2 张）：

- 截图 1：unsafe 的结果偏小（强调“++ 不是原子”）。
- 截图 2：mutex 的结果正确（强调“锁住临界区”）。
- 截图 3：加 `-race` 后的报错提示截图（展示数据竞争检测价值）。

## 5. 常见坑 & 解决方案（必写）

1. **把 WaitGroup 当线程安全工具**：只等完成，不保护共享数据。解决：共享写必须加锁或改成无共享（channel）。
2. **WaitGroup Add/Done 顺序错**：先启动 goroutine 再 Add，可能出现 Wait 提前返回或 panic。解决：先 `Add(n)` 再启动；或在启动前 Add(1)。
3. **拷贝 WaitGroup**：把 WaitGroup 当值传递/放入 struct 复制，导致计数错乱。解决：不复制 WaitGroup；需要共享就用指针或把 WaitGroup 放在同一作用域。
4. **Lock 后忘记 Unlock**：异常路径/return 导致死锁。解决：`mu.Lock(); defer mu.Unlock()`（注意临界区要尽量小）。
5. **临界区过大**：把耗时 IO 也放进锁里，吞吐暴跌。解决：只锁“共享状态更新”，IO 放锁外。
6. **锁顺序不一致**：多把锁交错获取导致死锁。解决：规定全局锁顺序；或用更高层结构（单锁/队列化）。
7. **只用“结果对不对”判断并发正确性**：有时碰巧正确，但仍然有 data race。解决：开发阶段用 `-race`，并写压力测试。
8. **并发度太高放大锁竞争**：锁保护得对但很慢。解决：降低并发度、有界并发、分片（sharding）、或改用 channel 串行化更新。

配图建议：
- “WaitGroup 正确用法”时序图（Add→go→Done→Wait）。
- “临界区大小”对比图（锁内 IO vs 锁内仅更新计数）。

## 6. 进阶扩展 / 思考题

- 把计数器改成 map 统计（如按 userID 计数），用 Mutex 保护 map 写入，观察 `-race` 的效果。
- 试着用 channel 替代 Mutex：所有更新发送到一个 goroutine 串行处理（对比可读性与吞吐）。
- 加入“有界并发”：在跑任务时限制 workers 数量，观察性能与一致性的平衡。
- 思考题：你的业务里哪些数据是“必须一致”的？哪些是“最终一致/近似一致”就够（例如统计指标）？不同答案会影响你选择 Mutex、atomic、还是异步聚合。

---

WaitGroup 解决“等大家做完”，Mutex 解决“别同时改同一份数据”。把这两个工具的职责分清，再配合 `-race` 做验证，你就能把大量并发玄学问题变成可定位、可修复的工程问题。下一篇我们会继续扩展共享数据的正确方式：WaitGroup 与 Mutex 的更多组合，以及 channel 的通信模型。
