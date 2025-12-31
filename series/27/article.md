# 文件与 IO：io.Reader / Writer 的威力

你好，我是汪小成。你有没有写过这样的 IO 代码：读文件先 `ReadAll`，再一口气写出；或者为了处理日志，把文件整个读入内存，结果线上一跑就 OOM。Go 标准库里真正“无处不在”的能力，其实是 `io.Reader` / `io.Writer` 这对接口。它们让你在不关心数据来源和去向的情况下，安全地处理大文件、网络流、压缩流、加密流。本文会从痛点场景出发，解释 Reader/Writer 的设计逻辑，给出完整可运行示例，并补齐工程中的常见坑与修复策略。

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
- 本篇目录：`series/27`。
- 示例入口：`series/27/cmd/ioflow/main.go`。

### 1.2 运行命令

```bash
go run ./series/27/cmd/ioflow
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/27/cmd/ioflow
```

### 1.3 前置知识

- string/[]byte/rune 的基本理解（第 17 篇）。
- Go 中错误处理的常见写法（`if err != nil`）。

配图建议：
- 一张“Reader → Writer 流式管道图”（输入源→中间处理→输出端）。
- 一张“流式处理 vs ReadAll”的对比图（内存占用差异）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 Reader / Writer 是“流式接口”

**概念**：`io.Reader` 只定义一个方法 `Read(p []byte) (n int, err error)`；`io.Writer` 只定义一个方法 `Write(p []byte) (n int, err error)`。  
**示例**：文件、网络连接、bytes.Buffer、strings.Reader 都实现了 Reader/Writer。  
**为什么这么设计**：最小接口原则，让任何“能读/能写”的东西都能组合在一起。

你可以把 Reader/Writer 看成“数据流标准”：上游只管读、下游只管写，细节交给实现。这样一来，`io.Copy`、`io.TeeReader`、`io.MultiWriter` 等工具就能跨场景复用。

### 2.2 io.Copy：最常用的“零心智负担”

**概念**：`io.Copy(dst, src)` 持续从 src 读，再写入 dst，直到 EOF。  
**示例**：把 `strings.Reader` 的内容 copy 到 `bytes.Buffer` 或文件。  
**为什么这么设计**：流式复制能避免一次性读入内存，特别适合大文件或网络流。

在工程里，`io.Copy` 是最安全的默认选择：它用内部 buffer 自动分块读写，减少你手写循环的错误。

### 2.3 TeeReader / MultiWriter：一份数据，多路去向

**概念**：

- `io.TeeReader`：读取时把数据同时写到另一个 Writer（例如计算 hash 或日志）。
- `io.MultiWriter`：写入时同时写到多个 Writer（例如写文件 + 写内存 + 写网络）。

**为什么这么设计**：这些工具把“旁路处理”变成组合，而不是复制数据、重复写逻辑。

### 2.4 LimitReader / Pipe：控制与流式协作

**概念**：

- `io.LimitReader`：只允许读取前 N 字节，用于预览/限制。
- `io.Pipe`：把写端当成 Reader，读端当成 Writer，适合流式处理和解耦生产/消费。

**为什么这么设计**：让你在不创建临时文件、不额外拷贝的情况下连接两个流程。

### 2.5 bufio：减少系统调用，提升吞吐

**概念**：`bufio.Reader/Writer` 在 Reader/Writer 外面加一层缓冲，减少频繁系统调用。  
**示例**：从 pipe 读取时用 bufio.Reader，写入时用 bufio.Writer。  
**为什么这么设计**：系统调用成本高，缓冲能显著提升吞吐，尤其在小块读写场景。

配图建议：
- 一张“工具矩阵”：Copy / TeeReader / MultiWriter / LimitReader / Pipe / bufio 的使用场景。
- 一张“Reader/Writer 组合链路”示意（Reader → Tee → Limit → Writer）。

## 3. 完整代码示例（可运行）

示例包含 5 个部分：

1. strings.Reader → bytes.Buffer 的 io.Copy  
2. TeeReader：一边 copy，一边计算 hash  
3. LimitReader：只读前 10 字节  
4. MultiWriter：写文件 + 写内存  
5. Pipe：模拟流式生产/消费  

代码路径：`series/27/cmd/ioflow/main.go`。

## 4. 运行效果 + 截图描述

运行：

```bash
go run ./series/27/cmd/ioflow
```

典型输出（节选）：

```
--- 1) Copy string reader to buffer ---
copied=30 err=<nil>
buffer="hello io.Reader and io.Writer\n"

--- 3) LimitReader: preview ---
preview="this is a "

--- 4) MultiWriter: file + buffer ---
bytes=25 err=<nil> file=series/27/tmp/output.txt
buffer="write to file and buffer\n"

--- 5) Pipe: streaming producer/consumer ---
recv="line-1"
recv="line-2"
recv="line-3"
lines=3
```

截图建议（每 500 字 1~2 张）：

- 截图 1：Copy + LimitReader 输出（展示“流式 + 预览”）。
- 截图 2：MultiWriter 写文件/内存的输出（展示“一份数据，多路去向”）。
- 截图 3：Pipe 的 streaming 输出（展示“生产者/消费者解耦”）。

## 5. 常见坑 & 解决方案（必写）

1. **直接 ReadAll 导致内存暴涨**：对大文件/大响应一次性读入。解决：优先使用 io.Copy/分块读取，必要时加 LimitReader。
2. **忘记关闭文件**：写文件后不 Close，数据丢失或句柄泄漏。解决：创建后立刻 `defer f.Close()`。
3. **忽略 bufio.Flush**：写入 bufio.Writer 后未 Flush，结果没落盘。解决：写入完成后 Flush，或使用 `defer w.Flush()`。
4. **TeeReader 重复读取**：以为 TeeReader 可以“复读”数据，结果数据被消费后无法重来。解决：需要复读就缓存到 buffer 或文件。
5. **MultiWriter 的错误处理忽略**：任何一个 Writer 返回错误都会中断。解决：检查返回值，必要时拆分多个写入或对错误做降级策略。
6. **LimitReader 误当“安全截断”**：LimitReader 只限制字节，不保证 UTF-8 边界。解决：文本场景按 rune 或 UTF-8 边界截断（参考第 17 篇）。
7. **Pipe 不 Close 导致读端阻塞**：写端 goroutine 退出但未 Close，读端永远等。解决：写端必须 Close/CloseWithError。
8. **在热路径频繁创建 buffer**：导致分配过多。解决：复用 buffer（sync.Pool）或用固定大小的工作区。

配图建议：
- “ReadAll 失败示意图”（内存曲线飙升）。
- “Pipe 未 Close 导致阻塞”时序图。

## 6. 进阶扩展 / 思考题

- 把示例扩展为“读取文件 → TeeReader 计算 hash → MultiWriter 写入新文件 + stdout”，练习组合链路。
- 用 `io.CopyBuffer` 自己提供 buffer，观察对性能的影响。
- 给 MultiWriter 增加错误降级：一个 Writer 失败时记录日志但不阻断主流程。
- 写一个“限速 Reader”：每读 N 字节 sleep 一下，模拟限速下载。
- 思考题：你的业务场景里哪些操作是“流式”的？哪些是“必须全量读入”的？如何取舍？

---

io.Reader/io.Writer 的价值在于：它们把“数据来源”和“数据去向”解耦，让流式处理成为默认选项。Copy/Tee/Multi/Pipe/Limit 这些工具是可组合积木，几乎可以覆盖所有 IO 处理需求。把本文示例跑一遍，再回头看看你的代码：哪里还在 ReadAll？哪里能用流式管道省内存？这些优化往往是低风险、高收益的。
