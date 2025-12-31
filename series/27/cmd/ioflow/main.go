package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type countingWriter struct {
	W io.Writer
	N int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.W.Write(p)
	c.N += int64(n)
	return n, err
}

func main() {
	fmt.Println("=== io.Reader / io.Writer demos ===")
	section("1) Copy string reader to buffer", demoCopy)
	section("2) TeeReader: copy + hash", demoTee)
	section("3) LimitReader: preview", demoLimit)
	section("4) MultiWriter: file + buffer", demoMultiWriter)
	section("5) Pipe: streaming producer/consumer", demoPipe)
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func demoCopy() {
	src := strings.NewReader("hello io.Reader and io.Writer\n")
	var buf bytes.Buffer
	n, err := io.Copy(&buf, src)
	fmt.Printf("copied=%d err=%v\n", n, err)
	fmt.Printf("buffer=%q\n", buf.String())
}

func demoTee() {
	src := strings.NewReader("log stream for hash\n")
	hasher := sha1.New()
	tee := io.TeeReader(src, hasher)

	var out bytes.Buffer
	_, _ = io.Copy(&out, tee)
	sum := hex.EncodeToString(hasher.Sum(nil))
	fmt.Printf("hash=%s output=%q\n", sum[:8], out.String())
}

func demoLimit() {
	src := strings.NewReader("this is a long line for preview\n")
	limited := io.LimitReader(src, 10)
	b, _ := io.ReadAll(limited)
	fmt.Printf("preview=%q\n", string(b))
}

func demoMultiWriter() {
	tmpDir := filepath.Join("series", "27", "tmp")
	_ = os.MkdirAll(tmpDir, 0o755)
	path := filepath.Join(tmpDir, "output.txt")

	f, err := os.Create(path)
	if err != nil {
		fmt.Println("create file error:", err)
		return
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := io.MultiWriter(&buf, f)
	cw := &countingWriter{W: mw}

	_, err = io.Copy(cw, strings.NewReader("write to file and buffer\n"))
	fmt.Printf("bytes=%d err=%v file=%s\n", cw.N, err, path)
	fmt.Printf("buffer=%q\n", buf.String())
}

func demoPipe() {
	pr, pw := io.Pipe()
	done := make(chan struct{})

	go func() {
		defer close(done)
		w := bufio.NewWriter(pw)
		for i := 1; i <= 3; i++ {
			_, _ = fmt.Fprintf(w, "line-%d\n", i)
			_ = w.Flush()
			time.Sleep(20 * time.Millisecond)
		}
		_ = pw.Close()
	}()

	reader := bufio.NewReader(pr)
	lines := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Println("read error:", err)
			}
			break
		}
		lines++
		fmt.Printf("recv=%q\n", strings.TrimSpace(line))
	}
	<-done
	fmt.Printf("lines=%d\n", lines)
}
