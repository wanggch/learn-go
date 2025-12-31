package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	workers int
	loops   int
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== atomic vs mutex：使用边界演示 ===")
	fmt.Printf("workers=%d loops=%d GOMAXPROCS=%d\n",
		cfg.workers, cfg.loops, runtime.GOMAXPROCS(0))

	runCounterAtomic(cfg)
	runCounterMutex(cfg)
	runStatsAtomic(cfg)
}

func parseFlags() config {
	var cfg config
	flag.IntVar(&cfg.workers, "workers", 40, "number of goroutines")
	flag.IntVar(&cfg.loops, "loops", 50_000, "increments per goroutine")
	flag.Parse()
	if cfg.workers < 1 {
		cfg.workers = 1
	}
	if cfg.loops < 0 {
		cfg.loops = 0
	}
	return cfg
}

func runCounterAtomic(cfg config) {
	fmt.Println("\n--- atomic counter ---")
	var counter int64
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(cfg.workers)
	for i := 0; i < cfg.workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < cfg.loops; j++ {
				atomic.AddInt64(&counter, 1)
			}
		}()
	}
	wg.Wait()
	fmt.Printf("counter=%d expected=%d cost=%s\n", counter, int64(cfg.workers*cfg.loops), time.Since(start))
}

func runCounterMutex(cfg config) {
	fmt.Println("\n--- mutex counter ---")
	var counter int64
	var mu sync.Mutex
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(cfg.workers)
	for i := 0; i < cfg.workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < cfg.loops; j++ {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Printf("counter=%d expected=%d cost=%s\n", counter, int64(cfg.workers*cfg.loops), time.Since(start))
}

type stats struct {
	requests int64
	errors   int64
	latency  int64
}

func runStatsAtomic(cfg config) {
	fmt.Println("\n--- atomic stats (multi-field boundary) ---")
	var s stats
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(cfg.workers)
	for i := 0; i < cfg.workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < cfg.loops; j++ {
				atomic.AddInt64(&s.requests, 1)
				if j%10 == 0 {
					atomic.AddInt64(&s.errors, 1)
				}
				atomic.AddInt64(&s.latency, int64(j%7))
			}
		}()
	}
	wg.Wait()

	req := atomic.LoadInt64(&s.requests)
	errs := atomic.LoadInt64(&s.errors)
	lat := atomic.LoadInt64(&s.latency)
	fmt.Printf("requests=%d errors=%d latency_sum=%d cost=%s\n", req, errs, lat, time.Since(start))
	fmt.Println("注意：多字段统计没有“原子快照”，若需要一致性应使用锁或拷贝策略。")
}
