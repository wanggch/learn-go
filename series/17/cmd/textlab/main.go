package main

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

func main() {
	fmt.Println("=== string / []byte / rune：编码与性能演示 ===")

	section("1) len vs rune 数", demoLenAndRuneCount)
	section("2) range 遍历 string 的真相", demoRangeOverString)
	section("3) 按字节切片会发生什么", demoByteSlicingPitfall)
	section("4) 安全截断：按 rune 或按字节边界", demoSafeTruncate)
	section("5) []byte 与 string 的拷贝与场景选择", demoBytesAndString)
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func demoLenAndRuneCount() {
	s1 := "hello"
	s2 := "你好"
	s3 := "Go😊"

	fmt.Printf("%q: len=%d bytes, rune=%d\n", s1, len(s1), utf8.RuneCountInString(s1))
	fmt.Printf("%q: len=%d bytes, rune=%d\n", s2, len(s2), utf8.RuneCountInString(s2))
	fmt.Printf("%q: len=%d bytes, rune=%d\n", s3, len(s3), utf8.RuneCountInString(s3))

	fmt.Println("结论：len(string) 是字节数，不是字符数。")
}

func demoRangeOverString() {
	s := "你a好😊"
	fmt.Printf("source: %q\n", s)
	fmt.Println("range 输出：index 是字节下标，r 是 rune（Unicode code point）")
	for i, r := range s {
		fmt.Printf("  i=%d r=%U char=%q\n", i, r, r)
	}
}

func demoByteSlicingPitfall() {
	s := "你好世界"
	fmt.Printf("source: %q len=%d\n", s, len(s))

	// Cut in the middle of a rune (UTF-8 code point is 3 bytes for Chinese here).
	bad := s[:4]
	fmt.Printf("bad slice s[:4]=%q (valid_utf8=%v)\n", bad, utf8.ValidString(bad))

	good := s[:6]
	fmt.Printf("good slice s[:6]=%q (valid_utf8=%v)\n", good, utf8.ValidString(good))
}

func demoSafeTruncate() {
	s := "Go 语言真香😊，但编码要小心"
	fmt.Printf("source: %q\n", s)

	fmt.Println("按 rune 截断（语义正确，但可能有额外分配）：")
	fmt.Printf("  truncateRunes(8) -> %q\n", truncateRunes(s, 8))

	fmt.Println("按 UTF-8 边界截断（不切断 rune）：")
	fmt.Printf("  truncateUTF8Bytes(10 bytes) -> %q\n", truncateUTF8Bytes(s, 10))
	fmt.Printf("  truncateUTF8Bytes(13 bytes) -> %q\n", truncateUTF8Bytes(s, 13))
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	rs := []rune(s)
	return string(rs[:n])
}

func truncateUTF8Bytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	b := []byte(s)
	i := maxBytes
	for i > 0 && !utf8.FullRune(b[:i]) {
		i--
	}
	if i == 0 {
		return ""
	}
	// Ensure valid boundary.
	for !utf8.Valid(b[:i]) {
		i--
		if i == 0 {
			return ""
		}
	}
	return string(b[:i])
}

func demoBytesAndString() {
	s := "abc你好"
	b := []byte(s) // makes a copy
	fmt.Printf("string -> []byte: %q -> %v (len=%d)\n", s, b, len(b))

	b[0] = 'A'
	fmt.Printf("改 b[0]='A' 后：b=%v，string 仍是 %q\n", b, s)

	s2 := string(b) // makes a copy
	fmt.Printf("[]byte -> string: %v -> %q\n", b, s2)

	fmt.Println("bytes.Buffer/Builder 的典型用途：拼接时减少中间对象")
	var buf bytes.Buffer
	buf.Grow(32)
	buf.WriteString("id=")
	buf.WriteString("1001")
	buf.WriteString(" msg=")
	buf.WriteString(s)
	fmt.Printf("buffer -> %q\n", buf.String())
}
