# struct 与方法：Go 的“面向对象”实践

大家好，我是汪小成。你是不是也踩过这样的坑：为了“复用”功能，给业务 struct 加了五个匿名字段，结果方法名冲突；或者把方法全写成值接收者，改状态时发现根本没生效。Go 里没有 class，但用 struct + 方法 + 组合照样能写出清晰的对象风格。本文围绕“值接收 vs 指针接收”“方法集与接口”“组合优于继承”展开，让你的代码既易读又好测。

## 目录

1. 环境准备 / 前提知识
2. 核心概念解释（概念 → 示例 → 设计原因）
3. 完整代码示例（可运行）
4. 运行效果 + 截图描述
5. 常见坑 & 解决方案
6. 进阶扩展 / 思考题

## 1. 环境准备 / 前提知识

### 1.1 必备版本与工具

- Go 1.22+（需要 `go work` 支持），命令：`go version`。
- Git + gofmt/go vet（Go 自带）；推荐 VS Code + Go 扩展或 Goland，保存自动格式化。

### 1.2 项目结构与运行

- 根目录 `go.work` 管理多模块；本篇目录：`series/08`。
- 示例入口：`series/08/cmd/ticket/main.go`。
- 运行：`go run ./series/08/cmd/ticket`；如沙盒限制缓存可用 `GOCACHE=$(pwd)/.cache/go-build go run ./series/08/cmd/ticket`。

### 1.3 前置知识

- 理解 struct 零值（字段为零值，指针/切片/map 为 nil）。
- 熟悉切片 append、map 取值 `v, ok`。
- 基本错误处理：`if err := ...; err != nil { return err }`。

配图建议：目录树截图突出 `series/08`；一张方法集示意图（值/指针接收者对接口适配的影响）。

## 2. 核心概念解释

### 2.1 struct：组合字段，零值可用

- Go 的 struct 只是字段集合，没有继承层级；零值可用是设计目标。
- 示例：`type Ticket struct { ID string; Tags []string }`，零值 Tags 是 nil，但 append 后可用。
- 设计原因：减少“必须调用构造函数”的强制性，允许小步迭代。

### 2.2 方法接收者：值 vs 指针

- **值接收者**：获得副本，修改不影响原值；适合只读方法（如 `String()`、`Summary()`）。
- **指针接收者**：修改原对象或避免拷贝；需要保持状态时使用。
- 设计原因：方法集基于接收者类型，区分读/写意图，避免无意修改。

### 2.3 方法集与接口

- 值类型方法集包含值接收者方法；指针类型方法集包含值 + 指针接收者方法。
- 只有实现接口的方法集完整时，类型才满足接口。
- 设计原因：让可变对象必须以指针实现接口，避免拷贝丢失修改。

### 2.4 组合优于继承：嵌入与方法提升

- `type EnrichedTicket struct { Ticket; Metrics *Metrics }`：匿名字段 = 组合 + 方法可提升。
- 方法名冲突时需显式调用 `et.Ticket.Method()`。
- 设计原因：Go 不支持继承链，鼓励按需求组合能力。

### 2.5 构造函数与零值策略

- 没有关键字 newclass，常用 `NewXxx(...) (*Xxx, error)`。
- 对外暴露的 struct 应尽量零值可用；必须初始化的资源（channel/map）在构造里完成。
- 设计原因：显式创建成本低，测试时可直接字面量或零值。

配图建议：表格对比值/指针接收者的修改效果；流程图展示接口匹配的“方法集决定权”。

## 3. 完整代码示例（可复制运行）

场景：做一个“工单处理演示器”，展示：

- 值接收者（只读视图）与指针接收者（修改状态）。
- 方法集如何影响接口实现。
- 嵌入 struct 的方法提升与冲突规避。
- 构造函数返回指针 + error。

入口：`series/08/cmd/ticket/main.go`。

