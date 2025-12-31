# context：并发世界的“取消协议”

你好，我是汪小成。你有没有遇到过这种并发事故：上游请求已经超时返回了，但你的下游 goroutine 还在跑；或者一个批处理任务被取消了，子任务却继续写数据库；又或者你在日志里想打 `request_id`，结果调用链越走越深，信息丢得一干二净。Go 的 `context` 不是“又一个参数”，它是并发世界的协议：**取消（cancel）、超时（deadline）、以及小型元数据（value）** 的统一载体。只要你把 ctx 贯穿调用链并在关键等待点监听 `<-ctx.Done()`，很多 goroutine 泄漏、超时不一致、链路信息丢失的问题都会自然消失。本文会先讲环境与前提，再解释核心概念（概念→示例→为什么这么设计），然后给出完整示例、运行效果、常见坑与解决方案。

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
- 本篇目录：`series/24`。
- 示例入口：`series/24/cmd/ctxlab/main.go`。

### 1.2 运行命令

```bash
go run ./series/24/cmd/ctxlab -timeout=90ms
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/24/cmd/ctxlab -timeout=90ms
```

### 1.3 前置知识

- goroutine + channel 的基本概念（第 19~23 篇已经铺垫）。
- 知道 select 的多路等待语义（第 22 篇）。

配图建议：
- 一张“请求生命周期”图（入口→下游→超时→取消传播）。
- 一张“父 ctx / 子 ctx 关系图”（cancel 传播到子链路）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 context 的三件事：Done / Deadline / Value

**概念**：你可以把 context 理解成一个只读的“控制面”对象，主要提供：

- `Done()`：一个会被 close 的 channel，表示“应该停止了”
- `Deadline()`：可选的截止时间（超时）
- `Value(key)`：小型元数据（如 request_id、trace_id）

**为什么这么设计**：并发程序最难的是生命周期管理。context 把“什么时候该停、为什么停、链路信息是什么”统一起来，让上游能控制下游，让下游能及时退出。

### 2.2 cancel：父取消，子立刻感知

**概念**：你从一个 parent ctx 派生子 ctx（WithCancel/WithTimeout/WithDeadline），一旦 parent cancel，子 ctx 会立即 Done。  
**示例**：parent cancel 后，child 的 `<-child.Done()` 会立刻就绪，child.Err() 变成 `context canceled`。  
**为什么这么设计**：取消必须“向下传播”，否则上游退出了，下游还在跑，就会形成 goroutine 泄漏或无意义工作。

### 2.3 timeout/deadline：超时是一种可观测的退出原因

**概念**：WithTimeout/WithDeadline 会在到点时自动触发 Done，Err() 返回 `context deadline exceeded`。  
**示例**：一个 80ms 的 job 在 50ms timeout 下会超时；同样的逻辑在 120ms timeout 下会完成。  
**为什么这么设计**：把超时变成显式信号，而不是“某个地方 Sleep 了太久”。你可以在日志、指标里区分“被取消”和“超时”。

### 2.4 value：只传小元数据，不传大对象

**概念**：ctx.Value 用来传递跨 API 边界的小型元数据（如 request_id），不是用来传业务参数或大对象。  
**示例**：在入口把 `request_id` 放进 ctx，下游打印时直接取出。  
**为什么这么设计**：ctx 会在调用链上层层传递，如果你塞大对象，会造成隐式依赖、内存占用与可测试性问题。value 的正确姿势是“小而稳定的元信息”。

一个工程建议：key 用自定义类型（例如 `type ctxKey string`），避免与其它包的 key 冲突。

### 2.5 “把 ctx 贯穿调用链”：不要丢

**概念**：ctx 的价值在于传播。如果你在某一层把 ctx 忘了传下去，就相当于断开了取消链路。  
**示例**：handleRequest(ctx) → callDB(ctx) → callRemote(ctx)，每一层都用 select 监听 ctx.Done。  
**为什么这么设计**：取消/超时必须在等待点生效：等待 IO、等待 channel、等待锁、等待 timer。把 `<-ctx.Done()` 放进 select，才能让退出变成“可达路径”。

