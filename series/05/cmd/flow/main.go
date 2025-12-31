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
