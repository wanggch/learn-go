package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Consumer-defined interfaces (最小接口原则)
type TaskLister interface {
	ListPending() ([]Task, error)
}

type Notifier interface {
	Notify(user string, message string) error
}

type Task struct {
	ID        string
	Owner     string
	Title     string
	Status    string
	CreatedAt time.Time
	DueInDay  int
}

type MemoryStore struct {
	tasks []Task
}

func (m *MemoryStore) ListPending() ([]Task, error) {
	var out []Task
	for _, t := range m.tasks {
		if t.Status == "pending" {
			out = append(out, t)
		}
	}
	return out, nil
}

type WebhookNotifier struct {
	Endpoint string
}

func (n WebhookNotifier) Notify(user string, message string) error {
	if n.Endpoint == "" {
		return errors.New("missing endpoint")
	}
	fmt.Printf("[webhook] send to %s via %s:\n%s\n", user, n.Endpoint, message)
	return nil
}

// Adapter via function type.
type NotifyFunc func(user string, message string) error

func (f NotifyFunc) Notify(user string, message string) error {
	return f(user, message)
}

type ReportService struct {
	store    TaskLister
	notifier Notifier
	builder  Formatter
}

type Formatter interface {
	Format(tasks []Task) string
}

type PlainFormatter struct{}

func (PlainFormatter) Format(tasks []Task) string {
	if len(tasks) == 0 {
		return "今天没有待处理任务，保持轻松！"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "今日待处理任务（%d 个）：\n", len(tasks))
	for i, t := range tasks {
		fmt.Fprintf(&b, "%d) %s | owner=%s | due=%d 天 | created=%s\n",
			i+1, t.Title, t.Owner, t.DueInDay, t.CreatedAt.Format("2006-01-02"))
	}
	return b.String()
}

func NewReportService(store TaskLister, notifier Notifier, formatter Formatter) (*ReportService, error) {
	if store == nil || notifier == nil || formatter == nil {
		return nil, errors.New("store/notifier/formatter 不能为空")
	}
	return &ReportService{store: store, notifier: notifier, builder: formatter}, nil
}

func (s *ReportService) SendDaily(user string) error {
	tasks, err := s.store.ListPending()
	if err != nil {
		return fmt.Errorf("list pending: %w", err)
	}

	msg := s.builder.Format(tasks)
	if err := s.notifier.Notify(user, msg); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	return nil
}

func main() {
	fmt.Println("=== interface：隐式实现与最小接口演示 ===")

	store := &MemoryStore{
		tasks: []Task{
			{ID: "t-101", Owner: "alice", Title: "修复支付超时", Status: "pending", CreatedAt: time.Now().Add(-8 * time.Hour), DueInDay: 1},
			{ID: "t-102", Owner: "bob", Title: "下线老接口", Status: "done", CreatedAt: time.Now().Add(-72 * time.Hour), DueInDay: 0},
			{ID: "t-103", Owner: "alice", Title: "检查风控告警", Status: "pending", CreatedAt: time.Now().Add(-4 * time.Hour), DueInDay: 2},
		},
	}

	webhook := WebhookNotifier{Endpoint: "https://notify.example.com/hooks/abc"}
	console := NotifyFunc(func(user, message string) error {
		fmt.Printf("[console] to %s:\n%s\n", user, message)
		return nil
	})

	service, err := NewReportService(store, webhook, PlainFormatter{})
	if err != nil {
		panic(err)
	}
	fmt.Println("\n使用 WebhookNotifier：")
	if err := service.SendDaily("pm-lisa"); err != nil {
		fmt.Println("send failed:", err)
	}

	fmt.Println("\n切换到 NotifyFunc 适配的控制台输出：")
	service.notifier = console
	if err := service.SendDaily("pm-lisa"); err != nil {
		fmt.Println("send failed:", err)
	}
}
