# 从 `main` 开始：Go 程序是怎么跑起来的

## 引子场景（100~200 字）

你接手一个线上小工具：每天定时抓取配置，再打印关键指标。故障发生时，日志里只有一句话：“程序启动失败”。排查一圈才发现是启动参数缺失。你意识到：一个可靠的程序，必须从入口就“把话说清楚”——参数怎么解析、默认值是什么、出错时怎么反馈。理解 Go 的入口结构，是把程序做成“可交付工具”的第一步。

## 目录

1. `package main` 与入口函数的意义
2. 最小 CLI 的结构：参数、默认值、校验
3. 让错误在入口处可读
4. 小结：一个可复用的入口模板

## 正文

### 1) `package main` 与入口函数的意义

Go 的可执行程序从 `package main` 开始，入口是 `func main()`。它的意义很简单：告诉编译器这是一段要产出可执行文件的代码。只要你理解“入口就是程序的第一层输出”，你就会自然地关注参数和错误。

**小结**：`main` 是程序“面对世界”的第一层，入口写得清楚，后面才稳定。

### 2) 最小 CLI 的结构：参数、默认值、校验

这一篇我们把“参数解析”变成一个小包，避免所有逻辑都堆在 `main` 里。这样做的价值是：入口清爽、逻辑可测试。

**代码片段：参数解析**

```go
fs := flag.NewFlagSet("hello", flag.ContinueOnError)
name := fs.String("name", "工程师", "读者名称")
lang := fs.String("lang", "go", "关注的语言")
```

再补一条规则：名称不能为空；语言默认走 `go`。

**小结**：入口的参数解析，不需要复杂，但必须明确默认值与约束。

### 3) 让错误在入口处可读

CLI 的第一条体验，是出错信息是否可读。这里的做法是：解析失败直接向 `stderr` 输出，并返回非零退出码。

**代码片段：错误反馈**

```go
cfg, err := cliinfo.Parse(os.Args[1:])
if err != nil {
	fmt.Fprintln(os.Stderr, "参数错误:", err)
	os.Exit(1)
}
```

**小结**：让错误在入口处“能读懂”，比在后面静默失败更重要。

### 4) 小结：一个可复用的入口模板

这次的产出是一个可直接复用的 CLI 入口模板：有参数、有默认值、有校验、有清晰错误输出。下一篇会继续拆解“一个目录=一个包”的结构组织方式。

**本篇立刻可用的小产出**：带参数校验的 CLI 程序入口模板。

## 示例项目

**目录树**

```
series/
└── 02/
    ├── article.md
    ├── go.mod
    ├── cmd/
    │   └── cli/
    │       └── main.go
    └── internal/
        ├── cliinfo/
        │   ├── cliinfo.go
        │   └── cliinfo_test.go
        └── reasons/
            ├── reasons.go
            └── reasons_test.go
```

**文件内容**

`cmd/cli/main.go`

```go
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
```

`internal/cliinfo/cliinfo.go`

```go
package cliinfo

import (
	"flag"
	"fmt"
	"strings"
)

type Config struct {
	Name string
	Lang string
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("hello", flag.ContinueOnError)
	name := fs.String("name", "工程师", "读者名称")
	lang := fs.String("lang", "go", "关注的语言")
	fs.SetOutput(new(strings.Builder))

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Name: strings.TrimSpace(*name),
		Lang: strings.ToLower(strings.TrimSpace(*lang)),
	}
	if cfg.Name == "" {
		return Config{}, fmt.Errorf("name 不能为空")
	}
	if cfg.Lang == "" {
		cfg.Lang = "go"
	}
	return cfg, nil
}
```

`internal/cliinfo/cliinfo_test.go`

```go
package cliinfo

import "testing"

func TestParse(t *testing.T) {
	cfg, err := Parse([]string{"-name", "小明", "-lang", "Go"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.Name != "小明" {
		t.Fatalf("name = %q, want %q", cfg.Name, "小明")
	}
	if cfg.Lang != "go" {
		t.Fatalf("lang = %q, want %q", cfg.Lang, "go")
	}
}

func TestParseEmptyName(t *testing.T) {
	_, err := Parse([]string{"-name", "  "})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}
```

## 运行方式（命令行）

```bash
# 运行本篇示例

go run ./series/02/cmd/cli -name="小明" -lang=go

# 运行测试

go test ./series/02/...
```

## 知识点清单

- `package main` 与 `func main()` 的职责
- `flag` 解析参数的基本模式
- 入口参数校验与错误反馈

## 自检结果

- 已运行 `go test ./...`
- 已运行 `go run ./cmd/cli`
- README 中命令可复制粘贴执行
# go.mod

```go
module learn-go/series/02

go 1.22
```
