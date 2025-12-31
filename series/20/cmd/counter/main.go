package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type config struct {
	mode      string
	workers   int
	loops     int
	showEvery int
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== WaitGroup 与 Mutex：共享数据的正确方式 ===")
	fmt.Printf("mode=%s workers=%d loops=%d GOMAXPROCS=%d\n",
		cfg.mode, cfg.workers, cfg.loops, runtime.GOMAXPROCS(0))

	start := time.Now()
	var got int
	var expected int

	switch cfg.mode {
	case "unsafe":
		got, expected = runUnsafe(cfg)
	case "mutex":
		got, expected = runWithMutex(cfg)
	default:
		fmt.Printf("unknown -mode=%q (use unsafe|mutex)\n", cfg.mode)
		return
	}

	fmt.Printf("expected=%d got=%d cost=%s\n", expected, got, time.Since(start))
	if got != expected {
		fmt.Println("WARNING: result mismatch (likely data race). Try: go run -race ./series/20/cmd/counter -mode=unsafe")
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", "mutex", "unsafe|mutex")
	flag.IntVar(&cfg.workers, "workers", 40, "number of goroutines")
	flag.IntVar(&cfg.loops, "loops", 50_000, "increments per goroutine")
	flag.IntVar(&cfg.showEvery, "show-every", 0, "print progress every N loops (0 to disable)")
	flag.Parse()

	if cfg.workers < 1 {
		cfg.workers = 1
	}
	if cfg.loops < 0 {
		cfg.loops = 0
	}
	return cfg
}

func runUnsafe(cfg config) (got int, expected int) {
	fmt.Println("\n--- unsafe: no mutex ---")
	expected = cfg.workers * cfg.loops

	counter := 0
	var wg sync.WaitGroup
	wg.Add(cfg.workers)

	for w := 0; w < cfg.workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < cfg.loops; i++ {
				counter++
				if cfg.showEvery > 0 && i%cfg.showEvery == 0 && w == 0 {
					fmt.Printf("  progress i=%d counter=%d\n", i, counter)
				}
			}
		}()
	}

	wg.Wait()
	return counter, expected
}

func runWithMutex(cfg config) (got int, expected int) {
	fmt.Println("\n--- mutex: protect shared counter ---")
	expected = cfg.workers * cfg.loops

	counter := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(cfg.workers)

	for w := 0; w < cfg.workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < cfg.loops; i++ {
				mu.Lock()
				counter++
				if cfg.showEvery > 0 && i%cfg.showEvery == 0 && w == 0 {
					fmt.Printf("  progress i=%d counter=%d\n", i, counter)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return counter, expected
}
