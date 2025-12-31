# 从“会写 Go”到“写好 Go”

你好，我是汪小成。很多人学完语法、写完几个小 demo 后，会遇到一个真实痛点：代码能跑，但不敢改；功能能交付，但出了问题很难查。真正的“写好 Go”，不是再会一个语法点，而是把 **结构、错误、并发、配置、测试** 这些工程能力串成一套稳定的实践。本文会先给出环境准备，再梳理“写好 Go”的关键原则，最后用一个完整示例串起来，并给出运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/40`。
- 示例入口：`series/40/cmd/capstone/main.go`。

### 1.2 运行命令

默认运行（含失败示例）：

```bash
go run ./series/40/cmd/capstone -timeout=900ms
```

想体验超时取消：

```bash
go run ./series/40/cmd/capstone -timeout=400ms
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/40/cmd/capstone -timeout=900ms
```

### 1.3 前置知识

- 了解 goroutine、channel 与 WaitGroup 的基础用法。
- 了解 `errors.Is` 的基本概念。

提示：示例是一个“小型任务执行器”，把你在系列中学到的核心能力串起来。

小建议：把每次练习的“经验总结”写成 checklist，长期积累下来就是你的工程规范，也更体系化、更有章法。

配图建议：
- 一张“工程能力拼图”示意图。
- 一张“任务执行器流程图”。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 结构优先：入口薄、逻辑清

**概念**：入口只负责参数与编排，业务逻辑尽量函数化。  
**示例**：`main` 里只做解析、组装与调用 `run`。  
**为什么这么设计**：可读性强、可测试性好。

### 2.2 错误可分类，才能做决策

**概念**：错误不是字符串，而是可判断的类型或标签。  
**示例**：`errors.Is(err, context.DeadlineExceeded)`。  
**为什么这么设计**：不同错误走不同处理策略。

### 2.3 并发要有“预算”和“收敛”

**概念**：任何并发都要有超时与退出条件。  
**示例**：`context.WithTimeout` 控制任务总预算。  
**为什么这么设计**：避免 goroutine 泄漏和资源占用。

### 2.4 日志结构化，排障更快

**概念**：日志统一 `key=value`。  
**示例**：`level=INFO service=capstone job=9 cost=...`。  
**为什么这么设计**：方便检索与聚合分析。

### 2.5 优先保证正确，再谈优化

**概念**：工程代码优先清晰、稳定，再逐步优化。  
**示例**：先让 worker 池跑通，再考虑性能细节。  
**为什么这么设计**：可维护性比“瞬时性能”更重要。

### 2.6 结果要“可验证”

**概念**：运行结果应有摘要与统计。  
**示例**：输出 `handled/failed/canceled`。  
**为什么这么设计**：让运维与开发一眼判断运行状态。

### 2.7 约束比灵活更重要

**概念**：先给工程建立边界，再谈灵活扩展。  
**示例**：统一入口参数、统一日志格式、统一错误分类。  
**为什么这么设计**：边界清晰时，团队协作成本最低。

### 2.8 “少而清晰”的抽象更可维护

**概念**：抽象的目标是降低复杂度，而不是制造层次。  
**示例**：一个清晰的 `run` 函数胜过一堆薄包装。  
**为什么这么设计**：可维护性来自“结构清晰”，而不是“结构复杂”。

配图建议：
- 一张“正确性优先”流程图。
- 一张“日志结构化字段示意图”。

## 3. 完整代码示例（可运行）

示例包含：

1. 可配置的任务执行器（jobs/workers/timeout）。
2. 上下文超时，统一控制执行预算。
3. 结构化日志 + 运行摘要。

代码路径：`series/40/cmd/capstone/main.go`。

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var errJobFailed = errors.New("job failed")

type Config struct {
	Jobs    int
	Workers int
	Timeout time.Duration
}

type Summary struct {
	Jobs     int
	Handled  int
	Failed   int
	Canceled int
	Elapsed  time.Duration
}

type result struct {
	id   int
	err  error
	cost time.Duration
}

type Logger struct {
	service string
	logger  *log.Logger
}

type Field struct {
	Key   string
	Value string
}

func main() {
	cfg := parseFlags()
	if err := validateConfig(cfg); err != nil {
		log.Fatal(err)
	}

	logger := NewLogger("capstone")
	runID := traceID()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	logger.Info("run start",
		Str("run", runID),
		Int("jobs", cfg.Jobs),
		Int("workers", cfg.Workers),
		Duration("timeout", cfg.Timeout),
	)

	summary := run(ctx, cfg, logger, runID)

	logger.Info("run summary",
		Str("run", runID),
		Int("handled", summary.Handled),
		Int("failed", summary.Failed),
		Int("canceled", summary.Canceled),
		Duration("cost", summary.Elapsed),
	)
}

func parseFlags() Config {
	cfg := Config{
		Jobs:    9,
		Workers: 3,
		Timeout: 500 * time.Millisecond,
	}

	flag.IntVar(&cfg.Jobs, "jobs", cfg.Jobs, "number of jobs")
	flag.IntVar(&cfg.Workers, "workers", cfg.Workers, "number of workers")
	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "run timeout")
	flag.Parse()

	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.Jobs <= 0 {
		return errors.New("jobs must be positive")
	}
	if cfg.Workers <= 0 {
		return errors.New("workers must be positive")
	}
	if cfg.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	return nil
}

func run(ctx context.Context, cfg Config, logger *Logger, runID string) Summary {
	start := time.Now()
	jobs := make(chan int)
	results := make(chan result)

	var wg sync.WaitGroup
	for i := 1; i <= cfg.Workers; i++ {
		wg.Add(1)
		go worker(ctx, i, jobs, results, logger, runID, &wg)
	}

	go func() {
		defer close(jobs)
		for i := 1; i <= cfg.Jobs; i++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	summary := Summary{Jobs: cfg.Jobs}
	for res := range results {
		summary.Handled++
		if res.err != nil {
			if errors.Is(res.err, context.DeadlineExceeded) || errors.Is(res.err, context.Canceled) {
				summary.Canceled++
			} else {
				summary.Failed++
			}
		}
	}
	summary.Elapsed = time.Since(start)
	return summary
}

func worker(ctx context.Context, id int, jobs <-chan int, results chan<- result, logger *Logger, runID string, wg *sync.WaitGroup) {
	defer wg.Done()
	for jobID := range jobs {
		jobStart := time.Now()
		err := processJob(ctx, jobID)
		cost := time.Since(jobStart)

		fields := []Field{
			Str("run", runID),
			Int("worker", id),
			Int("job", jobID),
			Duration("cost", cost),
		}

		switch {
		case err == nil:
			logger.Info("job done", fields...)
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			logger.Info("job canceled", fields...)
		default:
			logger.Error("job failed", append(fields, Err(err))...)
		}

		results <- result{id: jobID, err: err, cost: cost}
	}
}

func processJob(ctx context.Context, id int) error {
	delay := time.Duration(80+(id%5)*40) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	if id%9 == 0 {
		return fmt.Errorf("job %d: %w", id, errJobFailed)
	}
	return nil
}

func NewLogger(service string) *Logger {
	return &Logger{
		service: service,
		logger:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.emit("INFO", msg, fields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.emit("ERROR", msg, fields...)
}

func (l *Logger) emit(level, msg string, fields ...Field) {
	parts := []string{
		"level=" + level,
		"service=" + l.service,
		"msg=" + msg,
	}
	for _, f := range fields {
		parts = append(parts, f.String())
	}
	l.logger.Println(strings.Join(parts, " "))
}

func (f Field) String() string {
	return f.Key + "=" + f.Value
}

func Str(key, val string) Field {
	return Field{Key: key, Value: val}
}

func Int(key string, val int) Field {
	return Field{Key: key, Value: strconv.Itoa(val)}
}

func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Value: val.String()}
}

func Err(err error) Field {
	return Field{Key: "err", Value: err.Error()}
}

func traceID() string {
	return fmt.Sprintf("run-%d", time.Now().UnixNano())
}
```

