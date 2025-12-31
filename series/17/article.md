# string / []byte / rune：编码与性能的真相

大家好，我是汪小成。你可能遇到过这样的“文字类线上事故”：  
日志里用户名被截断成“你�”；短信模板里“😊”显示成问号；或者你以为 `len(s)` 是字符数，结果分页/截断把 UTF-8 切烂，用户投诉“内容乱码”。Go 的字符串看起来简单，其实背后是 UTF-8 编码、不可变语义、以及 byte/rune 的多层抽象。本文会把这些坑一次讲透：先讲环境与前置知识，再用“概念→示例→为什么这么设计”的方式解释核心点，最后给出完整示例、运行效果、常见坑与进阶练习。

## 目录

1. 环境准备 / 前提知识  
2. 核心概念解释（概念 → 示例 → 为什么这么设计）  
3. 完整代码示例（可运行）  
4. 运行效果 + 截图描述  
5. 常见坑 & 解决方案（必写）  
6. 进阶扩展 / 思考题  

## 1. 环境准备 / 前提知识

### 1.1 版本与目录

- Go 1.22+（本仓库用 `go.work` 管理多模块）。
- 本篇目录：`series/17`。
- 示例入口：`series/17/cmd/textlab/main.go`。

### 1.2 运行命令

```bash
go run ./series/17/cmd/textlab
```

如果你在沙盒环境里遇到 Go build cache 权限问题，可用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/17/cmd/textlab
```

### 1.3 需要的最小知识

- 知道 UTF-8 是可变长度编码：一个“字符”可能占 1~4 字节。
- 理解 Go 的 `string` 是不可变的字节序列（通常存 UTF-8 文本，但语言层面并不强制）。
- 知道 `rune` 是 `int32`，表示一个 Unicode code point（不等同于“用户感知字符”）。

配图建议：
- 一张“UTF-8 编码长度示意”（ASCII 1 字节、汉字 3 字节、emoji 4 字节）。
- 一张“string / []byte / rune 关系图”（string=只读 bytes；[]byte=可变；rune=码点）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `string`：不可变的字节序列

**概念**：Go 的 `string` 是一段只读字节序列。不可变意味着：你不能原地修改其中某个字节或字符。  
**示例**：把 `string` 转成 `[]byte` 后修改，原字符串不会变（因为发生了拷贝）。  
**为什么这么设计**：

- 不可变字符串可以安全共享（多个地方引用同一底层数据，不担心被悄悄修改）。
- 作为 map key 更稳定（不会因为内容改变导致哈希/等值关系变化）。
- 让编译器与运行时更容易做优化（例如常量字符串、切片引用等）。

### 2.2 `len(s)` 不是字符数：它是字节数

**概念**：`len(string)` 返回底层字节长度。对 UTF-8 文本来说，它不是“字符数”。  
**示例**：`"你好"` 的 `len` 是 6，因为每个汉字占 3 字节；但 rune 数是 2。  
**为什么这么设计**：

- Go 把编码细节交给你控制：`len` 是对底层表示的直接反映，性能稳定、含义明确。
- “字符数”的定义并不唯一：code point、grapheme cluster（用户感知字符）都可能不同，语言不替你做隐式选择。

业务里最常见的错误是：**用 `len(s)` 做“显示长度/截断长度”**，导致把 UTF-8 切成半个 rune，出现 `�` 或 `valid_utf8=false`。

### 2.3 `rune` 与 `for range`：你以为在按字符遍历，其实更精确

**概念**：`for i, r := range s` 会按 UTF-8 解码遍历字符串，`r` 是 rune（码点），`i` 是该 rune 的**字节下标**。  
**示例**：遍历 `"你a好😊"` 时，下标会跳：0、3、4、7，因为 rune 的字节长度不同。  
**为什么这么设计**：

- 让你既能拿到“语义层”的码点，又能保留“底层层”的字节位置（对切片、截断、日志定位很有价值）。
- 避免你手动写 UTF-8 解码器；但同时不隐藏 byte 下标的真实性。

注意：`rune` ≠ 用户感知字符（组合 emoji 等可能由多个 rune 组成），UI 显示宽度场景要更谨慎。

### 2.4 `[]byte`：可变、适合 IO 与协议

**概念**：`[]byte` 是可变字节序列。网络读写、文件 IO、编码/解码通常以 bytes 为中心。  
**示例**：你从 `net/http` 读取 body 得到 bytes，解析/过滤后再决定是否转成 string。  
**为什么这么设计**：

- IO 是字节世界；`[]byte` 能原地修改、复用 buffer，减少分配。
- 与编码库（JSON、压缩、加密）更自然地协作。

但你也要记住：`string(b)` 通常会拷贝（从可变 bytes 到不可变 string），这可能在热路径上产生大量分配。是否拷贝、是否值得，要基于场景判断。

### 2.5 “安全截断”两种思路：按 rune vs 按字节边界

**概念**：截断的核心是“不切断 UTF-8”。你有两条路：

1. 按 rune 截断：把 string 转 `[]rune` 后切片，语义最直观，但会分配 rune 数组（每个 rune 4 字节），成本更高。
2. 按 UTF-8 边界截断：按字节限制找到最近的 rune 边界，避免切断 rune，通常更省分配。

**为什么这么设计**：Go 给你工具（`utf8` 包）让你在“正确性”和“性能”之间做选择，而不是隐式地替你决定。

配图建议：
- 一张 “range 下标跳跃”图（标注每个 rune 的字节长度）。
- 一张“截断策略”对比表（按 rune：简单但分配；按边界：省分配但更复杂）。

## 3. 完整代码示例（可运行）

本文示例做了 5 个小实验：`len vs runeCount`、`range` 真相、字节切片坑、安全截断、`[]byte <-> string` 的拷贝与选择。代码在：`series/17/cmd/textlab/main.go`。

```go
package main

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

