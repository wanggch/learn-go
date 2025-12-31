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
		ID:        id,
		Title:     title,
		Status:    "new",
		Tags:      []string{"triage"},
		User:      User{ID: "u-" + userName, Name: userName, Tier: tier},
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
	ticket.Touch("ops-bot")                         // method from EnrichedTicket
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
