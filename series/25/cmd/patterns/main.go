package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type config struct {
	workers int
	items   int
	timeout time.Duration
}

type job struct {
	id   int
	data string
}

type result struct {
	id     int
	hash   string
	cost   time.Duration
	err    error
	worker int
}

func main() {
	cfg := parseFlags()
	fmt.Println("=== 并发模式：worker pool + fan-out/fan-in（演示） ===")
	fmt.Printf("GOMAXPROCS=%d workers=%d items=%d timeout=%s\n",
		runtime.GOMAXPROCS(0), cfg.workers, cfg.items, cfg.timeout)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	jobs := make(chan job)
	results := make(chan result)

	var wg sync.WaitGroup
	wg.Add(cfg.workers)
	for w := 1; w <= cfg.workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			worker(ctx, w, jobs, results)
		}()
	}

	go func() {
		defer close(jobs)
		for i := 0; i < cfg.items; i++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{id: i, data: fmt.Sprintf("payload-%d", i)}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	seen := 0
	var total time.Duration
	for r := range results {
		seen++
		if r.err != nil {
			fmt.Printf("result id=%d worker=%d err=%v\n", r.id, r.worker, r.err)
			continue
		}
		total += r.cost
		if r.id%20 == 0 {
			fmt.Printf("result id=%d worker=%d hash=%s cost=%s\n", r.id, r.worker, r.hash[:8], r.cost)
		}
	}

	fmt.Printf("done: results=%d avg_cost=%s ctx_err=%v\n", seen, avg(total, seen), ctx.Err())
}

func parseFlags() config {
	var cfg config
	flag.IntVar(&cfg.workers, "workers", 8, "worker pool size")
	flag.IntVar(&cfg.items, "items", 120, "jobs to produce")
	flag.DurationVar(&cfg.timeout, "timeout", 220*time.Millisecond, "overall timeout")
	flag.Parse()

	if cfg.workers < 1 {
		cfg.workers = 1
	}
	if cfg.items < 0 {
		cfg.items = 0
	}
	if cfg.timeout <= 0 {
		cfg.timeout = 220 * time.Millisecond
	}
	return cfg
}

func worker(ctx context.Context, workerID int, jobs <-chan job, results chan<- result) {
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-jobs:
			if !ok {
				return
			}

			start := time.Now()
			h, err := doHashWork(ctx, j.data)
			res := result{
				id:     j.id,
				hash:   h,
				cost:   time.Since(start),
				err:    err,
				worker: workerID,
			}

			select {
			case <-ctx.Done():
				return
			case results <- res:
			}
		}
	}
}

func doHashWork(ctx context.Context, s string) (string, error) {
	// Simulate CPU work + cancellation point.
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(6 * time.Millisecond):
	}

	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:]), nil
}

func avg(total time.Duration, n int) time.Duration {
	if n <= 0 {
		return 0
	}
	return total / time.Duration(n)
}