func main() {
	fmt.Println("=== string / []byte / rune：编码与性能演示 ===")

	section("1) len vs rune 数", demoLenAndRuneCount)
	section("2) range 遍历 string 的真相", demoRangeOverString)
	section("3) 按字节切片会发生什么", demoByteSlicingPitfall)
	section("4) 安全截断：按 rune 或按字节边界", demoSafeTruncate)
	section("5) []byte 与 string 的拷贝与场景选择", demoBytesAndString)
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func demoLenAndRuneCount() {
	s1 := "hello"
	s2 := "你好"
	s3 := "Go😊"

	fmt.Printf("%q: len=%d bytes, rune=%d\n", s1, len(s1), utf8.RuneCountInString(s1))
	fmt.Printf("%q: len=%d bytes, rune=%d\n", s2, len(s2), utf8.RuneCountInString(s2))
	fmt.Printf("%q: len=%d bytes, rune=%d\n", s3, len(s3), utf8.RuneCountInString(s3))

	fmt.Println("结论：len(string) 是字节数，不是字符数。")
}

func demoRangeOverString() {
	s := "你a好😊"
	fmt.Printf("source: %q\n", s)
	fmt.Println("range 输出：index 是字节下标，r 是 rune（Unicode code point）")
	for i, r := range s {
		fmt.Printf("  i=%d r=%U char=%q\n", i, r, r)
	}
}

func demoByteSlicingPitfall() {
	s := "你好世界"
	fmt.Printf("source: %q len=%d\n", s, len(s))

	bad := s[:4]
	fmt.Printf("bad slice s[:4]=%q (valid_utf8=%v)\n", bad, utf8.ValidString(bad))

	good := s[:6]
	fmt.Printf("good slice s[:6]=%q (valid_utf8=%v)\n", good, utf8.ValidString(good))
}

func demoSafeTruncate() {
	s := "Go 语言真香😊，但编码要小心"
	fmt.Printf("source: %q\n", s)

	fmt.Println("按 rune 截断（语义正确，但可能有额外分配）：")
	fmt.Printf("  truncateRunes(8) -> %q\n", truncateRunes(s, 8))

	fmt.Println("按 UTF-8 边界截断（不切断 rune）：")
	fmt.Printf("  truncateUTF8Bytes(10 bytes) -> %q\n", truncateUTF8Bytes(s, 10))
	fmt.Printf("  truncateUTF8Bytes(13 bytes) -> %q\n", truncateUTF8Bytes(s, 13))
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	rs := []rune(s)
	return string(rs[:n])
}

func truncateUTF8Bytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	b := []byte(s)
	i := maxBytes
	for i > 0 && !utf8.FullRune(b[:i]) {
		i--
	}
	if i == 0 {
		return ""
	}
	for !utf8.Valid(b[:i]) {
		i--
		if i == 0 {
			return ""
		}
	}
	return string(b[:i])
}

