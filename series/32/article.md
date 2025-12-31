# context + HTTP：请求生命周期管理

你好，我是汪小成。很多人写 HTTP 服务时都会遇到“偶发慢请求、超时不生效、日志对不上请求”的困扰。你以为是业务慢，其实是 **请求生命周期没有被管理**：超时没设置，取消没处理，日志没有关联 request id。本文会先做环境准备，再解释 context 在 HTTP 中的核心概念与设计原因，最后给出完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/32`。
- 示例入口：`series/32/cmd/ctxhttp/main.go`。

### 1.2 运行命令

```bash
go run ./series/32/cmd/ctxhttp
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/32/cmd/ctxhttp
```

### 1.3 前置知识

- 熟悉 `net/http` handler 的基本写法。
- 理解 `context.WithTimeout` / `context.WithCancel` 的基础概念。

配图建议：
- 一张“请求生命周期（接入→处理→响应→结束）”流程图。
- 一张“Context 取消信号传播路径”示意图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `r.Context()` 是请求的“生命线”

**概念**：每个 HTTP 请求自带 `Context`，随请求创建、随请求结束而取消。  
**示例**：在 handler 中使用 `r.Context()` 处理耗时任务。  
**为什么这么设计**：让请求取消、超时、链路追踪都能统一管理。

### 2.2 超时的“最短优先”

**概念**：多个超时叠加时，以最早到期的 deadline 为准。  
**示例**：客户端 300ms + 服务端 700ms，实际会在 300ms 触发取消。  
**为什么这么设计**：任何一方的预算耗尽，都应及时终止请求。

### 2.3 中间件是上下文的传递带

**概念**：中间件可以在 Context 中附加 request id 或 tracing 信息。  
**示例**：`requestIDMiddleware` 给每个请求注入 `req-0001`。  
**为什么这么设计**：把“链路关联信息”从业务逻辑中剥离出来。

### 2.4 正确处理取消与超时

**概念**：耗时任务必须监听 `ctx.Done()`，否则超时无效。  
**示例**：`select` 中监听 `time.After` 与 `ctx.Done()`。  
**为什么这么设计**：让服务端能够“及时止损”，释放资源。

### 2.5 `httptest` 做无端口演示

**概念**：不需要真实监听端口，也能模拟请求并验证响应。  
**示例**：`httptest.NewRequest` + `ServeHTTP`。  
**为什么这么设计**：适合测试和文档示例，避免外部环境依赖。

### 2.6 deadline 传递到下游调用

**概念**：同一个 `Context` 传给数据库或 RPC 调用，能把超时预算统一起来。  
**示例**：`db.QueryContext(ctx, ...)` 会在 deadline 到达时自动中断。  
**为什么这么设计**：避免上游已经超时，下游还在做无意义的耗时工作。

配图建议：
- 一张“客户端超时 vs 服务端超时”对比图。
- 一张“中间件链路 + Context 注入点”结构图。

## 3. 完整代码示例（可运行）

示例包含：

1. 通过中间件注入 `request_id`。
2. 服务端统一超时（700ms）。
3. 客户端请求级超时（300ms）。
4. 任务处理逻辑监听 `ctx.Done()`。

代码路径：`series/32/cmd/ctxhttp/main.go`。

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

var reqSeq int64

func main() {
	handler := buildHandler(700 * time.Millisecond)

	fmt.Println("=== context + http demo ===")
	simulate(handler, "fast", "/fast", 0)
	simulate(handler, "slow (client 300ms)", "/slow", 300*time.Millisecond)
	simulate(handler, "slow (server 700ms)", "/slow", 0)
}

func buildHandler(timeout time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/fast", handleWork(80*time.Millisecond))
	mux.HandleFunc("/slow", handleWork(1200*time.Millisecond))

	return chain(mux,
		recoverMiddleware,
		requestIDMiddleware,
		timeoutMiddleware(timeout),
		logMiddleware,
		jsonMiddleware,
	)
}

func handleWork(delay time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		if err := work(r.Context(), delay); err != nil {
			status := http.StatusGatewayTimeout
			if errors.Is(err, context.Canceled) {
				status = http.StatusRequestTimeout
			}
			writeError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"request_id": requestIDFromContext(r.Context()),
			"delay":      delay.String(),
			"message":    "ok",
		})
	}
}

func work(ctx context.Context, delay time.Duration) error {
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func chain(h http.Handler, m ...func(http.Handler) http.Handler) http.Handler {
	wrapped := h
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}
	return wrapped
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := atomic.AddInt64(&reqSeq, 1)
		reqID := fmt.Sprintf("req-%04d", id)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s rid=%s cost=%s\n", r.Method, r.URL.Path, requestIDFromContext(r.Context()), time.Since(start))
	})
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "unknown"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func simulate(handler http.Handler, label, path string, clientTimeout time.Duration) {
	ctx := context.Background()
	if clientTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, clientTimeout)
		defer cancel()
	}

	req := httptest.NewRequest(http.MethodGet, "http://api.local"+path, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rec, req)
	cost := time.Since(start)
	body := strings.TrimSpace(rec.Body.String())
	fmt.Printf("-> %s status=%d cost=%s body=%s\n", label, rec.Code, cost, body)
}
```

