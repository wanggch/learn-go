package main

import (
	"flag"
	"fmt"
	"runtime"
	"strings"
	"time"

	"learn-go/series/01/internal/reasons"
)

func main() {
	name := flag.String("name", "工程师", "读者名称")
	lang := flag.String("lang", "go", "关注的语言")
	flag.Parse()

	lines := []string{
		fmt.Sprintf("你好，%s！", *name),
		fmt.Sprintf("你正在体验：%s", strings.ToUpper(*lang)),
		fmt.Sprintf("今天的结论：%s", reasons.Reason(*lang)),
		fmt.Sprintf("运行环境：%s/%s", runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("Go 版本：%s", runtime.Version()),
		fmt.Sprintf("生成时间：%s", time.Now().Format(time.RFC3339)),
	}

	fmt.Println(strings.Join(lines, "\n"))
}
