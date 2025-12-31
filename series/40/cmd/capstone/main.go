package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var errJobFailed = errors.New("job failed")

type Config struct {
	Jobs    int
	Workers int
	Timeout time.Duration
}

type Summary struct {
	Jobs     int
	Handled  int
	Failed   int
	Canceled int
	Elapsed  time.Duration
}

type result struct {
	id   int
	err  error
	cost time.Duration
}

type Logger struct {
	service string
	logger  *log.Logger
}

type Field struct {
	Key   string
	Value string
}

func main() {
	cfg := parseFlags()
	if err := validateConfig(cfg); err != nil {
		log.Fatal(err)
	}

	logger := NewLogger("capstone")
	runID := traceID()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	logger.Info("run start",
		Str("run", runID),
		Int("jobs", cfg.Jobs),
		Int("workers", cfg.Workers),
		Duration("timeout", cfg.Timeout),
	)

	summary := run(ctx, cfg, logger, runID)

	logger.Info("run summary",
		Str("run", runID),
		Int("handled", summary.Handled),
		Int("failed", summary.Failed),
		Int("canceled", summary.Canceled),
		Duration("cost", summary.Elapsed),
	)
}

func parseFlags() Config {
	cfg := Config{
		Jobs:    9,
		Workers: 3,
		Timeout: 500 * time.Millisecond,
	}

	flag.IntVar(&cfg.Jobs, "jobs", cfg.Jobs, "number of jobs")
	flag.IntVar(&cfg.Workers, "workers", cfg.Workers, "number of workers")
	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "run timeout")
	flag.Parse()

	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.Jobs <= 0 {
		return errors.New("jobs must be positive")
	}
	if cfg.Workers <= 0 {
		return errors.New("workers must be positive")
	}
	if cfg.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	return nil
}

func run(ctx context.Context, cfg Config, logger *Logger, runID string) Summary {
	start := time.Now()
	jobs := make(chan int)
	results := make(chan result)

	var wg sync.WaitGroup
	for i := 1; i <= cfg.Workers; i++ {
		wg.Add(1)
		go worker(ctx, i, jobs, results, logger, runID, &wg)
	}

	go func() {
		defer close(jobs)
		for i := 1; i <= cfg.Jobs; i++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	summary := Summary{Jobs: cfg.Jobs}
	for res := range results {
		summary.Handled++
		if res.err != nil {
			if errors.Is(res.err, context.DeadlineExceeded) || errors.Is(res.err, context.Canceled) {
				summary.Canceled++
			} else {
				summary.Failed++
			}
		}
	}
	summary.Elapsed = time.Since(start)
	return summary
}

func worker(ctx context.Context, id int, jobs <-chan int, results chan<- result, logger *Logger, runID string, wg *sync.WaitGroup) {
	defer wg.Done()
	for jobID := range jobs {
		jobStart := time.Now()
		err := processJob(ctx, jobID)
		cost := time.Since(jobStart)

		fields := []Field{
			Str("run", runID),
			Int("worker", id),
			Int("job", jobID),
			Duration("cost", cost),
		}

		switch {
		case err == nil:
			logger.Info("job done", fields...)
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			logger.Info("job canceled", fields...)
		default:
			logger.Error("job failed", append(fields, Err(err))...)
		}

		results <- result{id: jobID, err: err, cost: cost}
	}
}

func processJob(ctx context.Context, id int) error {
	delay := time.Duration(80+(id%5)*40) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	if id%9 == 0 {
		return fmt.Errorf("job %d: %w", id, errJobFailed)
	}
	return nil
}

func NewLogger(service string) *Logger {
	return &Logger{
		service: service,
		logger:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.emit("INFO", msg, fields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.emit("ERROR", msg, fields...)
}

func (l *Logger) emit(level, msg string, fields ...Field) {
	parts := []string{
		"level=" + level,
		"service=" + l.service,
		"msg=" + msg,
	}
	for _, f := range fields {
		parts = append(parts, f.String())
	}
	l.logger.Println(strings.Join(parts, " "))
}

func (f Field) String() string {
	return f.Key + "=" + f.Value
}

func Str(key, val string) Field {
	return Field{Key: key, Value: val}
}

func Int(key string, val int) Field {
	return Field{Key: key, Value: strconv.Itoa(val)}
}

func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Value: val.String()}
}

func Err(err error) Field {
	return Field{Key: "err", Value: err.Error()}
}

func traceID() string {
	return fmt.Sprintf("run-%d", time.Now().UnixNano())
}