func demoBytesAndString() {
	s := "abc你好"
	b := []byte(s) // makes a copy
	fmt.Printf("string -> []byte: %q -> %v (len=%d)\n", s, b, len(b))

	b[0] = 'A'
	fmt.Printf("改 b[0]='A' 后：b=%v，string 仍是 %q\n", b, s)

	s2 := string(b) // makes a copy
	fmt.Printf("[]byte -> string: %v -> %q\n", b, s2)

	fmt.Println("bytes.Buffer/Builder 的典型用途：拼接时减少中间对象")
	var buf bytes.Buffer
	buf.Grow(32)
	buf.WriteString("id=")
	buf.WriteString("1001")
	buf.WriteString(" msg=")
	buf.WriteString(s)
	fmt.Printf("buffer -> %q\n", buf.String())
}
```

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/17/cmd/textlab
```

典型输出（节选）：

```
--- 1) len vs rune 数 ---
"hello": len=5 bytes, rune=5
"你好": len=6 bytes, rune=2
"Go😊": len=6 bytes, rune=3

--- 3) 按字节切片会发生什么 ---
source: "你好世界" len=12
bad slice s[:4]="你\xe5" (valid_utf8=false)
good slice s[:6]="你好" (valid_utf8=true)
```

截图建议：

- 截图：`range` 的 index 跳跃（0、3、4、7）与 `valid_utf8=false` 的对比输出。
- 截图：截断策略对比（按 rune vs 按 UTF-8 边界），标注“语义/分配/复杂度”。

## 5. 常见坑 & 解决方案（必写）

1. **用 `len(s)` 当字符数**：分页、截断、校验都错。解决：用 `utf8.RuneCountInString`（码点数）或更高层的“用户感知字符”方案（UI 场景）。
2. **按字节切片导致乱码**：`s[:n]` 切断 rune。解决：按 rune 截断，或按 UTF-8 边界截断（用 `utf8` 包）。
3. **`range` 的 index 不是字符下标**：它是字节下标。解决：需要“第 k 个 rune”时用计数器，不要拿 index 当字符索引。
4. **`[]rune(s)` 盲目使用**：对大文本会产生大量分配和内存占用（每个 rune 4 字节）。解决：只在必要时转 rune；热路径用 `range` 或按字节边界扫描。
5. **频繁 `[]byte` ↔ `string` 转换**：在循环里转换会产生大量分配。解决：在一个表示上完成处理；拼接/构造时用 builder/buffer；最后一次性转换。
6. **把“rune=字符”当成绝对真理**：某些 emoji/组合字符由多个 rune 组成，显示宽度也可能不同。解决：业务规则明确要的是什么（码点数、字节数、显示宽度），再选算法。
7. **日志/协议对编码假设不一致**：上游按 GBK、下游按 UTF-8，或者把任意 bytes 当 UTF-8 打印。解决：边界处明确编码；对不可信输入先 `utf8.Valid` 校验或替换策略。

配图建议：一张“坑点→症状→修复”的表格；一张“rune 与用户感知字符不同”的示意（用组合 emoji 举例即可）。

## 6. 进阶扩展 / 思考题

- 写一个 `IndexRune(s string, k int) (byteIndex int, ok bool)`：返回第 k 个 rune 对应的字节下标，练习 range 与计数。
- 改造 `truncateUTF8Bytes`：在保留 UTF-8 有效性的同时，追加省略号（`...`），并确保总字节不超限。
- 用 `pprof` 对比两种截断策略在长文本上的分配差异：`[]rune` vs 边界截断。
- 思考题：你的业务“长度限制”到底应该限制什么？（字节？码点？显示宽度？）不同答案会导致不同实现和不同成本。

---

string/[]byte/rune 的关键在于：`string` 是不可变字节序列，`len` 是字节长度，`range` 给你 rune 和字节下标，截断要避免切断 UTF-8。把这些底层事实掌握住，你就能避免大多数乱码事故，同时在需要时写出更省分配、更高性能的文本处理代码。
