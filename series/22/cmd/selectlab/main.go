package main

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"time"
)

type config struct {
	timeout time.Duration
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== select：写出不会死锁的并发代码（演示） ===")
	fmt.Printf("GOMAXPROCS=%d timeout=%s\n", runtime.GOMAXPROCS(0), cfg.timeout)

	section("1) 超时：select + time.After", func() { demoTimeout(cfg.timeout) })
	section("2) 非阻塞：default 分支", demoNonBlocking)
	section("3) 多路输入：fan-in（合并多个 channel）", demoFanIn)
	section("4) 取消传播：ctx.Done()", func() { demoContextCancel(cfg.timeout) })
	section("5) nil channel：动态禁用 select 分支", demoNilChannel)
}

func parseFlags() config {
	var cfg config
	flag.DurationVar(&cfg.timeout, "timeout", 80*time.Millisecond, "timeout used in demos")
	flag.Parse()
	if cfg.timeout <= 0 {
		cfg.timeout = 80 * time.Millisecond
	}
	return cfg
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func demoTimeout(timeout time.Duration) {
	work := func(d time.Duration) <-chan string {
		ch := make(chan string, 1)
		go func() {
			time.Sleep(d)
			ch <- fmt.Sprintf("work finished in %s", d)
		}()
		return ch
	}

	select {
	case msg := <-work(40 * time.Millisecond):
		fmt.Println("case work:", msg)
	case <-time.After(timeout):
		fmt.Println("case timeout: exceeded")
	}

	select {
	case msg := <-work(120 * time.Millisecond):
		fmt.Println("case work:", msg)
	case <-time.After(timeout):
		fmt.Println("case timeout: exceeded")
	}
}

func demoNonBlocking() {
	ch := make(chan int, 1)

	// non-blocking receive
	select {
	case v := <-ch:
		fmt.Println("recv:", v)
	default:
		fmt.Println("recv: no data (default)")
	}

	// non-blocking send
	select {
	case ch <- 1:
		fmt.Println("send: ok")
	default:
		fmt.Println("send: would block (default)")
	}

	select {
	case ch <- 2:
		fmt.Println("send: ok")
	default:
		fmt.Println("send: would block (default)")
	}
}

func demoFanIn() {
	a := producer("A", 3, 25*time.Millisecond)
	b := producer("B", 3, 40*time.Millisecond)

	merged := fanIn(a, b)
	for msg := range merged {
		fmt.Println("got:", msg)
	}
	fmt.Println("merged closed")
}

func producer(name string, n int, every time.Duration) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for i := 1; i <= n; i++ {
			time.Sleep(every)
			out <- fmt.Sprintf("%s-%d", name, i)
		}
	}()
	return out
}

func fanIn(a, b <-chan string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		aOpen, bOpen := true, true
		for aOpen || bOpen {
			select {
			case v, ok := <-a:
				if !ok {
					aOpen = false
					a = nil // disable this case
					continue
				}
				out <- v
			case v, ok := <-b:
				if !ok {
					bOpen = false
					b = nil // disable this case
					continue
				}
				out <- v
			}
		}
	}()
	return out
}

func demoContextCancel(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// simulate long work that checks ctx
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fmt.Println("worker: ctx canceled ->", ctx.Err())
				return
			case <-ticker.C:
				fmt.Println("worker: tick")
			}
		}
	}()

	select {
	case <-done:
		fmt.Println("main: worker finished")
	case <-ctx.Done():
		fmt.Println("main: ctx done ->", ctx.Err())
		<-done
	}
}

func demoNilChannel() {
	var a <-chan int
	b := make(chan int, 1)

	b <- 7

	select {
	case v := <-a:
		fmt.Println("a:", v)
	case v := <-b:
		fmt.Println("b:", v, "(a is nil so this case is effectively disabled)")
	}

	// When both are nil, select will block forever; avoid it by adding timeout/default.
	a = nil
	b = nil
	select {
	case <-a:
		fmt.Println("unreachable")
	case <-b:
		fmt.Println("unreachable")
	case <-time.After(10 * time.Millisecond):
		fmt.Println("both nil -> timeout branch prevents deadlock")
	}
}
