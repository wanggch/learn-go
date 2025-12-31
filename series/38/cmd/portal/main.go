package main

import (
	"fmt"
	"math/rand"
	"time"

	"learn-go/series/38/internal/config"
	"learn-go/series/38/internal/report"
)

func main() {
	cfg := config.Load()
	cfg.Mode = "portal"

	start := time.Now()
	handled, failed := simulateRequests(cfg.Workers, 120)

	summary := report.Summary(report.Snapshot{
		Config:  cfg,
		Handled: handled,
		Failed:  failed,
		Elapsed: time.Since(start),
	})

	fmt.Println("portal summary:")
	fmt.Println(summary)
}

func simulateRequests(workers, total int) (int, int) {
	rand.Seed(time.Now().UnixNano())
	failed := 0
	for i := 0; i < total; i++ {
		if rand.Intn(100) < 7 {
			failed++
		}
	}
	return total, failed
}
