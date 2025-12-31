package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type config struct {
	buffer     int
	items      int
	prodDelay  time.Duration
	consDelay  time.Duration
	workers    int
	semLimit   int
	showTiming bool
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== channel：通信 vs 同步（演示） ===")
	fmt.Printf("GOMAXPROCS=%d\n", runtime.GOMAXPROCS(0))

	section("1) 无缓冲：发送=同步点（handshake）", func() { demoUnbufferedHandshake(cfg.showTiming) })
	section("2) 有缓冲：队列化通信（producer-consumer）", func() { demoBufferedQueue(cfg) })
	section("3) close 与 ok：广播完成信号", func() { demoCloseAndOK(cfg.workers) })
	section("4) channel 当信号量：限制并发（同步用途）", func() { demoSemaphore(cfg.semLimit) })
}

func parseFlags() config {
	var cfg config
	flag.IntVar(&cfg.buffer, "buffer", 3, "buffer size for buffered demo")
	flag.IntVar(&cfg.items, "items", 8, "items produced in buffered demo")
	flag.DurationVar(&cfg.prodDelay, "prod", 10*time.Millisecond, "producer delay per item")
	flag.DurationVar(&cfg.consDelay, "cons", 30*time.Millisecond, "consumer delay per item")
	flag.IntVar(&cfg.workers, "workers", 3, "number of consumers in close demo")
	flag.IntVar(&cfg.semLimit, "sem", 2, "semaphore limit in demo")
	flag.BoolVar(&cfg.showTiming, "timing", true, "show timestamps in unbuffered handshake demo")
	flag.Parse()

	if cfg.buffer < 0 {
		cfg.buffer = 0
	}
	if cfg.items < 0 {
		cfg.items = 0
	}
	if cfg.workers < 1 {
		cfg.workers = 1
	}
	if cfg.semLimit < 1 {
		cfg.semLimit = 1
	}
	if cfg.prodDelay < 0 {
		cfg.prodDelay = 0
	}
	if cfg.consDelay < 0 {
		cfg.consDelay = 0
	}
	return cfg
}

func section(title string, fn func()) {
	fmt.Printf("\n--- %s ---\n", title)
	fn()
}

func demoUnbufferedHandshake(showTiming bool) {
	ch := make(chan string) // unbuffered

	start := time.Now()
	logf := func(format string, args ...any) {
		if showTiming {
			prefix := fmt.Sprintf("+%s ", time.Since(start).Truncate(time.Millisecond))
			fmt.Printf(prefix+format+"\n", args...)
			return
		}
		fmt.Printf(format+"\n", args...)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logf("sender: 准备发送 A（会阻塞，直到有人接收）")
		ch <- "A"
		logf("sender: 已发送 A（说明接收方已经接走）")
	}()

	go func() {
		defer wg.Done()
		time.Sleep(40 * time.Millisecond)
		logf("receiver: 准备接收（此时 sender 应该已经在等）")
		v := <-ch
		logf("receiver: 收到 %q", v)
	}()

	wg.Wait()
}

func demoBufferedQueue(cfg config) {
	ch := make(chan int, cfg.buffer)
	done := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for v := range ch {
			fmt.Printf("consumer: got=%d (len=%d cap=%d)\n", v, len(ch), cap(ch))
			time.Sleep(cfg.consDelay)
		}
		fmt.Println("consumer: channel closed, exit")
		close(done)
	}()

	for i := 0; i < cfg.items; i++ {
		time.Sleep(cfg.prodDelay)
		fmt.Printf("producer: send=%d (len=%d cap=%d)\n", i, len(ch), cap(ch))
		ch <- i
	}
	close(ch)
	<-done
	wg.Wait()
}

func demoCloseAndOK(workers int) {
	jobs := make(chan int)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 1; w <= workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for job := range jobs {
				fmt.Printf("worker-%d: job=%d\n", w, job)
				time.Sleep(15 * time.Millisecond)
			}
			fmt.Printf("worker-%d: jobs closed\n", w)
		}()
	}

	for i := 0; i < 6; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	fmt.Println("ok 形式读取：")
	v, ok := <-jobs
	fmt.Printf("  v=%d ok=%v (closed channel read returns zero value)\n", v, ok)
}

func demoSemaphore(limit int) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	task := func(id int) {
		defer wg.Done()
		sem <- struct{}{}        // acquire
		defer func() { <-sem }() // release
		fmt.Printf("task-%d: start (inflight=%d)\n", id, len(sem))
		time.Sleep(35 * time.Millisecond)
		fmt.Printf("task-%d: end   (inflight=%d)\n", id, len(sem))
	}

	wg.Add(5)
	for i := 1; i <= 5; i++ {
		go task(i)
	}
	wg.Wait()
}
