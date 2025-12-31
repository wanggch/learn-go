package main

import (
	"fmt"
	"sort"
	"strings"
)

type product struct {
	Name     string
	Quantity int
}

type bucket struct {
	tag   string
	items []string
}

func main() {
	fmt.Println("=== slice 和 map 的真实用法演示 ===")
	demoNilMapAssign()
	demoSliceAliasing()
	demoAppendCapacity()
	demoMapIterationOrder()
	demoMapWithSliceValue()
}

func demoNilMapAssign() {
	printTitle("1) nil map 赋值会 panic，必须 make")

	var stock map[string]int
	fmt.Printf("初始 stock == nil ? %v\n", stock == nil)
	fmt.Println("尝试写入会 panic，防止线上踩坑请先 make：")

	safeStock := make(map[string]int, 4)
	safeStock["apple"] = 10
	safeStock["banana"] = 6
	fmt.Printf("安全写入后：%v\n", safeStock)
}

func demoSliceAliasing() {
	printTitle("2) 子切片共享底层数组，修改会互相影响")

	base := []string{"A", "B", "C", "D"}
	window := base[:2:3] // len=2 cap=3，共享底层且限制 cap

	fmt.Printf("原始 base: %v\n", base)
	window[0] = "a"
	fmt.Printf("改 window[0]=a 后 base: %v (被修改)\n", base)

	window = append(window, "X") // 仍在原底层上写入
	fmt.Printf("append 1 次后 base: %v\n", base)

	window = append(window, "Y") // 触发扩容，window 分离
	window[1] = "b"
	fmt.Printf("二次 append 后 base: %v (未再受影响)\n", base)
	fmt.Printf("window 独立内容: %v\n", window)
}

func demoAppendCapacity() {
	printTitle("3) 预分配能减少扩容，避免多余拷贝")

	items := []product{
		{Name: "A", Quantity: 8},
		{Name: "B", Quantity: 5},
		{Name: "C", Quantity: 7},
	}

	noCap := []product{}
	for _, p := range items {
		noCap = append(noCap, p)
		fmt.Printf("无预分配 -> len=%d cap=%d\n", len(noCap), cap(noCap))
	}

	withCap := make([]product, 0, len(items))
	for _, p := range items {
		withCap = append(withCap, p)
		fmt.Printf("有预分配  -> len=%d cap=%d\n", len(withCap), cap(withCap))
	}
}

func demoMapIterationOrder() {
	printTitle("4) map 遍历无序，排序 keys 再输出")

	views := map[string]int{
		"/api/orders":  210,
		"/api/pay":     180,
		"/api/profile": 90,
		"/api/login":   260,
	}

	fmt.Println("直接 range（顺序不保证）：")
	for path, cnt := range views {
		fmt.Printf("  %s -> %d\n", path, cnt)
	}

	fmt.Println("排序 keys 后输出（顺序稳定）：")
	keys := make([]string, 0, len(views))
	for k := range views {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s -> %d\n", k, views[k])
	}
}

func demoMapWithSliceValue() {
	printTitle("5) map[string][]T 时别复用同一切片底层")

	// 错误用法：复用缓冲，所有 key 共用同一底层
	buffer := []string{}
	shared := map[string][]string{}
	addShared := func(tag, item string) {
		buffer = append(buffer, item)
		shared[tag] = buffer
	}

	addShared("slow", "order-1")
	addShared("slow", "order-2")
	addShared("retry", "order-3") // 会让 slow 也看到 order-3
	fmt.Printf("共享底层 map：%v\n", shared)

	// 正确用法：为每个 key 拷贝一份，或重新 append 到 nil
	isolated := map[string][]string{}
	addIsolated := func(tag, item string) {
		s := isolated[tag]
		s = append(s, item) // append 到独立切片
		isolated[tag] = s
	}

	addIsolated("slow", "order-1")
	addIsolated("slow", "order-2")
	addIsolated("retry", "order-3")
	fmt.Printf("隔离底层 map：%v\n", isolated)

	fmt.Println("打印更友好的格式：")
	for _, b := range bucketsFromMap(isolated) {
		fmt.Printf("  %s -> %s\n", b.tag, strings.Join(b.items, ", "))
	}
}

func bucketsFromMap(m map[string][]string) []bucket {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]bucket, 0, len(m))
	for _, k := range keys {
		out = append(out, bucket{tag: k, items: append([]string(nil), m[k]...)})
	}
	return out
}

func printTitle(title string) {
	fmt.Printf("\n--- %s ---\n", title)
}
