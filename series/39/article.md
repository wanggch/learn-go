# 错误、日志与可观测性设计

你好，我是汪小成。很多系统出问题时最尴尬的不是“报错”，而是“没线索”：日志没有请求标识、错误只是字符串、定位只能靠猜。可观测性并不是上复杂平台，而是先把 **错误与日志的语义** 做对。本文会先准备环境，再讲清错误包装、结构化日志与 trace 的核心概念，最后给出完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/39`。
- 示例入口：`series/39/cmd/obslog/main.go`。

### 1.2 运行命令

正常运行（随机 ID）：

```bash
go run ./series/39/cmd/obslog
```

固定错误场景（可复现超时）：

```bash
INVOICE_ID=35 go run ./series/39/cmd/obslog
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
INVOICE_ID=35 GOCACHE=$(pwd)/.cache/go-build go run ./series/39/cmd/obslog
```

### 1.3 前置知识

- 了解 `errors.Is` / `errors.As` 的基本用法。
- 了解结构体与接口的基础概念。

提示：本文用最小可用示例，不依赖外部日志库，重点在设计思路。

小建议：先从“错误分类 + 结构化日志”这两个最小要素做起，再逐步补指标与追踪，效果最稳。

配图建议：
- 一张“错误与日志的关系图”。
- 一张“可观测性三要素（日志/指标/追踪）”示意图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 错误要“可归类”

**概念**：错误不是字符串，而是可以被识别的类型或类型标签。  
**示例**：`Kind=timeout` 与 `Kind=not_found`。  
**为什么这么设计**：调用方可以基于类别做回退或告警，而不是靠字符串匹配。

### 2.2 错误要“可追溯”

**概念**：在错误中保留 `Trace` 或 `Op`，便于定位发生位置。  
**示例**：`Wrap("fetchInvoice", "timeout", trace, err)`。  
**为什么这么设计**：错误链条可追踪，排障效率更高。

### 2.3 日志要结构化

**概念**：日志应该输出 key=value，方便检索。  
**示例**：`level=ERROR service=billing trace=...`。  
**为什么这么设计**：结构化日志是查询和报警的基础。

### 2.4 同一请求要有统一 Trace

**概念**：一次请求内的日志要共享同一个 trace id。  
**示例**：请求级 trace + 错误级 err_trace 同时输出。  
**为什么这么设计**：快速串起“同一次请求”的所有日志。

### 2.5 错误处理逻辑要“可决策”

**概念**：不同类型错误应走不同分支。  
**示例**：超时走缓存，找不到走通知，其他走告警。  
**为什么这么设计**：避免所有错误都走同一种处理方式。

### 2.6 日志内容要“业务可读”

**概念**：日志字段要能对应业务语义。  
**示例**：`invoice_id`、`amount`、`cost`。  
**为什么这么设计**：业务人员和开发都能读懂。

### 2.7 错误要“可保留原因”

**概念**：错误包装后仍要保留原始错误，便于上层判断。  
**示例**：`errors.Is(err, errTimeout())` 仍然成立。  
**为什么这么设计**：这样既能分类，也能保留原始上下文。

### 2.8 结构化日志是观测的最小可用形态

**概念**：哪怕没有日志平台，也应该先保证格式统一。  
**示例**：统一输出 `level`、`service`、`trace`。  
**为什么这么设计**：后续接入平台时成本最低。
配图建议：
- 一张“错误分类树”示意图。
- 一张“trace 与 err_trace 关系图”。

## 3. 完整代码示例（可运行）

示例包含：

1. 自定义错误结构（带 kind、op、trace）。
2. 简单结构化日志器（key=value 输出）。
3. 一个“发票查询”流程，展示正常与异常分支。

代码路径：`series/39/cmd/obslog/main.go` 与 `series/39/internal/obs/obs.go`。

```go
package obs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Logger struct {
	service string
	logger  *log.Logger
}

type AppError struct {
	Op    string
	Kind  string
	Err   error
	Trace string
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

func (l *Logger) Error(err error, fields ...Field) {
	l.emit("ERROR", "error", append(fields, Field{"err", err.Error()})...)
}

func (l *Logger) ErrorWithTrace(err error, fields ...Field) {
	if app, ok := AsAppError(err); ok && app.Trace != "" {
		fields = append(fields, Field{Key: "err_trace", Value: app.Trace})
	}
	l.Error(err, fields...)
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

type Field struct {
	Key   string
	Value string
}

func (f Field) String() string {
	return f.Key + "=" + f.Value
}

func Str(key, val string) Field {
	return Field{Key: key, Value: val}
}

func Int(key string, val int) Field {
	return Field{Key: key, Value: fmt.Sprintf("%d", val)}
}

func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Value: val.String()}
}

func Wrap(op, kind, trace string, err error) error {
	if err == nil {
		return nil
	}
	return AppError{Op: op, Kind: kind, Trace: trace, Err: err}
}

func (e AppError) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Err)
}

func (e AppError) Unwrap() error {
	return e.Err
}

func IsKind(err error, kind string) bool {
	var app AppError
	if errors.As(err, &app) {
		return app.Kind == kind
	}
	return false
}

