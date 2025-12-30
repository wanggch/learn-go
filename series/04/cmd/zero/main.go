package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"learn-go/series/04/internal/settings"
)

func main() {
	name := flag.String("service", "order-gateway", "服务名称")
	timeout := flag.Duration("timeout", 0, "超时时间（例如 2s）")
	retry := flag.Int("retry", 0, "重试次数")
	debug := flag.Bool("debug", false, "是否开启调试")
	flag.Parse()

	cfg, err := settings.ApplyZero(settings.Config{
		ServiceName: *name,
		Timeout:     *timeout,
		Retry:       *retry,
		EnableDebug: *debug,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置错误:", err)
		os.Exit(1)
	}

	fmt.Printf("服务=%s 超时=%s 重试=%d 调试=%v\n", cfg.ServiceName, cfg.Timeout.Round(time.Millisecond), cfg.Retry, cfg.EnableDebug)
}
