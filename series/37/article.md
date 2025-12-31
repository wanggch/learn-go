# pprof：性能问题从哪里下手

你好，我是汪小成。很多人遇到性能问题时只会“猜”：是不是数据库慢？是不是算法太复杂？结果一通优化，效果依旧不明显。性能问题需要证据，而不是直觉。`pprof` 就是 Go 提供的证据工具，它能告诉你 **时间花在哪、内存耗在哪**。本文会先准备环境，再讲清 pprof 的核心概念与设计原因，最后给出完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/37`。
- 示例入口：`series/37/cmd/pproflab/main.go`。

### 1.2 运行命令

```bash
go run ./series/37/cmd/pproflab
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/37/cmd/pproflab
```

### 1.3 前置知识

- 了解 `go test` / `go run` 的基本流程。
- 了解函数调用栈的基础概念。

提示：`pprof` 有多种入口（HTTP、CPU/Heap 文件），本文用文件方式，避免端口依赖。

小建议：先在本地用固定输入跑一次，确认热点稳定，再去线上或更真实的环境采样。这样更容易判断“优化是否真的有效”，也更稳。

配图建议：
- 一张“性能诊断流程图”。
- 一张“CPU vs Heap Profiling 对比图”。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 CPU Profile 关注“时间去哪了”

**概念**：CPU profile 采样运行时的 CPU 使用情况。  
**示例**：`pprof.StartCPUProfile` + `pprof.StopCPUProfile`。  
**为什么这么设计**：采样方式开销小，适合线上或高频分析。

### 2.2 Heap Profile 关注“内存去哪了”

**概念**：Heap profile 记录内存分配与存活情况。  
**示例**：`pprof.WriteHeapProfile`。  
**为什么这么设计**：内存问题常常比时间问题更隐蔽。

### 2.3 Profile 只对“被测区间”有效

**概念**：profile 只覆盖你开始与结束之间的代码。  
**示例**：在 `workload()` 前后包住 CPU profile。  
**为什么这么设计**：避免噪音，聚焦关键路径。

### 2.4 结果必须能“回到代码”

**概念**：pprof 输出会关联到函数与行号。  
**示例**：`strings.Repeat` 或 `sort.Strings`。  
**为什么这么设计**：让你能直接定位“哪一行最耗”。

### 2.5 先测趋势，再做优化

**概念**：pprof 更适合找“主要热点”，而不是微优化。  
**示例**：如果排序占了 70%，先优化排序。  
**为什么这么设计**：性能优化优先级取决于“最大瓶颈”。

### 2.6 采样意味着“近似”，不是绝对真相

**概念**：pprof 是采样工具，结果是概率意义上的热点分布。  
**示例**：同一段代码多跑几次，热点排序可能略有波动。  
**为什么这么设计**：采样开销更小，适合在真实场景下使用。

配图建议：
- 一张“火焰图结构示意图”。
- 一张“热点函数排名表”。

## 3. 完整代码示例（可运行）

示例做了三件事：

1. 运行一段 CPU/内存都较重的 workload。
2. 生成 `cpu.pprof` 与 `heap.pprof` 文件。
3. 给出后续查看建议。

代码路径：`series/37/cmd/pproflab/main.go`。

```go
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"
)

func main() {
	outDir := profileDir()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	cpuPath := filepath.Join(outDir, "cpu.pprof")
	memPath := filepath.Join(outDir, "heap.pprof")

	cpuFile, err := os.Create(cpuPath)
	if err != nil {
		panic(err)
	}

	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		_ = cpuFile.Close()
		panic(err)
	}

	result := workload()

	pprof.StopCPUProfile()
	_ = cpuFile.Close()

	runtime.GC()
	memFile, err := os.Create(memPath)
	if err != nil {
		panic(err)
	}
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		_ = memFile.Close()
		panic(err)
	}
	_ = memFile.Close()

	fmt.Println("done")
	fmt.Printf("result checksum: %s\n", result)
	fmt.Printf("cpu profile: %s\n", cpuPath)
	fmt.Printf("heap profile: %s\n", memPath)
	fmt.Println("\nview tips:")
	fmt.Println("go tool pprof -top", cpuPath)
	fmt.Println("go tool pprof -top", memPath)
}

