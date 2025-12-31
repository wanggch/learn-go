package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== time 包：时区/解析/Timer/Ticker/坑点演示 ===")

	section("1) Duration 与单位换算", demoDuration)
	section("2) Parse vs ParseInLocation", demoParseLocation)
	section("3) Timer：超时与取消", demoTimer)
	section("4) Ticker：周期任务与 Stop", demoTicker)
	section("5) Round/Truncate 与“对齐到整点”", demoRoundTruncate)
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func demoDuration() {
	d := 1500 * time.Millisecond
	fmt.Printf("d=%s | ms=%d | sec=%.3f\n", d, d.Milliseconds(), d.Seconds())

	timeout := 2*time.Second + 300*time.Millisecond
	fmt.Printf("timeout=%s\n", timeout)
	fmt.Printf("deadline in %s\n", time.Until(time.Now().Add(timeout)))
}

func demoParseLocation() {
	layout := "2006-01-02 15:04:05"
	input := "2025-12-31 23:30:00"

	t1, err := time.Parse(layout, input)
	fmt.Printf("Parse: t=%s loc=%s err=%v\n", t1.Format(time.RFC3339), t1.Location(), err)

	locShanghai, _ := time.LoadLocation("Asia/Shanghai")
	t2, err := time.ParseInLocation(layout, input, locShanghai)
	fmt.Printf("ParseInLocation(Shanghai): t=%s loc=%s err=%v\n", t2.Format(time.RFC3339), t2.Location(), err)

	fmt.Printf("same instant? %v\n", t1.Equal(t2))
	fmt.Printf("diff = %s\n", t2.Sub(t1))
}

func demoTimer() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	work := func() error {
		time.Sleep(80 * time.Millisecond)
		return nil
	}

	start := time.Now()
	err := runWithContext(ctx, work)
	fmt.Printf("runWithContext err=%v cost=%s\n", err, time.Since(start))

	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	start = time.Now()
	err = runWithContext(ctx2, work)
	fmt.Printf("runWithContext err=%v cost=%s\n", err, time.Since(start))
}

func runWithContext(ctx context.Context, job func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- job()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func demoTicker() {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	count := 0
	start := time.Now()
	for {
		select {
		case t := <-ticker.C:
			count++
			fmt.Printf("tick #%d at %s\n", count, t.Format("15:04:05.000"))
			if count >= 3 {
				fmt.Printf("stop after %s\n", time.Since(start))
				return
			}
		}
	}
}

func demoRoundTruncate() {
	now := time.Now()
	fmt.Printf("now       = %s\n", now.Format("15:04:05.000"))
	fmt.Printf("Truncate  = %s\n", now.Truncate(time.Second).Format("15:04:05.000"))
	fmt.Printf("Round     = %s\n", now.Round(time.Second).Format("15:04:05.000"))

	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	fmt.Printf("nextMinute= %s\n", nextMinute.Format("15:04:05.000"))
}