说明：这个小项目把配置、并发、超时、日志和错误处理串起来，是“写好 Go”的最小实践。

实践建议：如果你把它当成模板复用，记得先收敛字段与命名，保持风格统一，再逐步增加功能。

配图建议：
- 一张“capstone 结构示意图”。
- 一张“worker pool 流程图”。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/40/cmd/capstone -timeout=900ms
```

示例输出（节选）：

```
2025/12/31 16:42:31 level=INFO service=capstone msg=run start run=run-1767170551001939000 jobs=9 workers=3 timeout=900ms
2025/12/31 16:42:31 level=INFO service=capstone msg=job done run=run-1767170551001939000 worker=1 job=1 cost=121.037881ms
2025/12/31 16:42:31 level=ERROR service=capstone msg=job failed run=run-1767170551001939000 worker=1 job=9 cost=240.239805ms err=job 9: job failed
2025/12/31 16:42:31 level=INFO service=capstone msg=run summary run=run-1767170551001939000 handled=9 failed=1 canceled=0 cost=601.82221ms
```

输出解读：这次运行里，任务 9 触发失败，日志中同时包含 run id、worker、job 与耗时，便于定位问题。

如果改成更短的超时，会看到 `job canceled` 日志，这意味着任务预算被提前收回，是“可控失败”的体现。

截图描述建议：
- 截一张错误日志图，突出 **job failed** 与 **err**。
- 再截一张 summary 图，突出 **failed=1**。

配图建议：
- 一张“运行摘要指标”示意图。
- 一张“错误定位链路”图。

## 5. 常见坑 & 解决方案（必写）

1. **main 过于臃肿**：入口变成一坨逻辑。  
   解决：把流程拆成函数，入口只做组装。

2. **超时缺失**：并发任务永远不结束。  
   解决：用 `context.WithTimeout` 管理预算。

3. **错误只有字符串**：上层无法决策。  
   解决：用 `errors.Is` 识别错误类型。

4. **日志无结构**：只能肉眼搜索。  
   解决：统一 `key=value` 输出。

5. **没有运行摘要**：运行结果不可见。  
   解决：输出 `handled/failed/canceled` 统计。

6. **并发没有收敛**：goroutine 泄漏。  
   解决：严格关闭 channel，等待 `WaitGroup`。

补充建议：把最关键的运行日志与摘要落库或落文件，哪怕没有平台也能留证据，便于回溯与复盘，也更安心、更可控。

配图建议：
- 一张“常见坑清单”图。
- 一张“goroutine 生命周期”示意图。

## 6. 进阶扩展 / 思考题

1. 为日志增加 `trace` 并传递到下游调用。
2. 把配置改成“文件 + env + flag”合并。
3. 为 worker 增加重试策略，并观察失败率变化。
4. 写一组测试，确保超时与失败分支被覆盖。

补充建议：把“稳定性改进”当成长期任务，每次迭代只加一条规则，积累起来就会很强，也更可持续、更稳、更安心。

配图建议：
- 一张“工程能力路线图”。
- 一张“扩展模块清单”图。