func AsAppError(err error) (AppError, bool) {
	var app AppError
	if errors.As(err, &app) {
		return app, true
	}
	return AppError{}, false
}

func TraceID() string {
	return fmt.Sprintf("trace-%d", time.Now().UnixNano())
}
```

```go
package main

import (
	"errors"
	"math/rand"
	"os"
	"strconv"
	"time"

	"learn-go/series/39/internal/obs"
)

const (
	kindTimeout  = "timeout"
	kindNotFound = "not_found"
)

func main() {
	logger := obs.NewLogger("billing")

	trace := obs.TraceID()
	start := time.Now()

	rand.Seed(time.Now().UnixNano())
	id := randomID()
	if v := os.Getenv("INVOICE_ID"); v != "" {
		if forced, err := strconv.Atoi(v); err == nil && forced > 0 {
			id = forced
		}
	}
	amount, err := fetchInvoice(id)
	if err != nil {
		logger.ErrorWithTrace(err,
			obs.Str("trace", trace),
			obs.Str("op", "fetch_invoice"),
			obs.Int("invoice_id", id),
			obs.Duration("cost", time.Since(start)),
		)

		switch {
		case errors.Is(err, errTimeout()):
			logger.Info("fallback to cache", obs.Str("trace", trace))
		case obs.IsKind(err, kindNotFound):
			logger.Info("notify billing", obs.Str("trace", trace))
		default:
			logger.Info("alert oncall", obs.Str("trace", trace))
		}
		return
	}

	logger.Info("invoice loaded",
		obs.Str("trace", trace),
		obs.Int("invoice_id", id),
		obs.Int("amount", amount),
		obs.Duration("cost", time.Since(start)),
	)
}

func fetchInvoice(id int) (int, error) {
	trace := obs.TraceID()
	if id%5 == 0 {
		return 0, obs.Wrap("fetchInvoice", kindTimeout, trace, errTimeout())
	}
	if id%7 == 0 {
		return 0, obs.Wrap("fetchInvoice", kindNotFound, trace, errMissing())
	}
	return id * 10, nil
}

func errTimeout() error {
	return errors.New("db timeout")
}

func errMissing() error {
	return errors.New("invoice missing")
}

func randomID() int {
	return rand.Intn(50) + 1
}
```

说明：日志中既有请求级 `trace`，也有错误链条里的 `err_trace`，能同时定位“请求路径”与“错误来源”。

实践建议：如果你发现日志里只有 `err_trace` 没有 `trace`，说明链路没有做统一传递，要优先补齐链路追踪。

配图建议：
- 一张“日志字段说明”图。
- 一张“错误链条流转”示意图。

## 4. 运行效果 + 截图描述

正常运行（随机成功）：

```bash
go run ./series/39/cmd/obslog
```

示例输出（节选）：

```
2025/12/31 15:55:13 level=INFO service=billing msg=invoice loaded trace=trace-1767167713258338000 invoice_id=26 amount=260 cost=68.651µs
```

固定错误场景（超时）：

```bash
INVOICE_ID=35 go run ./series/39/cmd/obslog
```

示例输出（节选）：

```
2025/12/31 15:56:21 level=ERROR service=billing msg=error trace=trace-1767167781799955000 op=fetch_invoice invoice_id=35 cost=85.34µs err_trace=trace-1767167781800052000 err=fetchInvoice: timeout: db timeout
2025/12/31 15:56:21 level=INFO service=billing msg=alert oncall trace=trace-1767167781799955000
```

输出解读：同一请求的日志用 `trace` 关联，错误内部用 `err_trace` 指向错误来源，两者结合可以快速定位问题位置。

截图描述建议：
- 截一张错误日志图，突出 **trace / err_trace** 的双链路。
- 再截一张 fallback/alert 日志图，强调错误处理分支。

配图建议：
- 一张“日志结构化输出示意图”。
- 一张“错误分类与处理分支”流程图。

## 5. 常见坑 & 解决方案（必写）

1. **错误只用字符串**：无法分类、无法自动处理。  
   解决：用错误类型或 `Kind` 标签。

2. **日志缺少 trace**：无法串起同一次请求。  
   解决：生成并传递 trace id。

3. **错误未包装**：上层拿不到上下文信息。  
   解决：在边界处 `Wrap`，并保留 op 与 kind。

4. **日志格式不统一**：不同模块输出格式不同，难检索。  
   解决：统一 `key=value` 结构。

5. **过度日志**：什么都写，噪音太大。  
   解决：只记录必要字段，避免业务敏感信息。

6. **错误处理一刀切**：超时与业务缺失混在一起。  
   解决：用 `errors.Is` 和 `IsKind` 做决策。

配图建议：
- 一张“常见坑清单”图。
- 一张“日志噪音 vs 价值”对比图。

## 6. 进阶扩展 / 思考题

1. 把 `Logger` 替换成 JSON 输出，看看检索体验差异。
2. 引入 `context`，把 trace id 注入上下游调用。
3. 为 `AppError` 增加 `Code` 字段，用于 API 返回。
4. 把错误统计做成指标（如 timeout 次数）。

配图建议：
- 一张“日志格式对比图”。
- 一张“指标化错误统计”示意图。
