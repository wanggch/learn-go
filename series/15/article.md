# GC 到底什么时候跑？会卡住程序吗

大家好，我是汪小成。你可能遇到过这样的场景：线上服务明明 CPU 不高，但延迟曲线偶尔会冒出几根“尖刺”；排查一圈发现不是慢 SQL，也不是网络抖动，最后有人丢来一句“可能是 GC 卡住了”。问题是——**GC 到底什么时候跑？是不是一跑就 STW？会卡多久？我能控制它吗？**  
Go 的 GC 是并发标记、增量清理的设计，绝大多数时间不是“全停”，但它确实会产生短暂停顿。本文用一个可运行的小程序带你看懂：GC 触发条件、`GOGC` 的含义、`runtime.MemStats` 关键字段，以及如何用数据判断“是不是 GC 在背锅”。

## 目录

1. 环境准备 / 前提知识
2. 核心概念解释（概念 → 示例 → 为什么这么设计）
3. 完整代码示例（可运行）
4. 运行效果 + 截图描述
5. 常见坑 & 解决方案（必写）
6. 进阶扩展 / 思考题

## 1. 环境准备 / 前提知识

### 1.1 环境

- Go 1.22+（项目根目录使用 `go.work`）。
- 本篇目录：`series/15`；示例入口：`series/15/cmd/gcplay/main.go`。

### 1.2 运行命令

```bash
go run ./series/15/cmd/gcplay

# 可选：打开 GC trace（输出很多，但很有用）
GODEBUG=gctrace=1 go run ./series/15/cmd/gcplay
```

### 1.3 你需要知道的证明方式

- **别靠感觉**：先用数据回答“是不是 GC 造成尖刺”。
- 本文主要用 `runtime.ReadMemStats` 的字段做观测：`NumGC`、`PauseTotalNs`、`NextGC`、`HeapAlloc` 等。

配图建议（约 500 字 1~2 张）：
- 一张“请求延迟尖刺 vs GC PauseTotal”对齐示意（概念图即可）。
- 一张 `MemStats` 字段速查表截图（或脑图）。

## 2. 核心概念解释

### 2.1 Go 的 GC 解决什么问题

**概念**：GC 的目标是自动回收“不可达对象”，让你不用手写释放逻辑。  
**示例**：你不停 `make([]byte, ...)` 分配临时对象；当这些对象不再被引用，就该被回收，否则内存会持续增长。  
**为什么这么设计**：在工程复杂度、性能和安全之间取平衡，避免手动内存管理的高风险。

### 2.2 GC 什么时候触发：核心是“堆增长到阈值”

Go 的 GC 不是按固定时间跑，主要由“堆增长”驱动。一个非常重要的字段是：

- `NextGC`：下一次 GC 的目标堆大小（近似阈值）。

当堆的分配增长接近这个阈值，运行时会更积极地推进 GC 工作。你可以把它理解成：**堆涨得越快，GC 越勤快**。

### 2.3 GOGC：用“增长比例”控制 GC 频率

你经常会听到 `GOGC=100`。它的直觉是：

- **GOGC 越小**：更激进，阈值更低，GC 更频繁，堆更小，但 CPU/暂停总量可能更高。
- **GOGC 越大**：更宽松，GC 更少，堆更大，暂停可能更少但内存占用更高。

这不是“越大越好/越小越好”的开关，而是内存与 CPU 的权衡旋钮。本文的示例会让你看到：同样的分配压力下，不同 `GOGC` 会带来不同的 `NumGC` 增量与 `PauseTotalNs` 增量。

### 2.4 会不会 STW：会，但通常很短

Go GC 的并发标记意味着“绝大多数时间应用仍在跑”，但它仍会有短暂停顿：

- 典型停顿来自某些阶段需要短暂同步（你可以把它理解成“换挡”）。
- 我们在示例里用 `PauseTotalNs`（总停顿）和 `last_pause`（最近一次停顿）做直观观测。

关键点：**GC 导致的“卡住”不等于长时间 STW**。多数情况下，问题更可能来自“分配太多导致 GC 过于频繁”，或者“某些路径产生大量短命对象”。

