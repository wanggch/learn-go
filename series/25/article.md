# 并发模式总结：worker pool、fan-in/out

你好，我是汪小成。你可能有过这种体验：学会 goroutine、WaitGroup、channel、select 之后，写业务并发仍然“像拼积木一样乱”。一会儿无界并发把系统打爆，一会儿 goroutine 泄漏越跑越多，一会儿又因为取消/超时没传下去导致链路不一致。原因往往不是语法不会，而是缺少一套可复用的并发结构。本篇把最常用的并发模式做一次“收口总结”：**worker pool、fan-out、fan-in，以及它们和 context 的组合**。你会得到一套可以直接抄进项目里的骨架：可控并发、可取消、可收敛结果、可观测。

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
- 本篇目录：`series/25`。
- 示例入口：`series/25/cmd/patterns/main.go`。

### 1.2 运行命令

```bash
go run ./series/25/cmd/patterns -workers=6 -items=80 -timeout=220ms
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/25/cmd/patterns -workers=6 -items=80 -timeout=220ms
```

### 1.3 前置知识

- goroutine、channel、select、context 的基本概念。

配图建议：
- 一张“fan-out/fan-in”模式图（输入任务 → 多 worker → 合并结果）。
- 一张“worker pool 生命周期”图（jobs close → workers exit → results close）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 worker pool：用固定 worker 数处理无限任务

**概念**：worker pool 是“固定数量的 worker goroutine + 一个任务队列”。它的关键价值是：**并发度有上限**。  
**示例**：`jobs` channel 投递任务，`workers` 个 goroutine `range jobs` 拉取并处理，把结果写到 `results`。  
**为什么这么设计**：

- 避免无界并发（流量突发不会把系统打死）。
- 更符合真实世界资源上限（CPU、连接池、下游 QPS）。
- 更容易观测（worker 数、队列长度、处理耗时都能统计）。


### 2.2 fan-out：把任务分发到多个 worker

**概念**：fan-out 就是把一条任务流分发到多个并发执行单元。  
**示例**：把 `job{id, data}` 持续写入 `jobs` channel，多个 worker 竞争读取，这天然就是分发。  
**为什么这么设计**：在 Go 里，channel 的竞争接收非常自然，调度器会在多个 goroutine 之间分配工作，代码比“自己写队列 + 自己抢锁”更直观。

### 2.3 fan-in：把多个结果合并成一个输出

**概念**：fan-in 是把多 worker 的结果合并到一个结果流里。  
**示例**：所有 worker 都往同一个 `results` channel 写；主 goroutine `range results` 统一消费。  
**为什么这么设计**：把“汇总与输出”集中到一个位置，便于统计成功/失败、平均耗时、错误聚合、以及统一取消策略。

注意：fan-in 的关闭时机最关键。一般做法是：

- jobs 关闭后，worker 自然退出；
- 用 WaitGroup 等所有 worker 退出；
- 然后再 close(results)，让消费者结束 range。

### 2.4 与 context 组合：让模式具备“可取消性”

**概念**：并发模式要工程化，就必须可取消：上游超时或主动取消时，下游应该停止继续做无意义工作。  
**示例**：worker 的 select 同时监听 `<-ctx.Done()` 与 `<-jobs`；发送结果时也要监听 ctx，避免阻塞导致泄漏。  
**为什么这么设计**：一旦取消发生，整个模式应该收敛：生产者停止投递、worker 停止处理、结果通道关闭、主协程退出。

### 2.5 三条“模式正确性”检查清单

你写完一个 worker pool + fan-in/out，可以用三件事自检：并发度是否有上限、退出条件是否完整、阻塞点是否可控。能回答清楚这三点，基本就不会写出“跑着跑着挂住”的并发代码。

配图建议：
- 一张“关闭顺序”时序图：producer close(jobs) → wg.Wait → close(results) → consumer range 结束。
- 一张“取消传播”图：ctx cancel → producer stop → workers stop → results close。

## 3. 完整代码示例（可运行）

