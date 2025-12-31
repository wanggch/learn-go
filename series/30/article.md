# net/http（客户端）：你每天都在用却没用好

你好，我是汪小成。很多人写 HTTP 客户端时只会一句 `http.Get`，线上却经常遇到“请求偶发卡死、超时不可控、连接数飙升”的问题。排查半天才发现：你没有设置超时、每次请求都新建 Client、Body 也没关。本文会先做环境准备，再解释 `net/http` 客户端的核心概念和设计原因，给出完整可运行示例、运行效果、常见坑与解决方案，最后留几个进阶思考。

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
- 本篇目录：`series/30`。
- 示例入口：`series/30/cmd/httpclient/main.go`。

### 1.2 运行命令

```bash
go run ./series/30/cmd/httpclient
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/30/cmd/httpclient
```

### 1.3 前置知识

- `context` 的超时与取消。
- `io.Reader` 读取响应体的基本流程。

配图建议：
- 一张“请求生命周期：DNS → Dial → TLS → 读响应”的示意图。
- 一张“Client/Transport 复用”关系图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `http.Client` 必须复用

**概念**：`http.Client` 内部包含连接池，应该复用而不是每次创建。  
**示例**：高并发下每次 `&http.Client{}` 会导致连接数飙升。  
**为什么这么设计**：连接复用能显著减少 TCP/TLS 建立成本，默认 Transport 就是为复用而生。

### 2.2 超时不是一个，而是一组

**概念**：`Client.Timeout` 是总超时，`Dial`/`TLSHandshake`/`ResponseHeaderTimeout` 是分阶段超时。  
**示例**：DNS 卡住时需要 Dial 超时；服务端慢响应头需要 ResponseHeaderTimeout。  
**为什么这么设计**：HTTP 请求链路长，单一超时无法覆盖所有阶段。

### 2.3 `context` 用于“每个请求”的上限

**概念**：`context.WithTimeout` 让单次请求有更细的时间约束，适合不同接口不同 SLO。  
**示例**：列表接口 300ms，详情接口 1s。  
**为什么这么设计**：请求级 SLA 更灵活，避免全局超时过于粗糙。

### 2.4 必须关闭 Body，且要限制读取大小

**概念**：不关闭 Body 会导致连接无法回收；不限制读取会被异常大响应拖垮。  
**示例**：使用 `defer resp.Body.Close()`，并搭配 `io.LimitReader`。  
**为什么这么设计**：连接池需要完整消费/关闭响应体才能复用。

### 2.5 Transport 才是“内核”

**概念**：`http.Transport` 管理连接池、复用策略和各类底层超时，Client 只是一个门面。  
**示例**：`MaxIdleConnsPerHost` 决定单个主机的空闲连接上限，`IdleConnTimeout` 决定连接闲置多久被回收。  
**为什么这么设计**：把底层连接管理和上层请求逻辑解耦，便于在不同场景下复用 Transport。

### 2.6 短连接并不更“安全”

**概念**：强行关闭 keep-alive 会让每次请求都走完整的 TCP/TLS 建连流程。  
**示例**：高 QPS 场景下，频繁建连会放大延迟与 CPU 消耗。  
**为什么这么设计**：HTTP/1.1 默认支持长连接，复用是性能优化的基础设施。

小结：客户端最佳实践可以归纳为三件事——**复用 Client、明确超时、正确处理 Body**。如果还能在调用层做一层统一封装，把日志、指标和错误分类集中管理，线上排障会轻松很多，尤其在高并发场景。

配图建议：
- 一张“超时分层示意图”。
- 一张“Body 未关闭导致连接泄漏”的示意图。

## 3. 完整代码示例（可运行）

示例包含：

1. 用自定义 `RoundTripper` 模拟快/慢响应（不依赖监听端口）。
2. 构建两个 Client（不同超时）。
3. 演示全局超时与请求级超时的差异。

代码路径：`series/30/cmd/httpclient/main.go`。