func workload() string {
	start := time.Now()
	data := make([]string, 0, 40000)
	for i := 0; i < 40000; i++ {
		data = append(data, strings.Repeat("go", i%10+1))
	}
	for i := 0; i < 4; i++ {
		sort.Strings(data)
	}

	h := sha256.New()
	for _, item := range data {
		h.Write([]byte(item))
	}
	elapsed := time.Since(start)
	fmt.Printf("workload finished in %s\n", elapsed)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func profileDir() string {
	if _, err := os.Stat(filepath.Join("series", "37")); err == nil {
		return filepath.Join("series", "37", "tmp")
	}
	return filepath.Join("tmp")
}
```

说明：示例用文件方式生成 profile，避免监听端口；`runtime.GC()` 用来尽量稳定 heap profile 的结果。

实践流程：先用 `-top` 看函数排名，再用 `-top -cum` 看累计耗时，最后再决定是否生成火焰图。先读结论，再看细节，能省很多时间。

配图建议：
- 一张“CPU/Heap 文件产出位置”的示意图。
- 一张“workload 组成”结构图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/37/cmd/pproflab
```

示例输出（节选）：

```
workload finished in 6.803856ms
done
result checksum: 1dd32a066aabbdf8d96c77a05c204d3d36137a4d5910e9a79413e054c637ef01
cpu profile: series/37/tmp/cpu.pprof
heap profile: series/37/tmp/heap.pprof

view tips:
go tool pprof -top series/37/tmp/cpu.pprof
go tool pprof -top series/37/tmp/heap.pprof
```

输出解读：程序会生成 CPU 与 Heap profile 文件。接下来你可以用 `go tool pprof -top` 查看热点函数排名，或生成火焰图进一步分析。

如果要看图形化视图，可以使用 `go tool pprof -web`，但需要本机安装 `graphviz`，更直观。

补充建议：如果你看见某个函数占比很高，先问自己“它是业务核心还是实现问题”。核心逻辑占比高是正常的，优化要优先从“意料之外”的热点下手。

截图描述建议：
- 截一张终端输出图，突出 **profile 文件路径**。
- 再截一张 `tmp/` 目录中文件列表，强调结果落盘。

配图建议：
- 一张“pprof 文件到分析结果”的流程图。
- 一张“火焰图结构示意图”。

## 5. 常见坑 & 解决方案（必写）

1. **Profile 范围太大**：把初始化或 IO 也算进去。  
   解决：只包住关键路径，减少噪音。

2. **忘记 StopCPUProfile**：文件不完整或无法读。  
   解决：确保 `StopCPUProfile` 在逻辑结束时执行。

3. **Heap 结果波动大**：GC 时机不同导致差异。  
   解决：采样前先 `runtime.GC()`，并多跑几次看趋势。

4. **只看单次输出**：随机波动导致误判。  
   解决：关注趋势与主要热点。

5. **`go tool pprof` 找不到**：环境里没有安装。  
   解决：使用官方 Go 发行版，确保 `go tool pprof` 可用。

6. **只盯 CPU**：忽略内存分配热点。  
   解决：CPU 与 Heap 一起看。

补充建议：热点函数不一定要“彻底消除”，先评估它是否符合业务预期，再考虑优化成本与收益。

配图建议：
- 一张“常见坑清单”图。
- 一张“CPU/Heap 联合分析”示意图。

## 6. 进阶扩展 / 思考题

1. 给程序加上 `net/http/pprof`，实现在线抓取。
2. 尝试用 `-bench` + `-cpuprofile` 生成测试基准的 profile。
3. 用 `pprof -top -cum` 看累计耗时，和 `-top` 对比。
4. 观察不同数据规模下热点是否发生变化。

配图建议：
- 一张“HTTP pprof 调用路径”图。
- 一张“累计耗时 vs 自身耗时”对比图。
