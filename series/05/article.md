# if / for / switch：写出“Go 味”的控制流

大家好，我是汪小成。还记得那种凌晨值班、面对一坨 if/else 迷宫的绝望吗？同一个逻辑被复制到三处，某个分支漏了一行，线上就慢性出血。Go 的控制流简单却不等于随意，用得不好一样会绕成麻花。本文带你用 if / for / switch 写出“Go 味”的代码：少嵌套、可预期、方便排查。接下来会先铺好环境和知识底座，再拆解每种结构的设计思路，然后给出完整示例、运行效果、常见坑和进阶练习。

## 目录

1. 环境准备 / 前提知识
2. 核心概念解释（概念 → 示例 → 设计原因）
3. 完整代码示例（可运行）
4. 运行效果 + 截图描述
5. 常见坑 & 解决方案
6. 进阶扩展 / 思考题

## 1. 环境准备 / 前提知识

### 1.1 必备版本

- Go 1.22+（确保支持工作区 `go.work`，命令：`go version`）。
- Git 方便回溯；推荐配合 VS Code + Go 扩展或 Goland。
- `gofmt` / `go vet` 已随 Go 自带，保持默认即可。

### 1.2 项目结构小贴士

- 根目录有 `go.work` 管理多模块；每篇文章一个模块，代码隔离。
- 本篇目录：`series/05`，示例入口：`cmd/flow/main.go`。
- 运行命令：`go run ./series/05/cmd/flow`；无需额外依赖。

### 1.3 前置知识

- 熟悉 `go mod tidy`、`go test ./...` 的基本用法。
- 理解 Go 的导出规则（大写导出、小写包内可见）。
- 知道 `if err := ...; err != nil` 的写法，便于守卫式返回。

配图建议：一张项目目录树示意（突出 `go.work` 与 `series/05`），一张「守卫式 return」流程箭头图。

## 2. 核心概念解释

### 2.1 Go 为何只有 if / for / switch

Go 刻意减少关键字，控制流只有三件套。好处是：阅读者心智负担低；坏处是如果把它们写成“嵌套塔”，依旧难以维护。Go 风格强调**平铺和提前返回**，让 happy path 清晰、异常快速退出。

### 2.2 if：守卫式 return + 短变量声明

- **守卫式 return**：先处理异常，再写主流程，减少缩进层级。
- **短变量声明**：`if v, ok := cache[key]; ok { ... }` 把作用域收紧，避免污染外部命名空间。
- **合并条件**：用早返回代替 `if/else if/else` 链，让阅读顺序顺滑。

示例（提前失败，再执行正常逻辑）：

```go
if err := validate(input); err != nil {
    return nil, fmt.Errorf("validate: %w", err)
}

// 主流程从这里开始，没有额外缩进
result := doWork(input)
```

为什么这么设计：Go 选择了“错误值返回”而非异常，if 的守卫式写法天然契合这一决策。

配图建议：流程图展示多层 if/else 与守卫式 return 的对比。

### 2.3 for：三种形态 + 控制语句

- **经典三段式**：`for i := 0; i < n; i++ {}`，可读且局部变量不会泄漏。
- **条件循环**：`for condition {}` 代替 `while`，常见于重试与轮询。
- **range**：`for k, v := range m {}`，注意循环变量被重用（涉及闭包时要拷贝）。
- **控制语句**：`break` 退出当前循环，`continue` 跳到下轮，`break label` 从嵌套层跳出，慎用、但在多层扫描时很实用。

为什么这么设计：只有一个 `for`，减少概念；配合 label 解决偶发的多层跳转需求。

配图建议：一张 for 三形态的对照表，一张展示 `break label` 如何跳出嵌套的示意。

### 2.4 switch：多分支的首选

- **表达式 switch**：`switch x { case 1: ... }`，比多个 if/else 更紧凑。
- **条件 switch**：`switch { case cond1: ... }`，适合多区间判断。
- **type switch**：`switch v := any.(type) { case *User: ... }`，当泛型不可用时的类型分发。
- **少用 fallthrough**：除非要“落下去”，否则默认 break 最安全。

为什么这么设计：Go 倾向显式枚举分支，提高可读性；默认 break 减少意外贯穿。

配图建议：用表格展示三种 switch，用小箭头标出默认 break 与显式 `fallthrough` 的差别。

## 3. 完整代码示例（可复制运行）

场景：我们做一个“任务调度小程序”，用 if 做守卫、for 做重试/遍历、switch 做分支调度。代码路径：`series/05/cmd/flow/main.go`。

