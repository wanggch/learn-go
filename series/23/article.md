# goroutine 泄漏：它们去哪了？

你好，我是汪小成。你有没有遇到过这种“看不见的故障”：服务跑着跑着内存慢慢涨、延迟越来越抖，但 CPU 不高；过几天一看 goroutine 数从几百涨到几万。你重启服务一切正常，但根因没解决，迟早复发。这个现象通常不是“内存泄漏”，而是 **goroutine 泄漏**：某些 goroutine 永远阻塞在 channel 读写、ticker，或等待一个永远不会到来的信号，导致它们不退出、资源不释放、队列越积越多。本文用可运行的小实验把泄漏模式讲清楚，并给出常用修复手法：退出条件、ctx 取消、close 广播、避免无接收的发送。

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
- 本篇目录：`series/23`。
- 示例入口：`series/23/cmd/leaklab/main.go`。

### 1.2 运行命令

```bash
# 演示：阻塞在接收，channel 永远不 close（泄漏）
go run ./series/23/cmd/leaklab -mode=leak-recv -n=2000 -linger=80ms

# 修复：select + ctx.Done()（可退出）
go run ./series/23/cmd/leaklab -mode=fix-recv -n=2000 -linger=80ms
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/23/cmd/leaklab -mode=fix-recv -n=2000 -linger=80ms
```

### 1.3 前置知识

- goroutine、channel、select 的基本语义。
- 知道 `context.WithCancel/WithTimeout` 用于取消与超时。

配图建议：
- 一张“goroutine 泄漏路径图”（阻塞点：channel/ticker/lock/ctx）。
- 一张“goroutine 数量随时间上涨”示意（类似监控曲线）。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 goroutine 泄漏是什么（它不是内存泄漏的同义词）

**概念**：goroutine 泄漏指的是 goroutine **长期不退出**。它可能持有：

- 堆上的对象（闭包捕获、缓冲区、任务数据）
- channel 的引用（导致生产者/消费者链路无法释放）
- timer/ticker 资源（持续触发或持续等待）

最终表现通常是：goroutine 数持续上涨、GC 更忙、延迟更抖。  
**为什么这么设计**：运行时不会“强杀” goroutine，退出条件必须由你的逻辑提供。

### 2.2 泄漏的本质：永远等不到的阻塞点

回到 channel 的语义：收发在条件不满足时会阻塞。泄漏通常就是：

- 读：一直等不到发送（发送方没发/没 close）
- 写：一直等不到接收（接收方退出/没启动）
- select：所有分支都卡住（没有 timeout/ctx/default 兜底）

这也是为什么并发代码里最常问的一句话是：**“这个 goroutine 的退出条件是什么？”**

### 2.3 三类最常见的泄漏模式

1. **blocked on receive**：消费者 `for v := range ch`，但 channel 永远不 close（或者生产者早退出）。  
2. **blocked on send**：生产者往无缓冲 channel 发送，但没有接收者（或接收者提前退出）。  
3. **ticker/loop never exits**：`for range ticker.C { ... }` 没有退出条件或没有 Stop，长期运行后越来越多。

你会发现这三类都可以归到一句话：**协作方消失了，但等待还在继续。**

### 2.4 修复的通用策略：退出条件 + 取消协议 + 资源释放

工程上最稳定的修复组合是：循环有退出条件（ctx/done/close）+ select 监听 `<-ctx.Done()` + 资源及时 Stop/Close（defer 贴近创建点）。

配图建议：
- 一张“泄漏 → 修复”的对照图（leak-recv vs fix-recv）。
- 一张“ctx.Done 与 close 广播”的比较图（单点取消 vs 广播结束）。

## 3. 完整代码示例（可运行）

示例程序提供多个模式（通过 `-mode` 切换）：

- `leak-recv` / `fix-recv`：接收阻塞 vs ctx 取消退出。  
- `leak-send` / `fix-send`：发送阻塞 vs 超时退出。  
- `leak-ticker` / `fix-ticker`：ticker 不退出 vs ctx+Stop+wait。  

代码路径：`series/23/cmd/leaklab/main.go`（可直接运行，通过 `-mode` 切换）。

## 4. 运行效果 + 截图描述

### 4.1 leak-recv：goroutine 数量飙升且不回落

