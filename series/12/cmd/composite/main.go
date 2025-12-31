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