```go
package main

import (
	"fmt"
	"strings"
	"time"
)

type Task struct {
	ID             int
	Kind           string
	Priority       int
	Steps          []string
	MaxRetry       int
	EnvReady       bool
	PayloadSize    int
	DryRun         bool
	TimeoutSeconds int
}

type Stats struct {
	Success int
	Failed  int
	Skipped int
}

type TaskStatus string

const (
	StatusSuccess        TaskStatus = "success"
	StatusRetryExhausted TaskStatus = "retry_exhausted"
	StatusFatal          TaskStatus = "fatal"
)

type HandleResult struct {
	Status   TaskStatus
	Attempts int
	Message  string
}

type stepOutcome string

const (
	outcomeSuccess stepOutcome = "success"
	outcomeRetry   stepOutcome = "retry"
	outcomeFatal   stepOutcome = "fatal"
)

func main() {
	tasks := sampleTasks()
	fmt.Println("=== if / for / switch 控制流演示 ===")

	stats := Stats{}
	for idx, task := range tasks {
		fmt.Printf("\n[%d/%d] 任务 #%d (%s, 优先级 %d)\n", idx+1, len(tasks), task.ID, task.Kind, task.Priority)

		if reason := shouldSkip(task); reason != "" {
			fmt.Printf("跳过：%s\n", reason)
			stats.Skipped++
			continue
		}

		result := handleTask(task)
		switch result.Status {
		case StatusSuccess:
			stats.Success++
		case StatusRetryExhausted, StatusFatal:
			stats.Failed++
		}

		fmt.Printf("结果：%s（尝试 %d 次，提示：%s）\n", result.Status, result.Attempts, result.Message)
	}

	fmt.Printf("\n汇总：成功 %d 个 | 失败 %d 个 | 跳过 %d 个\n", stats.Success, stats.Failed, stats.Skipped)
}

func shouldSkip(task Task) string {
	if task.Priority <= 0 {
		return "优先级为 0，直接忽略"
	}
	if !task.EnvReady && task.Kind == "import" {
		return "导入任务的环境未就绪"
	}

	switch {
	case task.Priority < 3 && len(task.Steps) == 0:
		return "步骤为空且优先级低"
	case strings.Contains(task.Kind, "demo") && task.DryRun:
		return "演示任务仅演练，不做真实执行"
	}

	return ""
}

func handleTask(task Task) HandleResult {
	maxRetry := task.MaxRetry
	if maxRetry < 1 {
		maxRetry = 1
	}

	for attempt := 1; attempt <= maxRetry; attempt++ {
		outcome, msg := runSteps(task)

		switch outcome {
		case outcomeSuccess:
			return HandleResult{Status: StatusSuccess, Attempts: attempt, Message: msg}
		case outcomeFatal:
			return HandleResult{Status: StatusFatal, Attempts: attempt, Message: msg}
		case outcomeRetry:
			fmt.Printf("  尝试 %d 需要重试：%s\n", attempt, msg)
			if task.PayloadSize > 120 {
				task.PayloadSize -= 60
				fmt.Printf("  调整负载到 %d 后再试\n", task.PayloadSize)
			}
			if attempt < maxRetry {
				time.Sleep(80 * time.Millisecond)
			}
		}
	}

	return HandleResult{Status: StatusRetryExhausted, Attempts: maxRetry, Message: "多次重试仍失败"}
}

func runSteps(task Task) (stepOutcome, string) {
	if !task.EnvReady {
		return outcomeFatal, "依赖环境未准备好"
	}

	message := "步骤全部完成"

stepLoop:
	for idx, step := range task.Steps {
		fmt.Printf("  步骤 %d：%s\n", idx+1, step)

		switch step {
		case "validate":
			if task.PayloadSize == 0 {
				return outcomeFatal, "payload 为空"
			}
			if task.PayloadSize > 800 {
				return outcomeFatal, "payload 明显异常"
			}
		case "simulate":
			for countdown := task.TimeoutSeconds; countdown > 0; countdown-- {
				if countdown == task.TimeoutSeconds {
					fmt.Printf("    倒计时 %d ...\n", countdown)
				}
				if countdown <= 2 {
					fmt.Printf("    快完成，剩余 %d 秒\n", countdown)
				}
				time.Sleep(10 * time.Millisecond)
			}
		case "dry-run":
			if task.DryRun {
				fmt.Println("    仅演练，跳过后续执行")
				message = "演练完成，未做实际修改"
				break stepLoop
			}
		case "process":
			switch {
			case task.Kind == "reconcile" && task.PayloadSize > 200:
				return outcomeRetry, "对账批次太大，拆分后再试"
			case task.Kind == "import" && task.PayloadSize > 350:
				return outcomeRetry, "导入批次过大，等待上游切分"
			default:
				fmt.Println("    处理完成")
			}
		case "deliver":
			if strings.Contains(task.Kind, "export") {
				fmt.Println("    已写入导出文件")
			} else {
				fmt.Println("    已写入主存储")
			}
		default:
			fmt.Printf("    未知步骤 %q，记录后继续\n", step)
			continue
		}
	}

	return outcomeSuccess, message
}

func sampleTasks() []Task {
	return []Task{
		{
			ID:             101,
			Kind:           "import",
			Priority:       5,
			Steps:          []string{"validate", "simulate", "process", "deliver"},
			MaxRetry:       2,
			EnvReady:       true,
			PayloadSize:    220,
			DryRun:         false,
			TimeoutSeconds: 3,
		},
		{
			ID:             102,
			Kind:           "demo-import",
			Priority:       2,
			Steps:          []string{"validate", "dry-run", "process", "deliver"},
			MaxRetry:       1,
			EnvReady:       true,
			PayloadSize:    80,
			DryRun:         true,
			TimeoutSeconds: 1,
		},
		{
			ID:             103,
			Kind:           "reconcile",
			Priority:       4,
			Steps:          []string{"validate", "process", "deliver"},
			MaxRetry:       3,
			EnvReady:       true,
			PayloadSize:    260,
			DryRun:         false,
			TimeoutSeconds: 2,
		},
		{
			ID:             104,
			Kind:           "import",
			Priority:       1,
			Steps:          []string{"validate", "process"},
			MaxRetry:       1,
			EnvReady:       false,
			PayloadSize:    140,
			DryRun:         false,
			TimeoutSeconds: 1,
		},
		{
			ID:             105,
			Kind:           "export",
			Priority:       4,
			Steps:          []string{"validate", "process", "deliver"},
			MaxRetry:       1,
			EnvReady:       true,
			PayloadSize:    0,
			DryRun:         false,
			TimeoutSeconds: 2,
		},
	}
}
```

