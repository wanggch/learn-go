# benchmark：你测的真的是性能吗

你好，我是汪小成。很多人第一次写基准测试，以为跑出一串 `ns/op` 就能下结论，结果线上还是慢：要么测到了编译器优化后的“假快”，要么把初始化开销算进了性能。基准测试不是跑一下就行，它需要正确的结构、合理的对比和可信的指标。本文会先准备环境，再解释 benchmark 的关键概念与设计原因，最后给出完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/35`。
- 示例入口：`series/35/format`。

### 1.2 运行命令

```bash
go test ./series/35/format -bench . -benchmem -run=^$
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go test ./series/35/format -bench . -benchmem -run=^$
```

### 1.3 前置知识

- 了解 `go test` 基本用法。
- 了解字符串拼接的基础写法。

提示：benchmark 不是“跑一次就信”，你需要多跑几次、对比结果，再决定优化方向，也能避免误判，更可靠。

配图建议：
- 一张“测试 vs 基准测试”的对比图。
- 一张“指标解释：ns/op、B/op、allocs/op”的示意图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `b.N` 是基准测试的节拍器

**概念**：benchmark 通过 `b.N` 自动放大循环次数，保证测试足够稳定。  
**示例**：`for i := 0; i < b.N; i++ { ... }`。  
**为什么这么设计**：避免你手动设置次数导致统计波动过大。

### 2.2 `-benchmem` 是性能问题的“X 光”

**概念**：默认只看耗时，`-benchmem` 会补充内存与分配次数。  
**示例**：`allocs/op` 能一眼发现字符串拼接导致的分配暴涨。  
**为什么这么设计**：性能瓶颈往往不是时间，而是分配。

### 2.3 `b.ResetTimer` 把准备工作排除掉

**概念**：初始化数据不应该算进耗时。  
**示例**：构造 `words` 之后再 `b.ResetTimer()`。  
**为什么这么设计**：保证只测“核心逻辑”。

### 2.4 防止编译器“聪明过头”

**概念**：如果结果没人使用，编译器可能优化掉你的代码。  
**示例**：使用 `sink` 保存结果。  
**为什么这么设计**：让基准测试真的测到执行成本。

### 2.5 子 benchmark 让对比更清晰

**概念**：`b.Run` 可以把同类方案放到一组对比里。  
**示例**：`plus` / `builder` / `join` 三种方式。  
**为什么这么设计**：更容易定位“哪个方案更快”。

### 2.6 指标不是越小越好

**概念**：`ns/op` 要结合 `B/op` 和 `allocs/op` 一起看。  
**示例**：时间差不大，但分配数差别巨大。  
**为什么这么设计**：内存分配会影响 GC，最终影响整体吞吐。

### 2.7 `b.StopTimer` 让数据更干净

**概念**：某些步骤只需要在每轮之间执行，不应计入耗时。  
**示例**：在循环内偶尔重置输入时，用 `b.StopTimer`/`b.StartTimer` 包住。  
**为什么这么设计**：把“准备成本”和“核心成本”分离，结果更可信。

### 2.8 关注“趋势”，而不是“绝对值”

**概念**：不同机器、不同负载下结果会波动。  
**示例**：只要 `Builder` 明显优于 `+`，趋势就成立。  
**为什么这么设计**：性能优化更关注方案之间的相对差距。
配图建议：
- 一张“基准测试流程图”。
- 一张“指标三元组”关系图。

## 3. 完整代码示例（可运行）

示例对比三种字符串拼接方式：

1. `+` 直接拼接。
2. `strings.Builder`。
3. `strings.Join`。

代码路径：`series/35/format`。

```go
package format

import "strings"

func BuildPlus(words []string) string {
	var s string
	for _, w := range words {
		s += w
	}
	return s
}

func BuildBuilder(words []string) string {
	var b strings.Builder
	need := 0
	for _, w := range words {
		need += len(w)
	}
	b.Grow(need)
	for _, w := range words {
		b.WriteString(w)
	}
	return b.String()
}

func BuildJoin(words []string) string {
	return strings.Join(words, "")
}
```

```go
package format

import (
	"strconv"
	"testing"
)

var sink string

