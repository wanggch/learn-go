# defer / panic / recover：异常不是你想的那样

大家好，我是汪小成。你是不是也遇到过这种恐慌：线上偶发 panic，日志只剩一行“nil pointer”；或者某个函数里写了一堆 defer，结果根本没执行，因为有人在中途 `os.Exit`。Go 没有异常机制，但 `defer / panic / recover` 组成了异常控制的三件套。理解它们的执行顺序、作用域和边界，是写出健壮服务的关键。本文用一个“安全执行器”示例把常见坑讲透：defer 顺序、panic 传播、recover 边界、`os.Exit` 的破坏性，以及如何设计更安全的 API。

## 目录

1. 环境准备 / 前提知识
2. 核心概念解释（概念 → 示例 → 设计原因）
3. 完整代码示例（可运行）
4. 运行效果 + 截图描述
5. 常见坑 & 解决方案
6. 进阶扩展 / 思考题

## 1. 环境准备 / 前提知识

- Go 1.22+，命令 `go version` 确认；Git + gofmt/go vet 随 Go 自带。
- 本篇代码目录：`series/11`；运行示例：`go run ./series/11/cmd/safejob`（沙盒限制可用 `GOCACHE=$(pwd)/.cache/go-build`）。
- 基础：了解 interface 与 nil（第 10 篇），会读简单的 panic 栈。

配图建议：目录树突出 `series/11`；一张 defer LIFO 流程图；一张 panic 传播/ recover 截断示意。

## 2. 核心概念解释

### 2.1 defer：延迟调用，LIFO 执行

- 注册顺序：遇到 defer 立刻压栈，函数返回（正常或 panic）时按“后进先出”执行。
- 捕获值 vs 捕获引用：defer 参数在注册时求值；闭包变量按运行时引用。
- 设计原因：用最少的语法成本把“释放资源”贴在获取处，降低遗漏风险。

### 2.2 panic：立即中止当前函数，展开栈

- panic 会终止当前 goroutine，沿调用栈向上执行 defer；若无 recover，程序崩溃并打印栈。
- 非致命逻辑不应滥用 panic；更适合表示“无法恢复”的 bug。
- 设计原因：保留“致命异常”的快速退出通道，同时让资源有机会在 defer 中清理。

### 2.3 recover：只能在 defer 中“接住” panic

- recover 只能在被 panic 触发的 defer 中调用才有效；否则返回 nil。
- 一旦 recover 成功，panic 被吞掉，控制流回到触发 panic 的函数并继续 return。
- 设计原因：限制 recover 作用域，避免随意捕获影响调试。

### 2.4 os.Exit：跳过所有 defer

- 调用 `os.Exit` 会立即退出进程，不执行任何 defer；这也是最危险的地方。
- 设计原因：为“立即退出”保留兜底，但需慎用，最好只在 `main` 的最外层使用。

### 2.5 设计更安全的 API

- 用“安全执行器”封装 panic/recover，将错误转化为 `error` 返回。
- 把清理逻辑放到 defer；将外部资源释放与 recover 放在同一函数，减少遗漏。
- 避免把 recover 放在库里“吞掉” panic；要记录日志并返回错误。

配图建议：一张栈展开示意图（defer 顺序标注），一张 recover 生效的作用域图。

## 3. 完整代码示例（可复制运行）

场景：实现一个 SafeExecute，包装任务执行，保证 defer/cleanup 执行，panic 被 recover 后转为 `error`，并统计耗时。入口：`series/11/cmd/safejob/main.go`。

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Job func() error

type Result struct {
	Name   string
	Status string
	Err    error
	Cost   time.Duration
}

// SafeExecute wraps a job with defer-recover and common cleanup.
func SafeExecute(name string, job Job, cleanup func()) (res Result) {
	res.Name = name
	res.Status = "ok"
	start := time.Now()

	defer func() {
		res.Cost = time.Since(start)
	}()

	defer func() {
		// cleanup always runs, even if panic happens
		if cleanup != nil {
			cleanup()
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[recover] job=%s panic=%v\n", name, r)
			res.Status = "panic"
			res.Err = fmt.Errorf("panic: %v", r)
		}
	}()

	if runErr := job(); runErr != nil {
		res.Status = "error"
		res.Err = runErr
	}

	return res
}

func main() {
	fmt.Println("=== defer / panic / recover 演示 ===")

	results := []Result{
		SafeExecute("normal", func() error {
			defer fmt.Println("  [normal] defer #1")
			defer fmt.Println("  [normal] defer #2 (LIFO)")
			time.Sleep(20 * time.Millisecond)
			return nil
		}, nil),
		SafeExecute("with-error", func() error {
			defer fmt.Println("  [with-error] defer 也会执行")
			return errors.New("业务错误：余额不足")
		}, func() {
			fmt.Println("  [with-error] cleanup: release db connection")
		}),
		SafeExecute("panic", func() error {
			defer fmt.Println("  [panic] defer 2")
			defer fmt.Println("  [panic] defer 1")
			panic("未知 panic：nil pointer")
		}, func() {
			fmt.Println("  [panic] cleanup: close files")
		}),
		SafeExecute("panic-no-recover", func() error {
			defer fmt.Println("  [panic-no-recover] defer only, no recover")
			panic("未恢复的 panic")
		}, func() {
			fmt.Println("  [panic-no-recover] cleanup before exit")
		}),
	}

	for _, r := range results {
		fmt.Printf("\njob=%s status=%s cost=%s\n", r.Name, r.Status, r.Cost)
		if r.Err != nil {
			fmt.Printf("  err: %v\n", r.Err)
		}
	}

	fmt.Println("\n演示 os.Exit：defer 不会执行")
	demoExit()
}