示例目标：用 worker pool 并发处理一批任务（模拟 CPU 工作 + 小等待），并用 fan-in 收敛结果；同时用 context 施加总超时，展示“可取消的并发模式”。  

代码路径：`series/25/cmd/patterns/main.go`。

关键点你可以在代码里重点看这几处：

- producer 往 jobs 投递任务时 `select { case <-ctx.Done(): ...; case jobs <- ... }`，避免超时后仍然阻塞投递。
- worker 读取 jobs 时也监听 ctx；写 results 时同样监听 ctx，避免“结果没人收导致阻塞”。
- results 的关闭由 WaitGroup 驱动，保证不会出现“发送到已关闭 channel”。

## 4. 运行效果 + 截图描述

运行：

```bash
go run ./series/25/cmd/patterns -workers=6 -items=80 -timeout=220ms
```

典型输出（节选）：

```
GOMAXPROCS=12 workers=6 items=80 timeout=220ms
result id=0 worker=2 hash=4437e408 cost=6ms...
result id=20 worker=6 hash=e704257d cost=6ms...
...
done: results=80 avg_cost=6ms... ctx_err=<nil>
```

截图建议：
- worker 分担任务输出 + fan-out/fan-in 流程图（标注 jobs/results 与 close 顺序）。
- 调整 `-timeout=40ms` 后的输出（部分任务完成、ctx_err=deadline exceeded），突出“可取消”。

## 5. 常见坑 & 解决方案（必写）

1. **无界并发假装是 worker pool**：看似开了 workers，但生产者仍然为每个任务开 goroutine。解决：任务必须通过 jobs 队列进入固定 worker。
2. **results 永远不 close**：消费者 `range results` 永远等不到结束。解决：用 WaitGroup 等 worker 退出后关闭 results。
3. **过早 close(results)**：仍有 worker 在发送，直接 panic。解决：只让一个协调者 close，并且 close 发生在 wg.Wait 之后。
4. **取消时仍在阻塞发送**：ctx cancel 了，但 worker 卡在 `results <- r`。解决：发送 results 也要 select ctx.Done；或给 results 合理 buffer；或让消费者保证 drain。
5. **任务队列 buffer 过大**：把背压藏进队列，延迟不可控。解决：buffer 只覆盖小波动，真正的限流要靠并发度与上游节流。
6. **错误处理策略混乱**：遇到一个错误是否要取消全局？还是收集所有错误？解决：明确策略：fail-fast vs best-effort，并体现在 ctx cancel 或错误聚合里。
7. **把 CPU 型任务并发度开太大**：超过有效 CPU 后收益下降。解决：并发度接近 `GOMAXPROCS`，用基准测试验证。
8. **遗漏退出条件导致泄漏**：producer/worker 任意一方不退出，模式无法收敛。解决：三处都要考虑退出：投递、处理、发送结果。

配图建议：
- “关闭顺序错误 → panic”的示意（close(results) 太早）。
- “fail-fast vs best-effort”决策表。

## 6. 进阶扩展 / 思考题

- 把结果 channel 改成带 buffer，观察在 slow consumer 下的行为差异；思考什么时候需要 buffer，什么时候需要背压。
- 加入错误阈值：如果错误超过 N 个，就 cancel ctx，提前收敛。
- 思考题：你的业务任务是 CPU 型还是 IO 型？最佳 worker 数怎么估？你会用哪些指标验证（吞吐、p95、错误率、队列长度）？
- 思考题：当 ctx timeout 发生时，你希望“尽量 drain 已完成结果”还是“立即返回”？两者对资源与用户体验的影响是什么？

---

worker pool 解决“并发度可控”，fan-out 解决“任务分发”，fan-in 解决“结果收敛”，context 解决“可取消与可收敛”。把这四件事组合起来，你就拥有一套稳定的并发骨架：不无界、不泄漏、可观测、可维护。建议把它当作默认模板，遇到并发需求先套模板，再根据业务小步调整。
