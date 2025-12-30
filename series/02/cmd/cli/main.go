package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"learn-go/series/02/internal/cliinfo"
	"learn-go/series/02/internal/reasons"
)

func main() {
	cfg, err := cliinfo.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "参数错误:", err)
		os.Exit(1)
	}

	lines := []string{
		fmt.Sprintf("你好，%s！", cfg.Name),
		fmt.Sprintf("你正在体验：%s", strings.ToUpper(cfg.Lang)),
		fmt.Sprintf("今天的结论：%s", reasons.Reason(cfg.Lang)),
		fmt.Sprintf("运行环境：%s/%s", runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("Go 版本：%s", runtime.Version()),
		fmt.Sprintf("生成时间：%s", time.Now().Format(time.RFC3339)),
	}

	fmt.Println(strings.Join(lines, "\n"))
}
