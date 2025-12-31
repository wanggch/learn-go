package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Job func() error

type Result struct {
	Name   string
	Status string
	Err    error
	Cost   time.Duration
}

// SafeExecute wraps a job with defer-recover and common cleanup.
func SafeExecute(name string, job Job, cleanup func()) (res Result) {
	res.Name = name
	res.Status = "ok"
	start := time.Now()

	defer func() {
		res.Cost = time.Since(start)
	}()

	defer func() {
		// cleanup always runs, even if panic happens
		if cleanup != nil {
			cleanup()
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[recover] job=%s panic=%v\n", name, r)
			res.Status = "panic"
			res.Err = fmt.Errorf("panic: %v", r)
		}
	}()

	if runErr := job(); runErr != nil {
		res.Status = "error"
		res.Err = runErr
	}

	return res
}

func main() {
	fmt.Println("=== defer / panic / recover 演示 ===")

	results := []Result{
		SafeExecute("normal", func() error {
			defer fmt.Println("  [normal] defer #1")
			defer fmt.Println("  [normal] defer #2 (LIFO)")
			time.Sleep(20 * time.Millisecond)
			return nil
		}, nil),
		SafeExecute("with-error", func() error {
			defer fmt.Println("  [with-error] defer 也会执行")
			return errors.New("业务错误：余额不足")
		}, func() {
			fmt.Println("  [with-error] cleanup: release db connection")
		}),
		SafeExecute("panic", func() error {
			defer fmt.Println("  [panic] defer 1")
			defer fmt.Println("  [panic] defer 2")
			panic("未知 panic：nil pointer")
		}, func() {
			fmt.Println("  [panic] cleanup: close files")
		}),
		SafeExecute("panic-no-recover", func() error {
			defer fmt.Println("  [panic-no-recover] defer only, no recover")
			panic("未恢复的 panic")
		}, func() {
			fmt.Println("  [panic-no-recover] cleanup before exit")
		}),
	}

	for _, r := range results {
		fmt.Printf("\njob=%s status=%s cost=%s\n", r.Name, r.Status, r.Cost)
		if r.Err != nil {
			fmt.Printf("  err: %v\n", r.Err)
		}
	}

	fmt.Println("\n演示 os.Exit：defer 不会执行")
	demoExit()
}

func demoExit() {
	defer fmt.Println("  [exit] 我不会被打印")
	if len(os.Args) > 1000 { // 不触发，避免真实退出
		os.Exit(1)
	}
	fmt.Println("  [exit] 未调用 os.Exit，程序继续")
}