```go
package main

import (
	"fmt"
	"sort"
	"strings"
)

type User struct {
	ID   string
	Name string
	Tier string
}

type AuditInfo struct {
	CreatedBy string
	UpdatedBy string
}

// Promote a method through embedding.
func (a AuditInfo) Label() string {
	return fmt.Sprintf("created_by=%s updated_by=%s", a.CreatedBy, a.UpdatedBy)
}

type Ticket struct {
	ID     string
	Title  string
	Status string
	Tags   []string
	User   User
	AuditInfo
}

type Metrics struct {
	success int
	failed  int
}

type EnrichedTicket struct {
	Ticket
	History []string
	Metrics *Metrics
}

// Value receiver: read-only view.
func (t Ticket) Summary() string {
	return fmt.Sprintf("%s (%s) [%s]", t.Title, t.User.Name, strings.Join(t.Tags, ","))
}

// Pointer receiver: mutates state, keeps shared slice intact.
func (t *Ticket) AddTag(tag string) bool {
	if tag == "" {
		return false
	}
	for _, existing := range t.Tags {
		if existing == tag {
			return false
		}
	}
	t.Tags = append(t.Tags, tag)
	return true
}

// Pointer receiver: stateful counter.
func (m *Metrics) MarkSuccess(note string, hist *[]string) {
	m.success++
	*hist = append(*hist, fmt.Sprintf("success: %s", note))
}

func (m *Metrics) MarkFailed(note string, hist *[]string) {
	m.failed++
	*hist = append(*hist, fmt.Sprintf("failed: %s", note))
}

// Value receiver: snapshot.
func (m Metrics) Snapshot() (int, int) {
	return m.success, m.failed
}

// Constructor pattern returning pointer + error.
func NewTicket(id, title, userName, tier string) (*Ticket, error) {
	if id == "" || title == "" || userName == "" {
		return nil, fmt.Errorf("id/title/user 不能为空")
	}
	return &Ticket{
		ID:     id,
		Title:  title,
		Status: "new",
		Tags:   []string{"triage"},
		User:   User{ID: "u-" + userName, Name: userName, Tier: tier},
		AuditInfo: AuditInfo{CreatedBy: userName, UpdatedBy: userName},
	}, nil
}

// Method promoted via embedding: EnrichedTicket can call Label directly.
func (e *EnrichedTicket) Touch(by string) {
	e.AuditInfo.UpdatedBy = by
	e.History = append(e.History, fmt.Sprintf("updated by %s", by))
}

func main() {
	fmt.Println("=== struct 与方法集演示 ===")

	tickets := buildSeedTickets()

	fmt.Println("\n1) 值接收者 vs 指针接收者")
	showValueVsPointer(tickets[0])

	fmt.Println("\n2) 嵌入 + 方法提升")
	showEmbedding(tickets[1])

	fmt.Println("\n3) 方法集与接口适配")
	runPipeline(tickets[2])

	fmt.Println("\n4) 构造函数 + 零值可用")
	showConstructor()
}

func buildSeedTickets() []EnrichedTicket {
	raw := []struct {
		id    string
		title string
		user  string
		tier  string
	}{
		{"t-1001", "支付页面报错", "alice", "gold"},
		{"t-1002", "退款进度查询", "bob", "silver"},
		{"t-1003", "地址修改失败", "carol", "silver"},
	}

	var out []EnrichedTicket
	for _, r := range raw {
		t, _ := NewTicket(r.id, r.title, r.user, r.tier)
		out = append(out, EnrichedTicket{
			Ticket:  *t,
			History: []string{},
			Metrics: &Metrics{},
		})
	}
	return out
}

func showValueVsPointer(ticket EnrichedTicket) {
	fmt.Printf("原始 tags: %v\n", ticket.Tags)
	// Value receiver: does not mutate tags outside.
	fmt.Printf("Summary (value receiver): %s\n", ticket.Summary())

	// Pointer receiver mutates.
	ticket.AddTag("urgent")
	fmt.Printf("AddTag 后 tags: %v\n", ticket.Tags)
}

// Interface demonstrating method sets.
type Resolver interface {
	Resolve(t *EnrichedTicket) error
}

type TagResolver struct {
	Tag string
}

func (r TagResolver) Resolve(t *EnrichedTicket) error {
	if ok := t.AddTag(r.Tag); ok {
		t.History = append(t.History, "add tag "+r.Tag)
	}
	return nil
}

type CloseResolver struct{}

func (CloseResolver) Resolve(t *EnrichedTicket) error {
	t.Status = "closed"
	t.History = append(t.History, "status -> closed")
	return nil
}

func showEmbedding(ticket EnrichedTicket) {
	ticket.Touch("ops-bot") // method from EnrichedTicket
	fmt.Printf("Audit label: %s\n", ticket.Label()) // promoted from AuditInfo
	fmt.Printf("History: %v\n", ticket.History)
}

func runPipeline(ticket EnrichedTicket) {
	pipeline := []Resolver{
		TagResolver{Tag: "ops"},
		TagResolver{Tag: "mobile"},
		CloseResolver{},
	}

	for _, step := range pipeline {
		if err := step.Resolve(&ticket); err != nil {
			fmt.Printf("处理失败: %v\n", err)
			return
		}
	}

	ticket.Metrics.MarkSuccess("pipeline ok", &ticket.History)
	success, failed := ticket.Metrics.Snapshot()

	fmt.Printf("最终状态: %s\n", ticket.Status)
	fmt.Printf("标签: %v\n", sorted(ticket.Tags))
	fmt.Printf("历史: %v\n", ticket.History)
	fmt.Printf("指标: success=%d failed=%d\n", success, failed)
}

func sorted(ss []string) []string {
	cp := append([]string(nil), ss...)
	sort.Strings(cp)
	return cp
}

func showConstructor() {
	fmt.Println("尝试创建缺少字段的 Ticket：")
	if _, err := NewTicket("", "标题", "user1", "bronze"); err != nil {
		fmt.Printf("  创建失败: %v\n", err)
	}

	t, err := NewTicket("t-2001", "通道切换", "dave", "bronze")
	if err != nil {
		fmt.Printf("  创建失败: %v\n", err)
		return
	}

	// Demonstrate zero-value usable Metrics and History.
	enriched := EnrichedTicket{
		Ticket: *t,
		Metrics: &Metrics{
			success: 0,
			failed:  0,
		},
	}
	enriched.History = append(enriched.History, "created via constructor")
	enriched.Metrics.MarkFailed("need confirmation", &enriched.History)

	success, failed := enriched.Metrics.Snapshot()
	fmt.Printf("创建成功：%s，状态=%s，history=%v，指标 s=%d f=%d\n",
		enriched.Title, enriched.Status, enriched.History, success, failed)
}
```

