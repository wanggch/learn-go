package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type config struct {
	mode        string
	n           int
	concurrency int
	sleep       time.Duration
	leak        int
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== goroutine：并发不是免费的午餐（演示） ===")
	fmt.Printf("GOMAXPROCS=%d\n", runtime.GOMAXPROCS(0))
	printStats("start")

	switch cfg.mode {
	case "seq":
		runSequential(cfg)
	case "unbounded":
		runUnbounded(cfg)
	case "bounded":
		runBounded(cfg)
	case "leak":
		demoLeak(cfg)
	default:
		fmt.Printf("unknown -mode=%q\n", cfg.mode)
	}

	printStats("end")
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", "bounded", "seq|unbounded|bounded|leak")
	flag.IntVar(&cfg.n, "n", 300, "number of tasks")
	flag.IntVar(&cfg.concurrency, "c", 20, "bounded concurrency")
	flag.DurationVar(&cfg.sleep, "sleep", 10*time.Millisecond, "simulated IO duration per task")
	flag.IntVar(&cfg.leak, "leak", 5000, "goroutines to park in leak mode")
	flag.Parse()

	if cfg.n < 0 {
		cfg.n = 0
	}
	if cfg.concurrency < 1 {
		cfg.concurrency = 1
	}
	if cfg.sleep < 0 {
		cfg.sleep = 0
	}
	if cfg.leak < 0 {
		cfg.leak = 0
	}
	return cfg
}

func runSequential(cfg config) {
	fmt.Printf("\n--- mode=seq n=%d sleep=%s ---\n", cfg.n, cfg.sleep)
	start := time.Now()
	for i := 0; i < cfg.n; i++ {
		doTask(i, cfg.sleep)
	}
	fmt.Printf("done in %s\n", time.Since(start))
}

func runUnbounded(cfg config) {
	fmt.Printf("\n--- mode=unbounded n=%d sleep=%s ---\n", cfg.n, cfg.sleep)
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(cfg.n)
	for i := 0; i < cfg.n; i++ {
		i := i
		go func() {
			defer wg.Done()
			doTask(i, cfg.sleep)
		}()
	}
	wg.Wait()
	fmt.Printf("done in %s\n", time.Since(start))
}

func runBounded(cfg config) {
	fmt.Printf("\n--- mode=bounded n=%d sleep=%s c=%d ---\n", cfg.n, cfg.sleep, cfg.concurrency)
	start := time.Now()

	sem := make(chan struct{}, cfg.concurrency)
	var wg sync.WaitGroup
	wg.Add(cfg.n)
	for i := 0; i < cfg.n; i++ {
		i := i
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			doTask(i, cfg.sleep)
		}()
	}
	wg.Wait()
	fmt.Printf("done in %s\n", time.Since(start))
}

func doTask(i int, sleep time.Duration) {
	if i%100 == 0 {
		// keep some observable output but not too noisy
		fmt.Printf("task #%d ...\n", i)
	}
	time.Sleep(sleep)
}

func demoLeak(cfg config) {
	fmt.Printf("\n--- mode=leak leak=%d ---\n", cfg.leak)

	block := make(chan struct{})
	for i := 0; i < cfg.leak; i++ {
		go func() {
			<-block
		}()
	}

	time.Sleep(80 * time.Millisecond)
	printStats("after spawn (parked)")

	close(block)
	time.Sleep(80 * time.Millisecond)
	printStats("after release")
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