```bash
go run ./series/23/cmd/leaklab -mode=leak-recv -n=2000 -linger=80ms
```

典型输出（节选）：

```
[start] goroutines=1 ...
[after spawn (still blocked)] goroutines=2001 heap_alloc=1.30MB ...
[end] goroutines=2001 ...
```

你会直观看到：goroutine 数一直保持在 2001——它们都在等一个永远不会发生的接收完成。

### 4.2 fix-recv：cancel 后回落到 1

```bash
go run ./series/23/cmd/leaklab -mode=fix-recv -n=2000 -linger=80ms
```

典型输出（节选）：

```
[after spawn] goroutines=2001 ...
[after cancel + wait] goroutines=1 ...
```

注意：goroutine 数会回落，但 `heap_alloc` 不一定立刻下降，这通常是回收时机问题。

### 4.3 leak-ticker vs fix-ticker

```bash
go run ./series/23/cmd/leaklab -mode=leak-ticker -linger=60ms
go run ./series/23/cmd/leaklab -mode=fix-ticker -linger=60ms
```

你会看到 leak 模式结束时 goroutine=2，而 fix 模式 cancel 后会回到 1。

截图建议：
- leak-recv vs fix-recv：goroutines 从 2001 回落到 1 的对比。
- leak-ticker vs fix-ticker：goroutine=2 vs goroutine=1。
- “退出条件检查点”流程图（循环里 select：data / ctx.Done）。

## 5. 常见坑 & 解决方案（必写）

1. **for range ch 永远不退出**：生产者忘记 close，消费者永久阻塞。解决：明确关闭时机；多生产者时集中关闭（协调者/WaitGroup 归零后关闭）。
2. **发送端没接收者**：无缓冲 channel 发送永久卡住。解决：确保接收者存在；或发送端带 ctx/timeout；或改成有缓冲并配背压策略。
3. **select 没有兜底**：所有 case 都不就绪就永久卡住。解决：加入 `<-ctx.Done()` 或超时分支，保证生命周期可控。
4. **ticker 没 Stop**：在循环/函数中创建 ticker 后不释放，或 goroutine 不退出。解决：`defer ticker.Stop()` + ctx 退出条件。
5. **把泄漏当成“内存不下降”**：goroutine 已退出，但 heap_alloc 没回落。解决：看 goroutine 数趋势；必要时再做进一步分析。
6. **关闭 channel 的职责混乱**：多个发送者都可能 close，导致 panic。解决：约定“只有发送方/协调者关闭”，其他只发送或只接收。
7. **忘记 cancel**：创建了 `WithCancel/WithTimeout` 却没调用 cancel，导致下游 goroutine 一直活着。解决：创建处立刻 `defer cancel()`，并把 ctx 传下去。
8. **泄漏隐藏在重试/循环里**：每次重试都启动 goroutine，失败路径没回收。解决：把 goroutine 生命周期绑定到 ctx，并确保所有路径都能退出。

配图建议：
- “多生产者如何关闭 channel”的决策图（集中 close）。
- “泄漏 vs 未立即回收”对照图（goroutine 数下降 vs heap_alloc 变化）。

## 6. 进阶扩展 / 思考题

- 给 leak-recv 增加一个真正的 consumer 逻辑：接收后处理并继续循环，思考如何在 ctx cancel 时退出循环。
- 把 fix-send 改成“带缓冲队列 + 有界并发”的模式，理解“背压/限流/取消”的组合。
- 把 ticker demo 改成“对齐到整秒 tick”，并加入 ctx cancel，练习 timer/ticker 的正确关闭姿势。
- 思考题：你项目里哪些 goroutine 是“常驻”的？它们的退出条件是什么？监控上你会用什么指标来证明它们没有泄漏？
- 思考题：当你发现 goroutine 数持续上涨，你会如何定位？（提示：先看阻塞点，最后看谁没 close/cancel）

---

goroutine 泄漏的本质是：代码里存在一个“永远等不到的阻塞点”。修复它也很工程化：给循环一个退出条件，把 `<-ctx.Done()` 放进 select，把 ticker/timer 当资源及时 Stop，把 close 的职责集中到发送方或协调者。把本文示例跑一遍，再回到你的业务代码：每个 goroutine 都问一句“它怎么退出？”通常就能定位根因。
