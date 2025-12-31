package format

import (
	"strconv"
	"testing"
)

var sink string

func BenchmarkBuild(b *testing.B) {
	small := makeWords(32, 6)
	large := makeWords(1024, 6)

	b.ReportAllocs()

	bench := func(name string, words []string, fn func([]string) string) {
		b.Run(name, func(b *testing.B) {
			var out string
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out = fn(words)
			}
			sink = out
		})
	}

	bench("plus/small", small, BuildPlus)
	bench("plus/large", large, BuildPlus)
	bench("builder/small", small, BuildBuilder)
	bench("builder/large", large, BuildBuilder)
	bench("join/small", small, BuildJoin)
	bench("join/large", large, BuildJoin)
}

func makeWords(n, size int) []string {
	words := make([]string, n)
	for i := 0; i < n; i++ {
		words[i] = "w" + strconv.Itoa(i%10) + repeat("x", size)
	}
	return words
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = s[0]
	}
	return string(out)
}
