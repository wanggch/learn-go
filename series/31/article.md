# net/http（服务端）：用标准库写一个 API

你好，我是汪小成。很多人第一次写 Go 服务端，会陷入两个极端：要么只会 `http.HandleFunc` 草草上阵，结果路由混乱、状态码随缘、错误无法统一；要么一上来就引框架，业务一变就被框架绑住。其实 `net/http` 已经足够写出清晰可维护的 API，只是你没把它的“核心模式”用起来。本文会先准备环境，再讲清 Handler、路由、Middleware、状态码等核心概念，接着给出完整可运行示例、运行效果与截图描述，并整理常见坑与进阶思考。

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
- 本篇目录：`series/31`。
- 示例入口：`series/31/cmd/httpapi/main.go`。

### 1.2 运行命令

```bash
go run ./series/31/cmd/httpapi
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/31/cmd/httpapi
```

### 1.3 前置知识

- 了解 `encoding/json` 的解码流程。
- 了解 `context` 的基础用法（本篇未深入）。

配图建议：
- 一张“请求 → Handler → 中间件 → Response”的流程图。
- 一张“标准库 API 架构图”。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `http.Handler` 是最小抽象

**概念**：服务端的所有入口都围绕 `ServeHTTP`。  
**示例**：`http.HandlerFunc` 让普通函数也能变成 handler。  
**为什么这么设计**：把“网络层”与“业务处理”解耦，便于组合与复用。

### 2.2 `ServeMux` 负责“最小路由”

**概念**：`ServeMux` 只做前缀匹配，不支持复杂路由。  
**示例**：`/orders/` 处理订单详情，`/orders` 处理列表或创建。  
**为什么这么设计**：标准库保持简单，给你足够的自由组合。

### 2.3 Middleware 是复用的关键

**概念**：日志、恢复、统一响应头等都应该做成中间件。  
**示例**：`recoverMiddleware` 捕获 panic，`jsonMiddleware` 统一输出 JSON。  
**为什么这么设计**：把横切逻辑放在链路外层，避免每个 handler 重复。

### 2.4 状态码是 API 的“契约”

**概念**：不同错误要用不同状态码，客户端才能可靠判断。  
**示例**：`400` 参数错误、`404` 资源不存在、`405` 方法不允许。  
**为什么这么设计**：HTTP 本身就是协议，状态码是语义的一部分。

### 2.5 JSON 解码要“严格且有限”

**概念**：未知字段默认会被忽略，过大的请求体可能拖垮服务。  
**示例**：`DisallowUnknownFields` + `io.LimitReader`。  
**为什么这么设计**：服务端更应“尽早失败”，减少隐性错误。

### 2.6 用 `httptest` 做“无端口演示”

**概念**：标准库提供 `httptest`，可在内存中模拟请求。  
**示例**：`httptest.NewRecorder` + `ServeHTTP` 直测 handler。  
**为什么这么设计**：不依赖真实监听端口，也能验证 API 行为。

### 2.7 Server 级别超时与资源边界

**概念**：`http.Server` 提供 `ReadTimeout`/`WriteTimeout` 等整体约束，避免慢连接拖垮服务。  
**示例**：对上传类接口设置更长写超时，对普通读请求保持更严格限制。  
**为什么这么设计**：服务端必须主动划出边界，避免少量异常请求占用大量资源。

配图建议：
- 一张“中间件链路示意图”。
- 一张“状态码与错误语义表”。

## 3. 完整代码示例（可运行）

示例包含：

1. `ServeMux` 路由（健康检查、订单列表、订单详情）。
2. 中间件链（恢复、日志、统一 JSON）。
3. 严格 JSON 解码 + 响应封装。
4. `httptest` 模拟请求，观察输出。

代码路径：`series/31/cmd/httpapi/main.go`。

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type order struct {
	ID        int    `json:"id"`
	Item      string `json:"item"`
	Price     int    `json:"price"`
	CreatedAt string `json:"created_at"`
}

type createOrderRequest struct {
	Item  string `json:"item"`
	Price int    `json:"price"`
}

type store struct {
	mu     sync.RWMutex
	nextID int
	items  map[int]order
}

func newStore() *store {
	return &store{
		nextID: 1000,
		items:  make(map[int]order),
	}
}

