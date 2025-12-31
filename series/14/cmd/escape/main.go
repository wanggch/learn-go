package main

import (
	"fmt"
	"math/rand"
)

type Record struct {
	ID   int
	Data [512]byte
	Meta string
}

// escapes: returns pointer to local
func newRecord(id int) *Record {
	r := Record{ID: id, Meta: "stack?"}
	return &r
}

// no escape if compiler can inline and keep on stack (small struct)
func sum(a, b int) int {
	return a + b
}

// might escape: append causes backing array to live beyond scope
func buildSlice(n int) []int {
	out := []int{}
	for i := 0; i < n; i++ {
		out = append(out, i)
	}
	return out
}

// escape via interface: storing pointer in interface may force escape
func storeInInterface() any {
	s := "hello"
	return &s
}

// stay on stack: large array passed by pointer, not stored globally
func touchLarge(p *Record) {
	p.Meta = "touched"
}

func main() {
	fmt.Println("=== 逃逸分析演示 ===")

	r := newRecord(rand.Intn(1000))
	fmt.Printf("newRecord 返回地址：%p（可能逃逸到堆）\n", r)

	total := sum(3, 4)
	fmt.Printf("sum 结果：%d（通常栈上完成）\n", total)

	s := buildSlice(5)
	fmt.Printf("buildSlice 长度=%d cap=%d（底层数组逃逸以返回）\n", len(s), cap(s))

	i := storeInInterface()
	fmt.Printf("接口持有的类型=%T\n", i)

	var big Record
	touchLarge(&big)
	fmt.Printf("touchLarge 后 meta=%s（指针参数避免大拷贝）\n", big.Meta)

	fmt.Println("\n提示：运行 go build -gcflags=\"-m\" ./... 查看具体逃逸位置")
}
