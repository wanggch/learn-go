package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"runtime"
	"time"
)

type ctxKey string

const (
	keyRequestID ctxKey = "request_id"
)

type config struct {
	timeout time.Duration
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== context：并发世界的取消协议（演示） ===")
	fmt.Printf("GOMAXPROCS=%d timeout=%s\n", runtime.GOMAXPROCS(0), cfg.timeout)

	section("1) cancel 传播：父 ctx 取消，子 ctx 立刻感知", demoCancelPropagation)
	section("2) timeout：DeadlineExceeded 与超时链路", func() { demoTimeout(cfg.timeout) })
	section("3) value：传递 request_id（只传小元数据）", demoValue)
	section("4) 把 ctx 贯穿调用链：不要丢", func() { demoCallChain(cfg.timeout) })
}

func parseFlags() config {
	var cfg config
	flag.DurationVar(&cfg.timeout, "timeout", 90*time.Millisecond, "timeout used in demoCallChain")
	flag.Parse()
	if cfg.timeout <= 0 {
		cfg.timeout = 90 * time.Millisecond
	}
	return cfg
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func demoCancelPropagation() {
	parent, cancel := context.WithCancel(context.Background())
	child, childCancel := context.WithCancel(parent)
	defer childCancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-child.Done():
			fmt.Println("child done ->", child.Err())
		case <-time.After(200 * time.Millisecond):
			fmt.Println("child still running (unexpected)")
		}
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
}

func demoTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-time.After(timeout / 2):
		fmt.Println("work finished before timeout")
	case <-ctx.Done():
		fmt.Println("ctx done ->", ctx.Err())
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), timeout/2)
	defer cancel2()

	select {
	case <-time.After(timeout):
		fmt.Println("work finished (unexpected)")
	case <-ctx2.Done():
		fmt.Println("ctx2 done ->", ctx2.Err())
	}
}

func demoValue() {
	base := context.Background()
	ctx := context.WithValue(base, keyRequestID, "req-1001")

	fmt.Println("request_id =", requestID(ctx))
	fmt.Println("unknown key =", ctx.Value(ctxKey("missing")))
}

func requestID(ctx context.Context) string {
	v := ctx.Value(keyRequestID)
	s, _ := v.(string)
	return s
}

func demoCallChain(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ctx = context.WithValue(ctx, keyRequestID, "req-9009")

	err := handleRequest(ctx)
	fmt.Printf("handleRequest err=%v\n", err)
}

func handleRequest(ctx context.Context) error {
	if requestID(ctx) == "" {
		return errors.New("missing request_id")
	}

	if err := callDB(ctx); err != nil {
		return fmt.Errorf("db: %w", err)
	}
	if err := callRemote(ctx); err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	return nil
}

func callDB(ctx context.Context) error {
	select {
	case <-time.After(40 * time.Millisecond):
		fmt.Println("callDB ok req=", requestID(ctx))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func callRemote(ctx context.Context) error {
	select {
	case <-time.After(80 * time.Millisecond):
		fmt.Println("callRemote ok req=", requestID(ctx))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