func BenchmarkBuild(b *testing.B) {
	small := makeWords(32, 6)
	large := makeWords(1024, 6)

	b.ReportAllocs()

	bench := func(name string, words []string, fn func([]string) string) {
		b.Run(name, func(b *testing.B) {
			var out string
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out = fn(words)
			}
			sink = out
		})
	}

	bench("plus/small", small, BuildPlus)
	bench("plus/large", large, BuildPlus)
	bench("builder/small", small, BuildBuilder)
	bench("builder/large", large, BuildBuilder)
	bench("join/small", small, BuildJoin)
	bench("join/large", large, BuildJoin)
}

func makeWords(n, size int) []string {
	words := make([]string, n)
	for i := 0; i < n; i++ {
		words[i] = "w" + strconv.Itoa(i%10) + repeat("x", size)
	}
	return words
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = s[0]
	}
	return string(out)
}
```

说明：`sink` 用来防止编译器优化掉结果；`small/large` 用于对比不同规模的差异。

小建议：如果你怀疑输入规模影响明显，可以再加一组 `medium`，或按业务真实请求规模调整 `n` 和 `size`。

配图建议：
- 一张“不同拼接方式对比表”。
- 一张“small vs large”对比图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go test ./series/35/format -bench . -benchmem -run=^$
```

示例输出（节选）：

```
goos: darwin
goarch: amd64
pkg: learn-go/series/35/format
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkBuild/plus/small-12          	 664088	    1754 ns/op	    4328 B/op	      31 allocs/op
BenchmarkBuild/plus/large-12          	   1233	  907548 ns/op	 4470536 B/op	    1023 allocs/op
BenchmarkBuild/builder/small-12       	 4597472	     252.8 ns/op	     256 B/op	       1 allocs/op
BenchmarkBuild/builder/large-12       	  145398	    7619 ns/op	    8192 B/op	       1 allocs/op
BenchmarkBuild/join/small-12          	 3367238	     364.0 ns/op	     256 B/op	       1 allocs/op
BenchmarkBuild/join/large-12          	  119653	   10047 ns/op	    8192 B/op	       1 allocs/op
PASS
ok  	learn-go/series/35/format	8.809s
```

输出解读：`+` 拼接在大规模下分配次数爆炸；`Builder` 和 `Join` 都能把分配控制在 1 次左右。`ns/op` 的差异反映了分配成本与复制开销的差距。

补充说明：由于机器与负载不同，你的具体数值会有差异，但趋势应该一致。关注趋势比纠结某一次的绝对值更重要。

截图描述建议：
- 截一张终端输出图，突出 **allocs/op** 的巨大差异。
- 再截一张“plus vs builder”对比图，强调大规模差异。

配图建议：
- 一张“分配次数对比柱状图”。
- 一张“性能随规模变化曲线”。

## 5. 常见坑 & 解决方案（必写）

1. **把初始化时间算进 benchmark**：结果虚高。  
   解决：用 `b.ResetTimer()` 排除准备工作。

2. **结果被编译器优化掉**：看起来很快，实际上没跑。  
   解决：把结果写入 `sink` 或返回值。

3. **只看 ns/op**：忽略分配导致 GC 问题。  
   解决：加 `-benchmem` 同时关注 B/op 与 allocs/op。

4. **样本过小**：小数据让方案差异被掩盖。  
   解决：准备 small/large 两组规模对比。

5. **日志打印过多**：I/O 成本盖过逻辑成本。  
   解决：benchmark 内不要输出日志。

6. **缓存干扰**：同一结果反复跑差异很大。  
   解决：用 `-count=1` 多跑几次取平均。

配图建议：
- 一张“常见坑清单”图。
- 一张“指标误读”示意图。

## 6. 进阶扩展 / 思考题

1. 用 `-benchtime=3s` 拉长时间，观察结果是否更稳定。
2. 用 `-count=5` 跑多次，并用 `benchstat` 生成对比报告。
3. 在 `BuildBuilder` 中移除 `Grow`，比较分配变化。
4. 试着把 `small/large` 改成真实业务数据规模。

配图建议：
- 一张“benchstat 报告示例”。
- 一张“优化前后对比”图。