### 2.5 如何判断 GC 是否在背锅

你可以用三个问题做快速定位：

1. 延迟尖刺是否与 `NumGC` 增加、`PauseTotalNs` 增加相关？
2. `HeapAlloc` 是否持续上升，`NextGC` 是否频繁被触发？
3. `GCCPUFraction`（GC 使用 CPU 的比例）是否异常偏高？

配图建议：
- 一张“GOGC 旋钮”示意（小 → 频繁 GC， 大 → 更大堆）。
- 一张“堆增长触发 GC”的阈值线示意（HeapAlloc 与 NextGC）。

## 3. 完整代码示例（可复制运行）

代码路径：`series/15/cmd/gcplay/main.go`。它把分配压力拆成多个 phase：默认 `GOGC=100`、激进 `GOGC=20`、宽松 `GOGC=200`、禁用 GC、手动 `runtime.GC()`，每个 phase 都打印 memstats 快照，方便对比。

```go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

type snapshot struct {
	heapAlloc      uint64
	heapSys        uint64
	heapObjects    uint64
	numGC          uint32
	pauseTotalNs   uint64
	lastPauseNs    uint64
	nextGC         uint64
	gccpuFraction  float64
	lastGCTimeUnix int64
}

func main() {
	fmt.Println("=== GC 何时运行 & STW 影响演示 ===")
	fmt.Println("可选：开启 gctrace 观察更细节：GODEBUG=gctrace=1 go run ./series/15/cmd/gcplay")

	phase("默认 GOGC=100", func() {
		debug.SetGCPercent(100)
		runChurn(80_000, 256, true)
	})

	phase("更激进 GOGC=20（更频繁 GC，堆更小）", func() {
		debug.SetGCPercent(20)
		runChurn(80_000, 256, true)
	})

	phase("更宽松 GOGC=200（更少 GC，堆更大）", func() {
		debug.SetGCPercent(200)
		runChurn(80_000, 256, true)
	})

	phase("禁用 GC（仅用于演示）", func() {
		debug.SetGCPercent(-1)
		runChurn(120_000, 512, false)
	})

	phase("手动触发 runtime.GC()", func() {
		debug.SetGCPercent(100)
		runtime.GC()
	})
}

func phase(name string, fn func()) {
	fmt.Printf("\n--- %s ---\n", name)
	before := readSnapshot()
	printSnapshot("before", before)

	start := time.Now()
	fn()
	cost := time.Since(start)

	after := readSnapshot()
	printSnapshot("after ", after)
	fmt.Printf("time=%s | gc+%d | pause+%s\n",
		cost,
		int(after.numGC-before.numGC),
		time.Duration(after.pauseTotalNs-before.pauseTotalNs))
}

func runChurn(objects int, size int, keep bool) {
	var keepAlive [][]byte
	if keep {
		keepAlive = make([][]byte, 0, objects/8)
	}

	for i := 0; i < objects; i++ {
		b := make([]byte, size)
		b[0] = byte(i)
		if keep && i%8 == 0 {
			keepAlive = append(keepAlive, b)
		}
	}

	runtime.KeepAlive(keepAlive)
}

func readSnapshot() snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	lastPause := uint64(0)
	if m.NumGC > 0 {
		lastPause = m.PauseNs[(m.NumGC+255)%256]
	}

	lastGCTime := int64(0)
	if m.LastGC != 0 {
		lastGCTime = int64(m.LastGC / 1e9)
	}

	return snapshot{
		heapAlloc:      m.HeapAlloc,
		heapSys:        m.HeapSys,
		heapObjects:    m.HeapObjects,
		numGC:          m.NumGC,
		pauseTotalNs:   m.PauseTotalNs,
		lastPauseNs:    lastPause,
		nextGC:         m.NextGC,
		gccpuFraction:  m.GCCPUFraction,
		lastGCTimeUnix: lastGCTime,
	}
}

func printSnapshot(label string, s snapshot) {
	fmt.Printf("%s | heap_alloc=%s heap_sys=%s heap_obj=%d next_gc=%s num_gc=%d last_pause=%s gccpu=%.4f last_gc=%d\n",
		label,
		bytes(s.heapAlloc),
		bytes(s.heapSys),
		s.heapObjects,
		bytes(s.nextGC),
		s.numGC,
		time.Duration(s.lastPauseNs),
		s.gccpuFraction,
		s.lastGCTimeUnix,
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

运行命令：

```bash
go run ./series/15/cmd/gcplay
```

你会看到每个 phase 都打印 `before/after` 两行快照，并给出 `gc+N` 与 `pause+T` 的增量。数值会因机器、版本、调度而不同，但趋势一般很稳定：

- `GOGC=20`：通常 `gc+N` 更大（更频繁），`heap_alloc` 更小。
- `GOGC=200`：通常 `gc+N` 更小（更少），但 `heap_alloc/next_gc` 会更大。
- `SetGCPercent(-1)`：几乎不发生 GC，堆迅速膨胀；随后 `runtime.GC()` 会把不可达对象清理掉。

截图建议：

- 截图 1：三段对比（GOGC=100/20/200），用荧光笔标出 `gc+N` 与 `pause+T`。
- 截图 2：禁用 GC 后 `heap_alloc` 飙升，再手动 GC 归位的对比。
- 截图 3：`GODEBUG=gctrace=1` 的片段截图，标出 `gc` 次数与 pause（仅示意即可）。

## 5. 常见坑 & 解决方案（必写）

1. **把 GC 当成“定时器”理解**：以为每隔几秒跑一次。解决：关注堆增长阈值（`NextGC`）与分配速率。
2. **只看 `HeapAlloc` 不看 `PauseTotalNs`**：内存不大不代表没有暂停。解决：同时看 GC 次数、总暂停、最近暂停。
3. **盲目调大 `GOGC`**：GC 次数少了，但内存暴涨触发 OOM 或容器被 kill。解决：先确定内存预算，再调 `GOGC`，用压测验证。
4. **盲目调小 `GOGC`**：堆变小但 GC 过勤，CPU 被吃掉，吞吐下降。解决：观察 `GCCPUFraction` 与 p99 延迟，别只看堆大小。
5. **把“延迟尖刺”都归因于 STW**：很多尖刺来自锁竞争、IO、调度、日志、DNS。解决：用指标对齐验证（GC pause vs p99）。
6. **短命对象太多**：比如热路径频繁 `fmt.Sprintf`、`[]byte` 拼接、临时 map。解决：复用 buffer、用 `strings.Builder`、预分配、减少转换。
7. **禁用 GC 解决问题**：短期看似稳定，长期内存爆炸。解决：不要在服务里长期 `SetGCPercent(-1)`；只用于实验与定位。

配图建议：一张“调 GOGC 的决策表”（内存预算/吞吐/延迟三者权衡）。

## 6. 进阶扩展 / 思考题

- 用 `GODEBUG=gctrace=1` 跑同一个程序，尝试对比不同 `GOGC` 下的 trace，写出你的观察结论。
- 把 `runChurn` 改成“更多短命对象”与“更多长命对象”两种模式，比较 GC 行为差异。
- 给程序加 `pprof`（后面章节会讲），定位分配热点：是谁在制造短命对象？
- 在你的业务服务里补齐 GC 指标：`NumGC`、`PauseTotalNs`、`HeapAlloc`、`GCCPUFraction`，并与 p95/p99 延迟一起看。
- 思考题：当你能减少 30% 分配量时，是否还需要调 `GOGC`？为什么？

---

GC 不是“偶尔全停一下”的黑盒，它主要由堆增长驱动，`GOGC` 控制频率与堆大小，STW 通常很短但“分配过多导致频繁 GC”会让系统变慢。把本文的示例跑一遍，再把同样的观测思路带回你的服务：先看数据再下结论，通常你会发现优化分配比调 GC 参数更划算。下一篇我们会继续深入：GC 的 STW 细节与调优边界，帮助你更有把握地做性能决策。