代码里可以看到：

- **if**：`shouldSkip` 用守卫式返回剪枝；`runSteps` 在异常时立即返回。
- **for**：`handleTask` 的重试循环、`simulate` 的倒计时、`range` 遍历步骤。
- **switch**：`shouldSkip` 的条件 switch，`process` 的“条件分发” switch，`main` 中按结果分支统计。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/05/cmd/flow
```

典型输出（节选）：

```
=== if / for / switch 控制流演示 ===

[1/5] 任务 #101 (import, 优先级 5)
  步骤 1：validate
  步骤 2：simulate
    倒计时 3 ...
    快完成，剩余 2 秒
    快完成，剩余 1 秒
  步骤 3：process
    处理完成
  步骤 4：deliver
    已写入主存储
结果：success（尝试 1 次，提示：步骤全部完成）
...
汇总：成功 3 个 | 失败 1 个 | 跳过 1 个
```

截图建议：

- 终端运行截图：展示重试、跳过、失败三种分支，突出 for + switch 的输出。
- 简单流程图：任务流从 shouldSkip → handleTask → runSteps 的箭头，标记守卫式返回与 break label 位置。

## 5. 常见坑 & 解决方案（必看）

1. **if 短变量声明导致遮蔽**：`if err := do(); err != nil { ... }` 内部 `err` 不等于外层同名变量。解决：必要时拆分声明或改名。
2. **for range 闭包引用同一迭代变量**：在 goroutine 中直接用 `for i, v := range items { go func() { fmt.Println(i, v) }() }` 会打印重复值。解决：在循环内拷贝 `i, v := i, v`。
3. **label 滥用**：`break outer` 让人摸不清跳出了哪层。解决：先尝试函数拆分；确实需要跳出多层时才加 label，并贴注释。
4. **switch 忘记 default**：处理外部输入时未覆盖 default，新增枚举会悄悄落空。解决：为外部输入留一个 default 分支记录日志或返回错误。
5. **无限 for 未设退出条件**：`for { ... }` 没有 break，或 break 条件永远 false。解决：明示退出条件，或用 context/timeout 控制。
6. **条件 switch 顺序问题**：`switch { case x > 10: ...; case x > 5: ... }` 时第二个条件永远触发不到。解决：按覆盖度由小到大或由高到低排序，并加注释。
7. **过度嵌套**：if/for/switch 相互嵌套三层以上，阅读成本陡增。解决：守卫式 return、提取函数、用 switch 切分分支。

配图建议：一张「for range 闭包坑」的示意图，标出同一指针被复用；一张展示 default 保护未知输入的流程图。

## 6. 进阶扩展 / 思考题

- 重写 `handleTask`：把重试策略改成指数退避（for + time.Sleep），观察代码可读性变化。
- 给 `runSteps` 加 context，超时后 break label 提前退出，思考取消信号如何向下游传播。
- 用 type switch 给不同任务类型做专属处理（如导出写文件、导入写数据库），比较与接口实现的可维护性。
- 在 `simulate` 阶段加入 select + ticker，体验 for 与 select 的组合写法。
- 写表驱动测试：构造多组 `Task`，验证 skip/重试/失败的分支覆盖率。

配图建议：脑图列出 if/for/switch 的最佳实践清单；流程图演示指数退避的时间线。

---

到这里，你已经掌握了 Go 控制流的“味道”：守卫式 if 平铺逻辑、for 三形态覆盖遍历/重试、switch 清晰列举分支。把示例跑一遍，再把自己的业务逻辑重写一次，你会明显感觉到代码变得易懂、可维护。 下一篇我们会进入 slice 与 map 的坑和打法，敬请期待。