```go
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func main() {
	base := "http://mock.local"

	fastClient := newClient(800 * time.Millisecond)
	slowClient := newClient(2 * time.Second)

	fmt.Println("base:", base)

	doGet("fast /fast", fastClient, base+"/fast", 0)
	doGet("fast /slow", fastClient, base+"/slow", 0)
	doGet("slow /slow", slowClient, base+"/slow", 0)
	doGet("slow /slow (ctx 500ms)", slowClient, base+"/slow", 500*time.Millisecond)

	fastClient.CloseIdleConnections()
	slowClient.CloseIdleConnections()
}

func newClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: mockTransport{},
	}
}

type mockTransport struct{}

func (mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	delay := 80 * time.Millisecond
	body := `{"ok":true,"path":"fast"}`
	if strings.Contains(req.URL.Path, "/slow") {
		delay = 1200 * time.Millisecond
		body = `{"ok":true,"path":"slow"}`
	}

	select {
	case <-time.After(delay):
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")

	return resp, nil
}

func (mockTransport) CloseIdleConnections() {}

func doGet(label string, client *http.Client, url string, ctxTimeout time.Duration) {
	start := time.Now()

	ctx := context.Background()
	if ctxTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ctxTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
	fmt.Printf("%s: build request error=%v\n", label, err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%s: error=%v cost=%s\n", label, err, time.Since(start))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		fmt.Printf("%s: read error=%v cost=%s\n", label, err, time.Since(start))
		return
	}

	fmt.Printf("%s: status=%d cost=%s body=%s\n", label, resp.StatusCode, time.Since(start), string(body))
}
```

说明：示例用 `mockTransport` 模拟快慢响应，避免实际监听端口。真实业务中可替换成 `http.Transport` 并设置 Dial/TLS/ResponseHeaderTimeout 等参数。

配图建议：
- 一张“模拟服务 + 客户端调用”的示意图。
- 一张“请求耗时对比柱状图”。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/30/cmd/httpclient
```

示例输出（节选）：

```
base: http://mock.local
fast /fast: status=200 cost=80.94951ms body={"ok":true,"path":"fast"}
fast /slow: error=Get "http://mock.local/slow": context deadline exceeded (Client.Timeout exceeded while awaiting headers) cost=800.264892ms
slow /slow: status=200 cost=1.200475796s body={"ok":true,"path":"slow"}
slow /slow (ctx 500ms): error=Get "http://mock.local/slow": context deadline exceeded cost=500.076059ms
```

截图描述建议：
- 截一张终端输出图，突出 **全局超时** 与 **请求级超时** 的差异。
- 如果可以，用红框标出超时错误与耗时值。

配图建议：
- 一张“快/慢接口对比”截图。
- 一张“超时错误提示”的局部放大图。

## 5. 常见坑 & 解决方案（必写）

1. **不设置超时**：网络抖动或服务端卡死会让请求无限挂起。  
   解决：设置 `Client.Timeout`，并为关键请求设置 `context` 超时。

2. **每次请求新建 Client**：连接无法复用，导致连接数激增。  
   解决：全局复用一个 `http.Client`（或按业务复用多个）。

3. **忘记关闭 Body**：连接无法回收，最终耗尽连接池。  
   解决：`defer resp.Body.Close()`，必要时读完或丢弃 body。

4. **不限制响应大小**：异常大响应会拖垮内存。  
   解决：用 `io.LimitReader` 或 `http.MaxBytesReader` 限制。

5. **忽略状态码**：只看 err 不看 `StatusCode`，导致业务误判。  
   解决：统一封装响应检查逻辑，明确成功与失败。

6. **错误复用 `http.DefaultClient`**：默认超时为 0，适合 demo 不适合生产。  
   解决：显式创建配置好的 Client。

配图建议：
- 一张“常见坑清单”表格图。
- 一张“连接池耗尽”的示意图。

## 6. 进阶扩展 / 思考题

1. 为客户端加上重试策略，如何避免“雪崩式重试”？
2. 观察 `Transport` 的 `MaxIdleConnsPerHost` 对吞吐的影响。
3. 如果需要代理或自定义 DNS，你会如何扩展 Dialer？
4. 尝试加入 `httptrace`，输出 DNS / TLS / 连接复用的耗时分布。

配图建议：
- 一张“重试与退避策略”曲线图。
- 一张“连接复用时序图”。