代码中的看点：

- 值接收者 `Summary` 只读；指针接收者 `AddTag` 修改原切片。
- 方法集：接口 `Resolver` 需要指针接收者方法，传入 `*EnrichedTicket` 保证可变。
- 嵌入：`AuditInfo.Label` 被提升，`EnrichedTicket` 直接调用；`Touch` 修改更新人并记录历史。
- 构造函数：`NewTicket` 返回 `*Ticket, error`，同时保证默认 tags、审计信息。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/08/cmd/ticket
```

节选输出：

```
=== struct 与方法集演示 ===

1) 值接收者 vs 指针接收者
原始 tags: [triage]
Summary (value receiver): 支付页面报错 (alice) [triage]
AddTag 后 tags: [triage urgent]

2) 嵌入 + 方法提升
Audit label: created_by=bob updated_by=ops-bot
History: [updated by ops-bot]

3) 方法集与接口适配
最终状态: closed
标签: [mobile ops triage]
历史: [add tag ops add tag mobile status -> closed success: pipeline ok]
指标: success=1 failed=0

4) 构造函数 + 零值可用
尝试创建缺少字段的 Ticket：
  创建失败: id/title/user 不能为空
创建成功：通道切换，状态=new，history=[created via constructor failed: need confirmation], 指标 s=0 f=1
```

截图建议：

- 终端输出截图，标注“值/指针接收者”部分的标签变化。
- 方法集示意图：`Ticket`、`*Ticket` 各自方法集，以及接口 `Resolver` 需要哪些。
- 嵌入示意图：`EnrichedTicket` 内含 `Ticket` 与 `Metrics`，突出方法提升路径。

## 5. 常见坑 & 解决方案（必看）

1. **值接收者修改无效**：写了 `func (t Ticket) AddTag(...)`，调用后状态没变。解决：需要修改状态的用指针接收者。
2. **方法集不匹配接口**：值类型实现了 `Resolve` 但签名是 `(*T) Resolve`，把值传给接口时报错。解决：接口需要指针方法时，使用 `*T` 实例。
3. **嵌入字段同名冲突**：多个匿名字段有同名方法/字段，调用变模糊。解决：显式命名字段或通过 `obj.Field.Method()` 调用。
4. **构造函数缺少必填校验**：零值可用不代表跳过校验。解决：必须字段在 `NewXxx` 检查，返回 `nil, error`。
5. **拷贝大 struct 频繁**：值接收者在大 struct 上拷贝成本高。解决：为大对象统一使用指针接收者；或拆成小 struct。
6. **指针接收者并发安全缺失**：多 goroutine 调用指针方法修改同一实例。解决：并发前复制，或在方法内加锁/用不可变数据结构。
7. **方法里直接暴露内部切片**：返回内部切片指针，外部 append 破坏封装。解决：返回副本，如 `append([]T(nil), s...)`。

配图建议：一张“值/指针接收者选择指南”对照表；一张方法集与接口匹配矩阵。

## 6. 进阶扩展 / 思考题

- 为 `Resolver` 管道写表驱动测试，验证标签追加、状态关闭、历史记录顺序。
- 给 `Metrics` 加锁，或改成原子计数，比较代码复杂度与性能。
- 增加 `DeadlineResolver`，演示在方法内组合 `context.Context` 控制超时。
- 设计一个包装 struct，组合多个小能力（告警、审计、指标），体验组合的可维护性。
- 把 `NewTicket` 改成可选参数配置（函数式选项），思考可读性变化。
- 写 `String()` 方法（值接收者）和 `Clone()` 方法（返回深拷贝），比较两者的用途。

配图建议：表驱动测试用例表；函数式选项调用示例的时序图。

---

struct 与方法集是 Go “面向对象”的核心积木。区分好值/指针接收者、用组合而非继承、在方法签名里表达接口契合度，就能写出清晰、可维护的业务对象。跑一遍示例，再回头看看你的项目：哪些方法该改成指针接收者？哪些匿名字段可以变成显式组合？下一篇我们会聊 interface 的隐式实现与抽象边界，继续把“契约”写清楚。 
