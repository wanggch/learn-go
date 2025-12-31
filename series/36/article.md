# race detector：并发 bug 的照妖镜

你好，我是汪小成。并发 bug 最可怕的地方是“看起来能跑”：线上偶发错乱、计数不准、日志顺序怪异，你却很难复现。原因往往不是逻辑错，而是 **数据竞争**。Go 的 race detector 就像一面照妖镜，它能把隐藏很深的并发问题直接暴露出来。本文会先准备环境，再讲清 race detector 的核心概念与设计原因，最后给出完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/36`。
- 示例入口：`series/36/racecase`。

### 1.2 运行命令

普通测试（不会触发 race 检测）：

```bash
go test ./series/36/racecase
```

启用 race detector：

```bash
go test ./series/36/racecase -race -run TestUnsafeCounter
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go test ./series/36/racecase -race -run TestUnsafeCounter
```

### 1.3 前置知识

- 理解 goroutine 并发执行。
- 知道 mutex 或 atomic 的基本用途。

提示：race detector 不是“偶尔跑一次”，建议在关键改动或版本发布前跑一遍，能省下很多线上排查时间。

配图建议：
- 一张“数据竞争示意图”（两个 goroutine 同时写）。
- 一张“race detector 工作流程”示意图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 什么是 data race

**概念**：两个或多个 goroutine 并发访问同一变量，且至少一个是写操作。  
**示例**：多个 goroutine 同时执行 `count++`。  
**为什么这么设计**：Go 运行时不会自动帮你加锁，需要显式同步。

### 2.2 `-race` 不是静态分析

**概念**：race detector 是动态检测，需要在运行时观察读写。  
**示例**：只有在执行到那段代码时才会报警。  
**为什么这么设计**：真正的竞态发生在运行期，静态规则无法覆盖全部情况。

### 2.3 mutex 与 atomic 的取舍

**概念**：mutex 更通用，atomic 更轻量但语义更严格。  
**示例**：计数器适合 atomic，复杂结构适合 mutex。  
**为什么这么设计**：性能与可读性需要权衡。

### 2.4 race detector 会让测试变慢

**概念**：`-race` 会插桩，运行成本明显上升。  
**示例**：同一套测试可能慢 2~5 倍。  
**为什么这么设计**：代价换来的是“安全性”，适合在 CI 或关键测试阶段跑。

### 2.5 结果要能“指向代码”

**概念**：race 报告会给出读写位置和 goroutine 堆栈。  
**示例**：报告里会出现 `race.go:15`。  
**为什么这么设计**：帮助你快速定位冲突点。

### 2.6 race 不是并发 bug 的全部

**概念**：没有 race 报告不代表没有并发问题，比如死锁或活锁。  
**示例**：goroutine 全部阻塞时，race detector 也无法帮你解开。  
**为什么这么设计**：race 只覆盖“数据竞争”这一类问题。
### 2.7 先保证正确，再谈性能

**概念**：并发安全是第一位，性能优化只能在正确基础上做。  
**示例**：先用 mutex 保证正确，再逐步拆分锁或引入 atomic。  
**为什么这么设计**：错误的优化只会让 bug 更难定位。

配图建议：
- 一张“mutex vs atomic”对比图。
- 一张“race 报告示意图”。

## 3. 完整代码示例（可运行）

示例包含：

1. 一个 **不安全计数器**（故意有竞态）。
2. 一个 **mutex 版本**。
3. 一个 **atomic 版本**。
4. 对应的测试用例。

代码路径：`series/36/racecase`。

```go
package racecase

import (
	"sync"
	"sync/atomic"
)

func UnsafeCounter(n int) int {
	var count int
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			count++
		}()
	}
	wg.Wait()
	return count
}

func SafeCounterMutex(n int) int {
	var count int
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			mu.Lock()
			count++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return count
}