func demoExit() {
	defer fmt.Println("  [exit] 我不会被打印")
	if len(os.Args) > 1000 { // 不触发，避免真实退出
		os.Exit(1)
	}
	fmt.Println("  [exit] 未调用 os.Exit，程序继续")
}
```

代码看点：

- SafeExecute：使用命名返回值，recover 能修改结果，defer 记录耗时并执行 cleanup。
- panic/no-recover：展示如果没有 recover，状态也会被置为 panic（由上层 recover 捕获）。
- demoExit：强调 `os.Exit` 跳过 defer。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/11/cmd/safejob
```

节选输出：

```
=== defer / panic / recover 演示 ===
  [normal] defer #2 (LIFO)
  [normal] defer #1
  [with-error] defer 也会执行
  [with-error] cleanup: release db connection
  [panic] defer 2
  [panic] defer 1
[recover] job=panic panic=未知 panic：nil pointer
  [panic] cleanup: close files
  [panic-no-recover] defer only, no recover
[recover] job=panic-no-recover panic=未恢复的 panic
  [panic-no-recover] cleanup before exit

job=normal status=ok cost=20.594517ms

job=with-error status=error cost=34.95µs
  err: 业务错误：余额不足

job=panic status=panic cost=61.548µs
  err: panic: 未知 panic：nil pointer

job=panic-no-recover status=panic cost=8.354µs
  err: panic: 未恢复的 panic

演示 os.Exit：defer 不会执行
  [exit] 未调用 os.Exit，程序继续
  [exit] 我不会被打印
```

截图建议：

- 终端截图标注 defer 执行顺序和 recover 打印。
- 一张“状态变化”表格：normal/error/panic 不同路径的 status/err。
- 一张 os.Exit 跳过 defer 的对比示意。

## 5. 常见坑 & 解决方案（必看）

1. **defer 参数在注册时求值**：`defer fmt.Println(i)` 捕获的是当时的 i。解决：改用闭包并在内部读变量，或传指针。
2. **panic 后状态丢失**：未用命名返回值，recover 后返回零值。解决：命名返回值或在 recover 中写入外部变量。
3. **在库里吞掉 panic**：recover 后不记录错误。解决：记录日志/指标，并将 panic 转换成 error 往上返回。
4. **跨 goroutine recover 失效**：recover 只对当前 goroutine 生效。解决：goroutine 入口处包一层 `safeGo(func() { ... })`。
5. **os.Exit 跳过 defer**：忘记释放资源。解决：尽量不用 os.Exit；必须用时确保外层没有必须执行的 defer。
6. **多重 defer 顺序误解**：认为按书写顺序执行。解决：牢记 LIFO，必要时在注释里说明执行顺序。
7. **panic 造成数据部分写入**：操作非原子化，panic 期间 defer 清理不完整。解决：使用事务/临时文件/幂等设计。
8. **在 recover 中继续 panic**：处理不当再次 panic 导致难排查。解决：recover 后尽量返回错误，不要再 panic。

配图建议：safeGo 模式代码示意；LIFO 顺序表；panic/error/status 三路径对比。

## 6. 进阶扩展 / 思考题

- 写一个 `safeGo(ctx, fn)` 包装 goroutine，recover 后把错误送到 channel/日志。
- 给 SafeExecute 加上下文超时，感受 defer 与 context 的组合。
- 为 SafeExecute 写表驱动测试，覆盖 normal/error/panic/os.Exit 路径（Exit 用 stub）。
- 在文件写入场景用 defer + 临时文件实现“写完再替换”，避免中途 panic 留下坏数据。
- 将 panic 信息包装成自定义错误类型，练习 `errors.Is` / `errors.As`。
- 探索 defer 性能：在高频路径上是否需要避免过多 defer？写基准测试验证。

配图建议：safeGo 时序图；临时文件写入流程图；defer 性能对比基准表。

---

defer / panic / recover 的关键是：defer 是 LIFO、recover 只能在 defer 中生效、panic 沿栈展开，os.Exit 直接跳过一切。把资源释放贴近获取，用安全执行器统一封装，再配合清晰的错误返回，就能大幅降低“偶发 panic”带来的线上冲击。跑一遍示例，检查你的代码里：哪里需要命名返回值防止状态丢失？哪里该加 safeGo 包装 goroutine？下一篇我们会聊组合优于继承的实践，继续优化代码结构。 
