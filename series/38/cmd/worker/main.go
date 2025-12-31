package main

import (
	"fmt"
	"time"

	"learn-go/series/38/internal/config"
	"learn-go/series/38/internal/report"
)

func main() {
	cfg := config.Load()
	cfg.Mode = "worker"

	start := time.Now()
	handled, failed := runJobs(cfg.Workers, 80)

	summary := report.Summary(report.Snapshot{
		Config:  cfg,
		Handled: handled,
		Failed:  failed,
		Elapsed: time.Since(start),
	})

	fmt.Println("worker summary:")
	fmt.Println(summary)
}

func runJobs(workers, total int) (int, int) {
	failed := 0
	for i := 0; i < total; i++ {
		if i%17 == 0 {
			failed++
		}
		time.Sleep(2 * time.Millisecond)
		_ = workers
	}
	return total, failed
}