配图建议：
- 一张“ctx 贯穿调用链”的箭头图（每层都监听 Done）。
- 一张“cancel vs deadline exceeded”对比表。

## 3. 完整代码示例（可运行）

示例包含 4 个部分：

1. cancel 传播：父 ctx cancel，子 ctx 立刻 Done  
2. timeout：DeadlineExceeded 的表现  
3. value：传递 request_id  
4. 调用链：把 ctx 贯穿到 DB/RPC，并在等待点 select ctx.Done  

代码路径：`series/24/cmd/ctxlab/main.go`。

## 4. 运行效果 + 截图描述

运行：

```bash
go run ./series/24/cmd/ctxlab -timeout=90ms
```

典型输出（节选）：

```
--- 1) cancel 传播：父 ctx 取消，子 ctx 立刻感知 ---
child done -> context canceled

--- 2) timeout：DeadlineExceeded 与超时链路 ---
work finished before timeout
ctx2 done -> context deadline exceeded

--- 4) 把 ctx 贯穿调用链：不要丢 ---
callDB ok req= req-9009
handleRequest err=remote: context deadline exceeded
```

截图建议：
- cancel 传播 + timeout 成功/失败对比输出。
- 调用链超时输出 + ctx 传播链路图（标出每层 select 的 Done 分支）。

## 5. 常见坑 & 解决方案（必写）

1. **创建了 ctx 但不传下去**：取消/超时无法到达下游，形成泄漏。解决：函数签名统一第一个参数 `ctx context.Context`，一路传到底。
2. **忘记调用 cancel**：WithCancel/WithTimeout 返回的 cancel 没调用，资源无法及时释放。解决：创建处立刻 `defer cancel()`。
3. **只在最外层检查 ctx**：下游等待点不监听 Done，取消不生效。解决：所有阻塞点用 select 监听 `<-ctx.Done()`。
4. **把 ctx.Value 当参数传递**：塞业务对象、把 ctx 变成“全局变量”。解决：Value 只放小元数据；业务参数显式传参。
5. **key 用 string 容易冲突**：不同包使用相同 key 名字覆盖。解决：用私有自定义类型作为 key。
6. **误把 `context.Background()` 当默认 ctx**：在中间层重建 background 等于切断取消链路。解决：只在入口创建 background；中间层一律接收并传递 ctx。
7. **超时层级不一致**：上游 100ms，下游默认 5s，导致上游超时后下游还跑。解决：下游使用上游 ctx，必要时再 `WithTimeout` 缩短而不是延长。
8. **吞掉 ctx.Err**：超时/取消原因丢失，排查困难。解决：返回/包装 `ctx.Err()`，并在日志里区分 canceled vs deadline exceeded。

配图建议：
- “正确传递 ctx 的签名规范”小抄（`func Foo(ctx context.Context, ...)`）。
- “Value 滥用反例”示意（把大对象塞进 ctx）。

## 6. 进阶扩展 / 思考题

- 把示例扩展成 3 层调用链：API → DB → RPC → cache，观察 timeout 在不同层的表现。
- 写一个 `WithRequestID(ctx, id)` 辅助函数，统一注入/读取 request_id，并写单测验证。
- 把 callRemote 改成并发 fan-out：同时请求两个下游，select 等最先成功或 ctx.Done，练习 ctx 与 select 的组合。
- 思考题：你的服务里有哪些“后台 goroutine”？它们的 ctx 从哪里来？谁负责 cancel？
- 思考题：对一个请求而言，哪些工作应该被取消（可丢弃），哪些工作必须完成（例如写审计日志）？你会如何设计“可取消/不可取消”的边界？

---

context 的价值不在于“多了一个参数”，而在于它把并发世界最难的生命周期管理做成了统一协议：取消能传播、超时可观测、元信息可携带。把 ctx 贯穿调用链，在每个等待点监听 `<-ctx.Done()`，再配合 `defer cancel()` 和规范化的 Value 使用，你就能把大量泄漏与超时类问题从“玄学”变成“工程常识”。
