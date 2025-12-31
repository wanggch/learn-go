package main

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type config struct {
	mode   string
	n      int
	tick   time.Duration
	linger time.Duration
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== goroutine 泄漏演示（blocked / ticker / send） ===")
	fmt.Printf("mode=%s n=%d tick=%s linger=%s GOMAXPROCS=%d\n",
		cfg.mode, cfg.n, cfg.tick, cfg.linger, runtime.GOMAXPROCS(0))
	printStats("start")

	switch cfg.mode {
	case "leak-recv":
		leakRecv(cfg.n, cfg.linger)
	case "fix-recv":
		fixRecv(cfg.n, cfg.linger)
	case "leak-send":
		leakSend(cfg.n, cfg.linger)
	case "fix-send":
		fixSend(cfg.n, cfg.linger)
	case "leak-ticker":
		leakTicker(cfg.tick, cfg.linger)
	case "fix-ticker":
		fixTicker(cfg.tick, cfg.linger)
	default:
		fmt.Println("modes: leak-recv|fix-recv|leak-send|fix-send|leak-ticker|fix-ticker")
		return
	}

	printStats("end")
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", "fix-recv", "leak-recv|fix-recv|leak-send|fix-send|leak-ticker|fix-ticker")
	flag.IntVar(&cfg.n, "n", 3000, "number of goroutines for recv/send demos")
	flag.DurationVar(&cfg.tick, "tick", 10*time.Millisecond, "ticker interval")
	flag.DurationVar(&cfg.linger, "linger", 120*time.Millisecond, "sleep duration to observe goroutines")
	flag.Parse()

	if cfg.n < 0 {
		cfg.n = 0
	}
	if cfg.tick <= 0 {
		cfg.tick = 10 * time.Millisecond
	}
	if cfg.linger <= 0 {
		cfg.linger = 120 * time.Millisecond
	}
	return cfg
}

func leakRecv(n int, linger time.Duration) {
	fmt.Println("\n--- leak-recv: goroutines blocked on receive (channel never closed) ---")
	ch := make(chan int)

	for i := 0; i < n; i++ {
		go func() {
			<-ch
		}()
	}

	time.Sleep(linger)
	printStats("after spawn (still blocked)")
}

func fixRecv(n int, linger time.Duration) {
	fmt.Println("\n--- fix-recv: receiver select with ctx.Done() ---")
	ch := make(chan int)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			select {
			case <-ch:
			case <-ctx.Done():
			}
		}()
	}

	time.Sleep(linger)
	printStats("after spawn")

	cancel()
	wg.Wait()
	printStats("after cancel + wait")
}

func leakSend(n int, linger time.Duration) {
	fmt.Println("\n--- leak-send: goroutines blocked on send (no receiver) ---")
	ch := make(chan int) // unbuffered

	for i := 0; i < n; i++ {
		i := i
		go func() {
			ch <- i
		}()
	}

	time.Sleep(linger)
	printStats("after spawn (still blocked)")
}

func fixSend(n int, linger time.Duration) {
	fmt.Println("\n--- fix-send: send with ctx timeout ---")
	ch := make(chan int) // still no receiver
	ctx, cancel := context.WithTimeout(context.Background(), linger/2)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			select {
			case ch <- i:
			case <-ctx.Done():
			}
		}()
	}

	time.Sleep(linger)
	wg.Wait()
	printStats("after timeout + wait")
}

func leakTicker(tick time.Duration, linger time.Duration) {
	fmt.Println("\n--- leak-ticker: ticker not stopped, goroutine never exits ---")
	ticker := time.NewTicker(tick)
	go func() {
		for range ticker.C {
			// do nothing
		}
	}()

	time.Sleep(linger)
	printStats("after linger (ticker goroutine still running)")
}

func fixTicker(tick time.Duration, linger time.Duration) {
	fmt.Println("\n--- fix-ticker: ctx + Stop + exit ---")
	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(tick)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	time.Sleep(linger)
	printStats("after spawn")

	cancel()
	wg.Wait()
	printStats("after cancel + wait")
}

func printStats(label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("[%s] goroutines=%d heap_alloc=%s heap_objects=%d num_gc=%d\n",
		label,
		runtime.NumGoroutine(),
		bytes(m.HeapAlloc),
		m.HeapObjects,
		m.NumGC,
	)
}

func bytes(n uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)
	switch {
	case n >= MB:
		return fmt.Sprintf("%.2fMB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.2fKB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
