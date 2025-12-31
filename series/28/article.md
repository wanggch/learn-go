# bufio：为什么加一层 buffer 差这么多

你好，我是汪小成。你可能遇到过这种“性能怪事”：同样读取一个文件，别人用 Go 跑得飞快，你的程序却慢得像蜗牛。排查半天发现只是读写方式不同——你用 `Read` 读小块，别人用 `bufio.Reader`。IO 不是 CPU 计算，慢往往慢在系统调用和切换成本上，而 buffer 就是最直接的“降本神器”。本文会先讲环境与前提，再解释 bufio 的设计逻辑，最后给出完整示例、运行效果、常见坑与进阶练习。

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
- 本篇目录：`series/28`。
- 示例入口：`series/28/cmd/buflab/main.go`。

### 1.2 运行命令

```bash
go run ./series/28/cmd/buflab -lines=2000 -size=80
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/28/cmd/buflab -lines=2000 -size=80
```

### 1.3 前置知识

- `io.Reader` / `io.Writer` 的流式概念（第 27 篇）。
- `os.Open` / `os.Create` 的基本用法。

配图建议：
- 一张“系统调用次数 vs 吞吐”的示意图（小块读写导致调用次数暴增）。
- 一张“bufio 位置”图（Reader → bufio → 业务处理）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 bufio 解决的核心问题：系统调用太贵

**概念**：直接对文件 `Read`/`Write`，每次调用都会触发系统调用。小块读写会导致“系统调用风暴”。  
**示例**：每次只读 64 字节，读 160KB 就要触发 2500 次系统调用。  
**为什么这么设计**：IO 是慢设备，系统调用是昂贵边界。缓冲把多次小读写合并成少量大读写。

### 2.2 bufio.Reader / Writer 的行为

**概念**：Reader 会一次读入较大块数据放入内存 buffer，后续小读直接从内存取；Writer 会先写到 buffer，积累到一定量再一次性写入底层。  
**示例**：`bufio.NewReaderSize(f, 32*1024)` 让 32KB 的块进入用户态，避免频繁进入内核。  
**为什么这么设计**：把高成本的系统调用数量降下来，通常吞吐会显著提升。

### 2.3 Scanner/Reader 的边界选择

**概念**：`bufio.Scanner` 适合按行/按 token 读取，但默认 token 上限是 64K；`bufio.Reader` 更通用。  
**示例**：日志行超过 64K，Scanner 会报错，需要 `Buffer` 调整上限或改用 Reader。  
**为什么这么设计**：Scanner 提供便利但设置了安全默认值，避免巨大 token 导致内存爆炸。

### 2.4 buffer 大小不是越大越好

**概念**：更大的 buffer 并不一定更快，还可能增加内存。  
**示例**：读 160KB 文件，用 32KB buffer 已足够；盲目设为 8MB 反而浪费。  
**为什么这么设计**：工程优化追求“足够大且不浪费”，而不是“最大化”。

配图建议：
- 一张“Read 小块 vs bufio 大块”的系统调用次数对比图。
- 一张“Scanner vs Reader”使用场景表。

## 3. 完整代码示例（可运行）

示例做四组对比：

1. `bufio.Scanner` 按行读取  
2. `io.ReadAll` 一次性读入  
3. 小块 `Read`（无缓冲）  
4. 小块 `Read` + `bufio.Reader`  

代码路径：`series/28/cmd/buflab/main.go`。