func SafeCounterAtomic(n int) int {
	var count int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			atomic.AddInt64(&count, 1)
		}()
	}
	wg.Wait()
	return int(count)
}
```

```go
package racecase

import "testing"

func TestUnsafeCounter(t *testing.T) {
	for i := 0; i < 3; i++ {
		_ = UnsafeCounter(1000)
	}
}

func TestSafeCounterMutex(t *testing.T) {
	got := SafeCounterMutex(1000)
	if got != 1000 {
		t.Fatalf("got %d want %d", got, 1000)
	}
}

func TestSafeCounterAtomic(t *testing.T) {
	got := SafeCounterAtomic(1000)
	if got != 1000 {
		t.Fatalf("got %d want %d", got, 1000)
	}
}
```

说明：`TestUnsafeCounter` 在普通测试中可能“看起来没问题”，但 `-race` 会直接报出数据竞争。

实践建议：如果你的项目里并发很多，可以先挑最核心的包跑 `-race`，逐步扩大范围，而不是一开始就全量跑。

配图建议：
- 一张“Unsafe vs Safe”对比表。
- 一张“锁与原子操作差异”示意图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go test ./series/36/racecase -race -run TestUnsafeCounter
```

示例输出（节选）：

```
==================
WARNING: DATA RACE
Read at 0x00c000012328 by goroutine 15:
  learn-go/series/36/racecase.UnsafeCounter.func1()
      /Users/wanggc/02-my/04-demo/learn-go/series/36/racecase/race.go:15 +0x7b

Previous write at 0x00c000012328 by goroutine 12:
  learn-go/series/36/racecase.UnsafeCounter.func1()
      /Users/wanggc/02-my/04-demo/learn-go/series/36/racecase/race.go:15 +0x8d

--- FAIL: TestUnsafeCounter (0.01s)
    testing.go:1617: race detected during execution of test
FAIL
FAIL	learn-go/series/36/racecase	0.033s
```

输出解读：race 报告清楚给出了 **读写冲突的代码位置**，并指向相关的 goroutine。把这些位置对应到代码，基本就能定位问题根源，而且很直接。

截图描述建议：
- 截一张 race 报告图，突出 **Read/Write** 两个位置。
- 再截一张 FAIL 结果图，强调 race 会让测试失败。

配图建议：
- 一张“race 报告结构说明图”。
- 一张“检测流程图”。

## 5. 常见坑 & 解决方案（必写）

1. **只在本地跑普通测试**：竞态问题被掩盖。  
   解决：在 CI 或关键模块定期跑 `-race`。

2. **以为 atomic 万能**：多个变量需要一致性时用 atomic 也不安全。  
   解决：复杂状态优先用 mutex 或更高层的同步结构。

3. **锁粒度太大**： race 解决了，但性能变差。  
   解决：缩小锁范围或拆分状态。

4. **滥用 `sync.Map`**：以为并发 map 就没有竞态。  
   解决：理解读写模式，必要时仍需外部锁。

5. **错误共享临时变量**：循环中引用同一个变量导致竞态。  
   解决：每个 goroutine 拷贝自己的值。

6. **忽略 race 报告的堆栈**：只看错误标题。  
   解决：定位读写位置，找到冲突的 goroutine。

补充建议：把关键包的 `-race` 结果纳入发布流程，哪怕每周跑一次，也能显著降低并发故障风险。

配图建议：
- 一张“竞态常见场景”图。
- 一张“锁粒度对比”图。

## 6. 进阶扩展 / 思考题

1. 尝试把 `UnsafeCounter` 改成 `map` 写入，看看 race 输出如何变化。
2. 把 `SafeCounterAtomic` 改成 `atomic.Int64`，比较可读性。
3. 在项目的 CI 中加入 `-race`，思考运行成本与收益。
4. 用 `-run` 只跑关键测试，减少 `-race` 的执行时间。

配图建议：
- 一张“CI 中的 race 流程图”。
- 一张“优化跑法（-run/-race）示意图”。
