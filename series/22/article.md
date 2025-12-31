# select：写出不会死锁的并发代码

你好，我是汪小成。你有没有遇到过这种并发现场：线上某个请求突然卡死，一堆 goroutine 都停在 channel 读写；或者你写了“等两个结果谁先回来就用谁”，结果偶发超时、偶发泄漏。并发代码难，很多时候是“等待”不够明确：等谁、等多久、等不到怎么办。`select` 就是 Go 的“等待控制器”：把多个等待点放在同一个位置，让你显式表达 **超时、取消、默认分支、多路复用**。

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
- 本篇目录：`series/22`。
- 示例入口：`series/22/cmd/selectlab/main.go`。

### 1.2 运行命令

```bash
go run ./series/22/cmd/selectlab -timeout=80ms
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/22/cmd/selectlab -timeout=80ms
```

### 1.3 前置知识

- goroutine 与 channel 的基本概念。
- 知道 `context.WithTimeout` 用于超时/取消。

配图建议：
- 一张“多个等待点汇聚到 select”的示意图（结果A/结果B/超时/取消）。
- 一张“无超时等待 vs 有超时等待”的对比图（卡死 vs 可控退出）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 select 是什么：多路等待（multiplexing）

**概念**：`select` 让你同时等待多个 channel 操作（收/发）。当有一个 case 就绪，就执行对应分支。  
**示例**：等待 `resultCh` 或 `time.After(timeout)`，谁先到就走谁。  
**为什么这么设计**：并发程序的痛点不是“开 goroutine”，而是“等待的不确定性”。select 把不确定性集中在一个位置，便于读、便于测试、便于加超时/取消。

### 2.2 超时：select + time.After（或 Timer）

**概念**：当你等待某个结果时，必须考虑“等不到怎么办”。超时是并发系统的保险丝。  
**示例**：`select { case <-work: ...; case <-time.After(80ms): ... }`。  
**为什么这么设计**：Go 选择把超时写成显式分支，而不是隐藏在调用栈里。这样你读到 select，就能马上看到这个等待是“可控”的。

工程提示：`time.After` 在循环中会创建很多定时器对象，热点路径更推荐复用 `time.Timer`（Stop/Reset）。

### 2.3 default：把“阻塞操作”变成“尝试操作”

**概念**：`select` 有 `default` 时，如果所有 case 都不就绪，会立刻执行 default，不会阻塞。  
**示例**：非阻塞接收：`select { case v := <-ch: ...; default: ... }`。  
**为什么这么设计**：有时你需要“尽力而为”的读写（例如尝试发送一个指标、尝试从队列取一个任务），不希望卡住当前 goroutine。default 让你把这种策略表达得非常清晰。

但也要小心：default 容易写出忙等把 CPU 打满。通常需要一个“节奏来源”（timer/ticker/背压）。

### 2.4 多路输入 fan-in：从多个 channel 读，统一输出

**概念**：fan-in 是把多个输入 channel 合并成一个输出 channel。select 是 fan-in 的核心工具。  
**示例**：同时读取 A、B 两个生产者，只要哪边来消息就转发到 out。  
**为什么这么设计**：你可以把系统拆成小组件，每个组件用 channel 输出自己的事件；fan-in 把它们合并，形成清晰的事件流。

关键技巧：输入 channel 关闭后把变量设为 `nil`，在 select 中禁用该分支。

### 2.5 ctx.Done：取消是更高级的超时

**概念**：超时是“到点就停”；取消是“上游不需要了就停”。  
**示例**：worker 在 select 里同时等 ticker 和 `ctx.Done()`，一旦取消立即退出。  
**为什么这么设计**：Go 把取消做成一个通用协议（context），而 select 是把这个协议落实到代码里的工具：你只要把 `<-ctx.Done()` 放进 select，退出条件就变得显式且统一。

### 2.6 nil channel：动态禁用分支（高级但实用）

**概念**：对 nil channel 的收发会永久阻塞；把变量设为 nil 可动态禁用某个 case。  
**示例**：fan-in 里某个输入关闭后，`a = nil`，对应 case 就永远不会再被选中。  
**为什么这么设计**：它让你在不引入额外 if/else 的情况下，动态调整 select 的可选分支，代码更整洁。

注意：所有分支都变 nil 且没有 timeout/default 时会永久阻塞，所以要留兜底（timeout 或 ctx）。

配图建议：
- 选择器示意图：多个 channel + timeout + ctx.Done 汇聚到 select。
- nil channel 禁用分支的示意（A关闭→A=nil→只剩B）。

## 3. 完整代码示例（可运行）

示例程序包含 5 个部分：

1. `select + time.After` 实现超时  
2. `default` 实现非阻塞收/发  
3. fan-in 合并多个 channel（并展示用 nil 禁用关闭分支）  
4. `ctx.Done()` 展示取消传播  
5. nil channel 的行为与“防死锁兜底”  

代码路径：`series/22/cmd/selectlab/main.go`（可直接运行，参数可调）。

## 4. 运行效果 + 截图描述

运行：

```bash
go run ./series/22/cmd/selectlab -timeout=80ms
```

你会看到类似输出（节选，顺序可能略有差异）：

```
--- 1) 超时：select + time.After ---
case work: work finished in 40ms
case timeout: exceeded

--- 2) 非阻塞：default 分支 ---
recv: no data (default)
send: ok
send: would block (default)

--- 5) nil channel：动态禁用 select 分支 ---
b: 7 (a is nil so this case is effectively disabled)
both nil -> timeout branch prevents deadlock
```

截图建议：
- 超时成功/失败对比 + default 非阻塞效果。
- fan-in 合并输出 + nil channel + timeout 防死锁兜底。

## 5. 常见坑 & 解决方案（必写）

1. **没有 timeout/ctx 的 select**：所有 case 都阻塞时会永远卡住。解决：为等待加超时或 `<-ctx.Done()`。
2. **循环里滥用 `time.After`**：每轮创建 timer 对象。解决：热点循环复用 `time.Timer`（Stop/Reset）。
3. **default 写成忙等**：把 CPU 打满。解决：加入阻塞点（channel/timer/ticker）或背压。
4. **读关闭 channel 的零值陷阱**：关闭后会不断返回零值。解决：用 `v, ok := <-ch` 或 `for range ch`。
5. **fan-in 忘记禁用关闭分支**：关闭后仍被 select 命中。解决：关闭后把该 channel 设为 nil。
6. **nil channel 用法不自证**：设为 nil 后忘记恢复。解决：必须配注释/测试覆盖分支切换。

配图建议：
- “无兜底 select 导致卡死”的示意图（所有 case 不就绪）。

## 6. 进阶扩展 / 思考题

- 把 fan-in 扩展成 N 路输入（`[]<-chan T`），思考实现与可读性。
- 用 `time.NewTimer` 重写超时分支，练习 Stop/Reset 的正确姿势。
- 思考题：你项目里哪些等待点需要 timeout？哪些更适合 ctx cancel？

---

select 的价值在于把等待变成显式控制：超时、取消、非阻塞尝试、多路复用都能在一个地方表达清楚。写并发代码时，先把“我在等什么、等多久、等不到怎么办”写出来，死锁和泄漏就会少很多。把示例跑一遍，再回头看你项目里的 goroutine：有没有永远等不到的 channel？有没有缺少 ctx/timeout 的等待？这些通常是最高 ROI 的并发改进点。