```go
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type config struct {
	lines int
	size  int
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== bufio：缓冲带来的差异 ===")
	fmt.Printf("lines=%d size=%d bytes\n", cfg.lines, cfg.size)

	tmpDir := filepath.Join("series", "28", "tmp")
	_ = os.MkdirAll(tmpDir, 0o755)
	path := filepath.Join(tmpDir, "data.txt")

	if err := writeFile(path, cfg.lines, cfg.size); err != nil {
		fmt.Println("write error:", err)
		return
	}

	section("1) 逐行读取：bufio.Scanner", func() {
		n, cost, err := readWithScanner(path)
		fmt.Printf("lines=%d cost=%s err=%v\n", n, cost, err)
	})

	section("2) 直接 ReadAll：io.ReadAll", func() {
		n, cost, err := readAll(path)
		fmt.Printf("bytes=%d cost=%s err=%v\n", n, cost, err)
	})

	section("3) 小块读取：io.ReadAtLeast (no buffer)", func() {
		n, cost, err := readSmallChunks(path, 64)
		fmt.Printf("bytes=%d cost=%s err=%v\n", n, cost, err)
	})

	section("4) bufio.Reader：小块读取 + 缓冲", func() {
		n, cost, err := readBuffered(path, 64, 32*1024)
		fmt.Printf("bytes=%d cost=%s err=%v\n", n, cost, err)
	})
}

func parseFlags() config {
	var cfg config
	flag.IntVar(&cfg.lines, "lines", 2000, "lines to generate")
	flag.IntVar(&cfg.size, "size", 80, "bytes per line")
	flag.Parse()
	if cfg.lines < 1 {
		cfg.lines = 1
	}
	if cfg.size < 1 {
		cfg.size = 1
	}
	return cfg
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func writeFile(path string, lines int, size int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 32*1024)
	line := bytes.Repeat([]byte("x"), size-1)
	line = append(line, '\n')

	for i := 0; i < lines; i++ {
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return w.Flush()
}

func readWithScanner(path string) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, time.Since(start), scanner.Err()
}

func readAll(path string) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	b, err := io.ReadAll(f)
	if err != nil {
		return 0, 0, err
	}
	return len(b), time.Since(start), nil
}

func readSmallChunks(path string, chunk int) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	buf := make([]byte, chunk)
	total := 0
	for {
		n, err := f.Read(buf)
		total += n
		if err != nil {
			if err == io.EOF {
				break
			}
			return total, time.Since(start), err
		}
	}
	return total, time.Since(start), nil
}

func readBuffered(path string, chunk int, bufSize int) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	br := bufio.NewReaderSize(f, bufSize)
	buf := make([]byte, chunk)
	total := 0
	for {
		n, err := br.Read(buf)
		total += n
		if err != nil {
			if err == io.EOF {
				break
			}
			return total, time.Since(start), err
		}
	}
	return total, time.Since(start), nil
}
```

## 4. 运行效果 + 截图描述

运行：

```bash
go run ./series/28/cmd/buflab -lines=2000 -size=80
```

典型输出（节选，数值与机器有关，关注相对关系）：

```
--- 1) 逐行读取：bufio.Scanner ---
lines=2000 cost=207µs

--- 3) 小块读取：io.ReadAtLeast (no buffer) ---
bytes=160000 cost=3.29ms

--- 4) bufio.Reader：小块读取 + 缓冲 ---
bytes=160000 cost=82µs
```

截图建议（每 500 字 1~2 张）：

- 截图 1：无缓冲小块读取 vs bufio 缓冲的耗时对比。
- 截图 2：Scanner 与 ReadAll 的对比输出（强调“便捷 vs 内存”）。
- 截图 3：画一个“系统调用次数减少”的示意图。

## 5. 常见坑 & 解决方案（必写）

1. **Scanner token 超限**：默认 64K，超出会报错。解决：`scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)` 或改用 Reader。
2. **忘记 Flush**：bufio.Writer 不 Flush 导致数据没写入。解决：写完后 `Flush()`，或 `defer w.Flush()`。
3. **buffer 太小或太大**：太小系统调用多，太大浪费内存。解决：从 32KB 起步，结合基准测试调整。
4. **ReadAll 误用**：大文件/大响应一次性读入内存。解决：优先流式处理，必要时加 LimitReader。
5. **混用 bufio.Reader 与底层文件读**：直接对 `f.Read` 和 `bufio.Reader.Read` 混用，会导致数据错乱。解决：选择一种接口，别混用。
6. **忽略错误处理**：Read/Write 返回的 n 和 err 需要同时检查。解决：遵循 `n>0` 仍可能有 err 的约定。
7. **Scanner 当 JSON 解析器**：Scanner 只按行/分隔符读，不负责复杂协议。解决：大块协议用 Reader + 自定义解析。
8. **缓冲误当缓存**：buffer 只是读写缓冲，不是持久缓存。解决：需要缓存就用 cache 结构或文件。

配图建议：
- “Scanner 超限”错误示意图。
- “混用 Reader 导致错位”的示意图。

## 6. 进阶扩展 / 思考题

- 让 `readBuffered` 的 `bufSize` 可配置，跑三组数据（8KB / 32KB / 128KB），观察耗时变化。
- 把读取逻辑改成写入逻辑，对比“无缓冲写小块”和“bufio.Writer”。
- 结合 `io.CopyBuffer`，看看手动 buffer 是否更快。
- 思考题：你的服务里哪些 IO 路径适合加 bufio？哪些不适合（比如极小数据/一次性操作）？
- 思考题：你会如何在生产环境观测“系统调用过多”导致的 IO 变慢？

---

bufio 的核心价值是：**用更大的用户态缓冲换更少的系统调用**。当你的 IO 由大量小读写组成时，bufio 的收益往往是数量级的。把这层 buffer 加到正确的地方，再配合合理的大小和正确的 Flush/Close，你就能在不改业务逻辑的情况下获得稳定的吞吐提升。建议你用本文示例跑一遍，感受“同样数据，不同读法”的差距，然后回到自己的项目里挑一条 IO 热路径试试。