说明：这里用 `httptest` 在内存里模拟请求，强调 **Context 超时如何传递**。真实服务只需要把 `buildHandler()` 放到 `http.ListenAndServe` 中即可。

配图建议：
- 一张“请求进入 → 中间件注入 → handler 处理”的流程图。
- 一张“ctx.Done() 触发时机”时间线图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/32/cmd/ctxhttp
```

示例输出（节选）：

```
=== context + http demo ===
GET /fast rid=req-0001 cost=81.342395ms
-> fast status=200 cost=81.466824ms body={
  "delay": "80ms",
  "message": "ok",
  "request_id": "req-0001"
}
GET /slow rid=req-0002 cost=300.064959ms
-> slow (client 300ms) status=504 cost=300.149707ms body={
  "error": "context deadline exceeded"
}
GET /slow rid=req-0003 cost=700.36405ms
-> slow (server 700ms) status=504 cost=700.45671ms body={
  "error": "context deadline exceeded"
}
```

输出解读：第一条快速请求完成；第二条因为客户端只给了 300ms，最先触发取消；第三条没有客户端限制，但服务端的 700ms 预算耗尽，同样返回超时。这也是最短超时优先的直观体现，定位问题更直接。

截图描述建议：
- 截一张终端输出图，突出 **request_id** 与 **超时耗时** 的对应关系。
- 再截一张“快/慢请求”对比图，强调最短超时生效。

配图建议：
- 一张“请求时间预算”示意图。
- 一张“超时链路”对照图。

## 5. 常见坑 & 解决方案（必写）

1. **只设置超时不监听 ctx.Done()**：超时不会真正生效。  
   解决：耗时任务必须用 `select` 监听 `ctx.Done()`。

2. **忘记传递 Context**：子函数还是用 `context.Background()`。  
   解决：把 `ctx` 作为参数层层传递。

3. **滥用 Context Value**：把大量业务数据塞进 Context。  
   解决：Context 只放请求级元数据，如 request id 或 trace id。

4. **超时设置过短**：正常请求被误杀。  
   解决：根据接口 SLO 分级设置超时。

5. **错误码混乱**：超时和业务失败混在一起。  
   解决：统一封装错误返回，区分超时与业务错误。

6. **忽略日志关联**：定位问题无法串起一条请求链。  
   解决：request id 在日志与响应中保持一致。

配图建议：
- 一张“Context 使用误区”列表图。
- 一张“超时策略分级表”。

## 6. 进阶扩展 / 思考题

1. 为每个请求增加 `trace_id`，并在日志中输出。
2. 用 `http.Server` 的 `ReadTimeout`/`WriteTimeout` 做更完整的超时保护。
3. 在 handler 内部引入 goroutine 时，如何避免 goroutine 泄漏？
4. 把 `request_id` 注入到响应 header（如 `X-Request-ID`）。

配图建议：
- 一张“全链路追踪字段”示意图。
- 一张“goroutine 泄漏防护”流程图。
