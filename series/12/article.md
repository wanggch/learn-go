# 组合优于继承：Go 的设计思维

大家好，我是汪小成。你是否遇到过这种“继承地狱”：为了在工单处理里复用日志、指标和校验逻辑，继承层层嵌套，方法名冲突、状态串改，改一处动全身。Go 没有继承关键字，取而代之的是“组合 + 方法集提升”。用组合把能力拼在一起，用接口约束协作边界，代码更可预测、更易测试。本文用一个“工单处理”例子，展示组合、嵌入、接口拆分与无侵入替换的实践。

## 目录

1. 环境准备 / 前提知识
2. 核心概念解释（概念 → 示例 → 设计原因）
3. 完整代码示例（可运行）
4. 运行效果 + 截图描述
5. 常见坑 & 解决方案
6. 进阶扩展 / 思考题

## 1. 环境准备 / 前提知识

- Go 1.22+；`go version` 确认。Git + gofmt/go vet 默认随 Go 安装。
- 目录：`series/12`；运行示例：`go run ./series/12/cmd/composite`（沙盒可用 `GOCACHE=$(pwd)/.cache/go-build`）。
- 基础：熟悉 struct、方法集、接口（前几篇已覆盖）。

配图建议：目录树截图；“组合 vs 继承”对比图；方法提升示意图。

## 2. 核心概念解释

### 2.1 组合 vs 继承

- 组合：把独立能力（日志、指标、校验）作为字段嵌入/引用，按需拼装。
- 继承：在 Go 中不存在 class 继承链；避免“父类决定子类行为”的紧耦合。
- 设计原因：组合让依赖关系显式、可替换，减少“超类影响子类”的隐性副作用。

### 2.2 嵌入（embedding）与方法提升

- 匿名字段可把方法集提升到外层，调用更简洁；冲突时需显式引用。
- 示例：`Processor` 嵌入 `Logger` 和 `*Metrics`，直接调用 `p.Info()`、`p.MarkSuccess()`。
- 设计原因：在保持组合灵活性的同时，提供方便的调用语法。

### 2.3 接口拆分与无侵入替换

- 最小接口：`Validator` 只有 `Validate`，`Handler` 只有 `Process`。
- 通过接口参数解耦：无需修改 `Processor`，就能替换成 `DryRunHandler`。
- 设计原因：调用方定义接口，便于测试（fake/闭包实现）和扩展。

### 2.4 状态独立，避免共享副作用

- 组合保留各自状态：`UploadHandler` 有自己的 `store`；`Processor` 自己的指标。
- 避免继承式“共享父类字段”导致的状态串改。
- 设计原因：数据局部化，易于并发和测试。

### 2.5 构造函数 + 零值策略

- 提供 `NewProcessor`、`NewUploadHandler` 初始化必需字段。
- 零值仍可用：嵌入的 `Metrics`、map 需初始化。
- 设计原因：明确必填项，减少“半初始化”对象。

配图建议：组件拼装图（Processor + Logger + Metrics + Handler/Validator），接口替换示意。

## 3. 完整代码示例（可复制运行）

入口：`series/12/cmd/composite/main.go`。场景：处理文件上传任务，要求后缀校验、可切换 Handler（真实上传/演练），并记录指标。

```go
package main

import (
	"fmt"
	"strings"
)

type Logger struct {
	Prefix string
}

func (l Logger) Info(msg string) {
	fmt.Printf("[%s] %s\n", l.Prefix, msg)
}

// Embeddable behavior: adding metrics.
type Metrics struct {
	success int
	failed  int
}

func (m *Metrics) MarkSuccess() {
	m.success++
}

func (m *Metrics) MarkFailed() {
	m.failed++
}

func (m Metrics) Snapshot() (int, int) {
	return m.success, m.failed
}

// Core component: Processor holds behavior via embedding.
type Processor struct {
	Logger
	*Metrics
	Name string
}

func NewProcessor(name string) *Processor {
	return &Processor{
		Logger:  Logger{Prefix: name},
		Metrics: &Metrics{},
		Name:    name,
	}
}

// High-level method built from composed behaviors.
func (p *Processor) Handle(items []string, validator Validator, handler Handler) error {
	if validator == nil || handler == nil {
		return fmt.Errorf("validator/handler is nil")
	}
	p.Info(fmt.Sprintf("handling %d items", len(items)))

	for _, item := range items {
		if err := validator.Validate(item); err != nil {
			p.MarkFailed()
			p.Info(fmt.Sprintf("skip invalid item %q: %v", item, err))
			continue
		}
		if err := handler.Process(item); err != nil {
			p.MarkFailed()
			p.Info(fmt.Sprintf("process failed for %q: %v", item, err))
			continue
		}
		p.MarkSuccess()
	}
	return nil
}

// Interfaces kept minimal.
type Validator interface {
	Validate(item string) error
}

type Handler interface {
	Process(item string) error
}

// Concrete validator using composition (no inheritance).
type SuffixValidator struct {
	AllowedSuffix string
}

func (v SuffixValidator) Validate(item string) error {
	if !strings.HasSuffix(item, v.AllowedSuffix) {
		return fmt.Errorf("suffix must be %q", v.AllowedSuffix)
	}
	return nil
}

// Handler that embeds Logger for reuse but keeps its own state.
type UploadHandler struct {
	Logger
	store map[string]string
}

func NewUploadHandler(prefix string) *UploadHandler {
	return &UploadHandler{
		Logger: Logger{Prefix: prefix},
		store:  make(map[string]string),
	}
}

func (h *UploadHandler) Process(item string) error {
	h.Info("uploading " + item)
	h.store[item] = "uploaded"
	return nil
}

// Handler demonstrating alternative behavior.
type DryRunHandler struct {
	Logger
}

func (h DryRunHandler) Process(item string) error {
	h.Info("dry-run " + item)
	return nil
}

func main() {
	fmt.Println("=== 组合优于继承：行为嵌入示例 ===")

	items := []string{"report.pdf", "avatar.png", "notes.txt"}

	validator := SuffixValidator{AllowedSuffix: ".png"}
	uploader := NewUploadHandler("uploader")
	dry := DryRunHandler{Logger{Prefix: "dry"}}

	proc := NewProcessor("processor")
	proc.Handle(items, validator, uploader)
	ok, fail := proc.Snapshot()
	fmt.Printf("Uploader metrics: success=%d failed=%d\n", ok, fail)

	fmt.Println("\n切换 Handler 为 DryRun（无侵入替换）")
	proc2 := NewProcessor("processor-dry")
	proc2.Handle(items, validator, dry)
	ok, fail = proc2.Snapshot()
	fmt.Printf("DryRun metrics: success=%d failed=%d\n", ok, fail)
}
```

