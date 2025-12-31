package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type config struct {
	lines int
	size  int
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== bufio：缓冲带来的差异 ===")
	fmt.Printf("lines=%d size=%d bytes\n", cfg.lines, cfg.size)

	tmpDir := filepath.Join("series", "28", "tmp")
	_ = os.MkdirAll(tmpDir, 0o755)
	path := filepath.Join(tmpDir, "data.txt")

	if err := writeFile(path, cfg.lines, cfg.size); err != nil {
		fmt.Println("write error:", err)
		return
	}

	section("1) 逐行读取：bufio.Scanner", func() {
		n, cost, err := readWithScanner(path)
		fmt.Printf("lines=%d cost=%s err=%v\n", n, cost, err)
	})

	section("2) 直接 ReadAll：io.ReadAll", func() {
		n, cost, err := readAll(path)
		fmt.Printf("bytes=%d cost=%s err=%v\n", n, cost, err)
	})

	section("3) 小块读取：io.ReadAtLeast (no buffer)", func() {
		n, cost, err := readSmallChunks(path, 64)
		fmt.Printf("bytes=%d cost=%s err=%v\n", n, cost, err)
	})

	section("4) bufio.Reader：小块读取 + 缓冲", func() {
		n, cost, err := readBuffered(path, 64, 32*1024)
		fmt.Printf("bytes=%d cost=%s err=%v\n", n, cost, err)
	})
}

func parseFlags() config {
	var cfg config
	flag.IntVar(&cfg.lines, "lines", 2000, "lines to generate")
	flag.IntVar(&cfg.size, "size", 80, "bytes per line")
	flag.Parse()
	if cfg.lines < 1 {
		cfg.lines = 1
	}
	if cfg.size < 1 {
		cfg.size = 1
	}
	return cfg
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func writeFile(path string, lines int, size int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 32*1024)
	line := bytes.Repeat([]byte("x"), size-1)
	line = append(line, '\n')

	for i := 0; i < lines; i++ {
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return w.Flush()
}

func readWithScanner(path string) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, time.Since(start), scanner.Err()
}

func readAll(path string) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	b, err := io.ReadAll(f)
	if err != nil {
		return 0, 0, err
	}
	return len(b), time.Since(start), nil
}

func readSmallChunks(path string, chunk int) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	buf := make([]byte, chunk)
	total := 0
	for {
		n, err := f.Read(buf)
		total += n
		if err != nil {
			if err == io.EOF {
				break
			}
			return total, time.Since(start), err
		}
	}
	return total, time.Since(start), nil
}

func readBuffered(path string, chunk int, bufSize int) (int, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	start := time.Now()
	br := bufio.NewReaderSize(f, bufSize)
	buf := make([]byte, chunk)
	total := 0
	for {
		n, err := br.Read(buf)
		total += n
		if err != nil {
			if err == io.EOF {
				break
			}
			return total, time.Since(start), err
		}
	}
	return total, time.Since(start), nil
}
