package main

import (
	"errors"
	"fmt"
)

// Service is the consumer-facing interface.
type Service interface {
	Do() error
}

// Implementation that might be nil (e.g., optional plugin).
type OptionalClient struct {
	Name string
}

func (c *OptionalClient) Do() error {
	if c == nil {
		return errors.New("OptionalClient is nil")
	}
	fmt.Printf("OptionalClient %s executing...\n", c.Name)
	return nil
}

// Factory returns an interface, but beware of typed-nil.
func NewClient(enabled bool) Service {
	if !enabled {
		return (*OptionalClient)(nil) // typed nil: interface value is non-nil
	}
	return &OptionalClient{Name: "primary"}
}

// Safe wrapper returning (Service, error) to avoid typed nil surprises.
func NewClientSafe(enabled bool) (Service, error) {
	if !enabled {
		return nil, nil
	}
	return &OptionalClient{Name: "primary"}, nil
}

type Processor struct {
	Client Service
}

func (p Processor) Run() error {
	if p.Client == nil {
		return errors.New("client is nil")
	}
	return p.Client.Do()
}

// Nil in concrete type but non-nil in interface scenario.
func demoTypedNil() {
	fmt.Println("\n--- 场景 1：typed nil 被当成非 nil ---")
	client := NewClient(false)
	fmt.Printf("client == nil ? %v\n", client == nil)

	err := Processor{Client: client}.Run()
	fmt.Printf("Run 结果: err=%v\n", err)
}

// Use nil check on concrete type to avoid double nil.
func demoConcreteCheck() {
	fmt.Println("\n--- 场景 2：具体类型为 nil，接口非 nil ---")
	var c *OptionalClient = nil
	var s Service = c
	fmt.Printf("具体指针 nil? %v，接口 nil? %v\n", c == nil, s == nil)
	err := Processor{Client: s}.Run()
	fmt.Printf("调用结果 err=%v\n", err)
}

// Provide safe factory to avoid typed nil.
func demoSafeFactory() {
	fmt.Println("\n--- 场景 3：安全工厂返回 (Service, error) ---")
	s, err := NewClientSafe(false)
	fmt.Printf("工厂返回 nil? %v，err=%v\n", s == nil, err)
	if err := (Processor{Client: s}).Run(); err != nil {
		fmt.Printf("Run 结果 err=%v\n", err)
	}

	s, err = NewClientSafe(true)
	fmt.Printf("工厂返回 nil? %v，err=%v\n", s == nil, err)
	if err := (Processor{Client: s}).Run(); err != nil {
		fmt.Printf("Run 结果 err=%v\n", err)
	}
}

// Defensive method avoiding panic when receiver is nil.
func demoNilReceiverMethod() {
	fmt.Println("\n--- 场景 4：方法内部防御 nil 接收者 ---")
	var c *OptionalClient
	err := c.Do()
	fmt.Printf("nil 接收者调用 Do，err=%v\n", err)
}

// Interface variable holding nil concrete.
func demoInterfaceAssertion() {
	fmt.Println("\n--- 场景 5：类型断言时的 nil ---")
	var svc Service
	if _, ok := svc.(*OptionalClient); !ok {
		fmt.Println("svc 为空接口，断言失败 ok=false")
	}

	var c *OptionalClient
	svc = c
	if oc, ok := svc.(*OptionalClient); ok {
		fmt.Printf("断言成功，oc==nil? %v\n", oc == nil)
	}
}

func main() {
	fmt.Println("=== interface + nil 场景演示 ===")
	demoTypedNil()
	demoConcreteCheck()
	demoSafeFactory()
	demoNilReceiverMethod()
	demoInterfaceAssertion()
}