代码看点：

- 嵌入 Logger、Metrics：方法直接提升，减少样板；各自状态独立。
- 接口拆分：Validator/Handler 分别定义，方便替换、测试。
- Handler 替换：从 Upload 换成 DryRun，无需改 Processor。

## 4. 运行效果 + 截图描述

运行：

```bash
go run ./series/12/cmd/composite
```

节选输出：

``+
=== 组合优于继承：行为嵌入示例 ===
[processor] handling 3 items
[processor] skip invalid item "report.pdf": suffix must be ".png"
[uploader] uploading avatar.png
[processor] skip invalid item "notes.txt": suffix must be ".png"
Uploader metrics: success=1 failed=2

切换 Handler 为 DryRun（无侵入替换）
[processor-dry] handling 3 items
[processor-dry] skip invalid item "report.pdf": suffix must be ".png"
[dry] dry-run avatar.png
[processor-dry] skip invalid item "notes.txt": suffix must be ".png"
DryRun metrics: success=1 failed=2
```

截图建议：

- 终端截图突出“替换 Handler”前后输出。
- 组件拼装图：Processor + Logger + Metrics + Handler/Validator。
- 方法提升示意：Processor 直接调用 Logger.Info / Metrics.MarkSuccess。

## 5. 常见坑 & 解决方案（必看）

1. **滥用嵌入导致命名冲突**：多个匿名字段同名方法/字段。解决：显式命名字段或通过限定名调用。
2. **状态串改**：把可变状态放在被嵌入的共享 struct 中，多个实例互相影响。解决：每个实例持有自己的状态（如独立 map、Metrics）。
3. **接口过大**：把多能力塞到一个接口里，替换困难。解决：拆成最小接口，调用方组合。
4. **假装继承**：希望“复写”父方法，结果只是覆盖字段。解决：用组合 + 策略注入，而非指望继承语义。
5. **方法提升误解**：以为嵌入就是继承。解决：记住只是语法糖，冲突需显式调用，且不支持多态覆盖。
6. **零值不可用**：嵌入指针未初始化导致 panic。解决：构造函数初始化必需字段，必要时提供零值可用策略。
7. **可测试性差**：Handler/Validator 硬编码具体类型。解决：用接口参数 + fake/闭包注入，便于单测。

配图建议：冲突示意图（两个匿名字段同名方法）、接口拆分表、状态隔离图。

## 6. 进阶扩展 / 思考题

- 给 Processor 增加中间件链（链式组合），支持前置/后置 hook。
- 为 Validator/Handler 写表驱动测试，覆盖合法/非法/错误分支。
- 添加指标导出接口，让 Metrics 可以被替换为 Prometheus 适配器。
- 用函数式选项配置 Processor（比如是否严格校验、日志前缀），体验组合 + 配置模式。
- 扩展 DryRunHandler：记录模拟写入的条目，比较与真实上传的差异。

配图建议：中间件链时序图；函数式选项调用示意；真实/演练 handler 对比表。

---

组合让能力拼装清晰、状态各自独立，接口拆分让替换和测试变得简单。与继承相比，Go 的嵌入只是语法糖，没有隐式“父类契约”，这迫使你显式设计边界与协作点。跑一遍示例，再检查你的项目：哪些地方可以拆成最小接口？哪些共享状态应该移回各自实例？下一篇我们会进入指针与逃逸分析，继续完善 Go 思维。 