func (s *store) create(req createOrderRequest) order {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	ord := order{
		ID:        s.nextID,
		Item:      req.Item,
		Price:     req.Price,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.items[ord.ID] = ord
	return ord
}

func (s *store) list() []order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]order, 0, len(s.items))
	for _, ord := range s.items {
		result = append(result, ord)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (s *store) get(id int) (order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ord, ok := s.items[id]
	return ord, ok
}

type api struct {
	store *store
}

func main() {
	handler := buildHandler()

	fmt.Println("=== net/http server demo ===")
	simulate(handler, http.MethodGet, "/health", nil)
	simulate(handler, http.MethodPost, "/orders", createOrderRequest{Item: "latte", Price: 28})
	simulate(handler, http.MethodPost, "/orders", createOrderRequest{Item: "sandwich", Price: 38})
	simulate(handler, http.MethodGet, "/orders", nil)
	simulate(handler, http.MethodGet, "/orders/1001", nil)
	simulate(handler, http.MethodGet, "/orders/4040", nil)
	simulate(handler, http.MethodPut, "/orders/1001", nil)
}

func buildHandler() http.Handler {
	store := newStore()
	api := &api{store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.handleHealth)
	mux.HandleFunc("/orders", api.handleOrders)
	mux.HandleFunc("/orders/", api.handleOrder)

	return chain(mux, recoverMiddleware, logMiddleware, jsonMiddleware)
}

func (a *api) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.store.list())
	case http.MethodPost:
		var req createOrderRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Item == "" || req.Price <= 0 {
			writeError(w, http.StatusBadRequest, "item and price are required")
			return
		}
		ord := a.store.create(req)
		writeJSON(w, http.StatusCreated, ord)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *api) handleOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/orders/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	ord, ok := a.store.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, http.StatusOK, ord)
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

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s cost=%s\n", r.Method, r.URL.Path, time.Since(start))
	})
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	limited := io.LimitReader(r.Body, 1<<20)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("invalid json: unexpected extra data")
	}
	return nil
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

func simulate(handler http.Handler, method, path string, payload any) {
	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, "http://api.local"+path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	fmt.Printf("-> %s %s status=%d body=%s\n", method, path, rec.Code, strings.TrimSpace(rec.Body.String()))
}
```

说明：这里用 `httptest` 模拟请求，避免实际监听端口。真实部署时可直接调用 `http.ListenAndServe`，把 `buildHandler()` 作为入口即可。

配图建议：
- 一张“模拟请求 + Handler 输出”的示意图。
- 一张“中间件链路”的结构图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/31/cmd/httpapi
```

示例输出（节选）：

```
=== net/http server demo ===
GET /health cost=73.701µs
-> GET /health status=200 body={
  "status": "ok"
}
POST /orders cost=288.098µs
-> POST /orders status=201 body={
  "id": 1001,
  "item": "latte",
  "price": 28,
  "created_at": "2025-12-31T14:37:00+08:00"
}
GET /orders/4040 cost=4.71µs
-> GET /orders/4040 status=404 body={
  "error": "order not found"
}
```

截图描述建议：
- 截一张终端输出图，突出 **日志中间件** 与 **API 响应体** 的对应关系。
- 截一张“404 与 405”对比图，强调状态码语义。

配图建议：
- 一张“请求流转路径”图。
- 一张“状态码映射表”的截图。

## 5. 常见坑 & 解决方案（必写）

1. **忘记设置状态码**：默认是 200，错误也会被当成成功。  
   解决：统一封装 `writeJSON` / `writeError`。

2. **未知字段静默忽略**：客户端拼错字段，服务端无感知。  
   解决：解码时启用 `DisallowUnknownFields`。

3. **请求体过大**：恶意或误操作导致内存暴涨。  
   解决：`io.LimitReader` 或 `http.MaxBytesReader` 做保护。

4. **路由冲突**：`/orders` 与 `/orders/` 处理逻辑混乱。  
   解决：显式区分“集合”与“单资源”。

5. **没有统一错误结构**：前端或调用方无法稳定解析。  
   解决：统一错误 JSON 结构，如 `{"error":"..."}`。

6. **中间件顺序错误**：日志、恢复、鉴权顺序错会影响结果。  
   解决：明确链路顺序，从外到内可读性更好。

配图建议：
- 一张“常见坑清单”思维导图。
- 一张“中间件顺序示意图”。

## 6. 进阶扩展 / 思考题

1. 尝试为 API 增加 `context` 超时，避免慢请求拖垮服务。
2. 把 `store` 抽象为接口，写一个 mock，思考如何做单元测试。
3. 增加一个 `PUT /orders/{id}`，支持更新订单字段。
4. 用 `http.Server` 手动控制 `ReadTimeout` / `WriteTimeout`，比较差异。

配图建议：
- 一张“超时控制点位图”。
- 一张“可测试性改造”结构图。
