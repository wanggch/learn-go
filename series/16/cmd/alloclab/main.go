package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func main() {
	fmt.Println("=== 减少分配：写出对 GC 友好的 Go 代码 ===")
	fmt.Println("提示：allocs/op 使用 testing.AllocsPerRun 估算，数值随版本与机器略有差异。")

	runCase("string 拼接: +", func() {
		_ = concatPlus(200)
	})
	runCase("string 拼接: strings.Builder + Grow", func() {
		_ = concatBuilder(200)
	})
	runCase("数字拼接: fmt.Sprintf", func() {
		_ = numbersFmt(200)
	})
	runCase("数字拼接: strconv.AppendInt", func() {
		_ = numbersAppendInt(200)
	})
	runCase("slice 追加: 不预分配", func() {
		_ = appendNoPrealloc(10_000)
	})
	runCase("slice 追加: 预分配 cap", func() {
		_ = appendPrealloc(10_000)
	})
	runCase("bytes.Buffer: 每次 new", func() {
		_ = bufferNew(200)
	})
	runCase("bytes.Buffer: sync.Pool 复用", func() {
		_ = bufferPool(200)
	})
}

func runCase(name string, fn func()) {
	allocs := testing.AllocsPerRun(200, fn)
	start := time.Now()
	fn()
	cost := time.Since(start)
	fmt.Printf("%-30s | allocs/op=%.2f | time=%s\n", name, allocs, cost)
}

func concatPlus(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += "a"
	}
	return s
}

func concatBuilder(n int) string {
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		b.WriteByte('a')
	}
	return b.String()
}

func numbersFmt(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(fmt.Sprintf("%d,", i))
	}
	return b.String()
}

func numbersAppendInt(n int) string {
	buf := make([]byte, 0, n*4)
	for i := 0; i < n; i++ {
		buf = strconv.AppendInt(buf, int64(i), 10)
		buf = append(buf, ',')
	}
	return string(buf)
}

func appendNoPrealloc(n int) []int {
	var out []int
	for i := 0; i < n; i++ {
		out = append(out, i)
	}
	return out
}

func appendPrealloc(n int) []int {
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, i)
	}
	return out
}

func bufferNew(n int) []byte {
	var b bytes.Buffer
	b.Grow(n * 8)
	tmp := make([]byte, 0, 32)
	for i := 0; i < n; i++ {
		b.WriteString("item=")
		tmp = tmp[:0]
		tmp = strconv.AppendInt(tmp, int64(i), 10)
		b.Write(tmp)
		b.WriteByte('\n')
	}
	out := make([]byte, b.Len())
	copy(out, b.Bytes())
	return out
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func bufferPool(n int) []byte {
	b := bufPool.Get().(*bytes.Buffer)
	b.Reset()
	defer bufPool.Put(b)

	b.Grow(n * 8)
	tmp := make([]byte, 0, 32)
	for i := 0; i < n; i++ {
		b.WriteString("item=")
		tmp = tmp[:0]
		tmp = strconv.AppendInt(tmp, int64(i), 10)
		b.Write(tmp)
		b.WriteByte('\n')
	}

	out := make([]byte, b.Len())
	copy(out, b.Bytes())
	return out
}
