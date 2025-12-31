package format

import "strings"

func BuildPlus(words []string) string {
	var s string
	for _, w := range words {
		s += w
	}
	return s
}

func BuildBuilder(words []string) string {
	var b strings.Builder
	need := 0
	for _, w := range words {
		need += len(w)
	}
	b.Grow(need)
	for _, w := range words {
		b.WriteString(w)
	}
	return b.String()
}

func BuildJoin(words []string) string {
	return strings.Join(words, "")
}
