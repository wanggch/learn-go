# 我为什么最终选择 Go，而不是 Java / Python

## 引子场景（100~200 字）

周一早上，运维同事发来消息：夜里新增了三台机器，日志收集脚本要跟着扩容。原来的 Python 脚本启动慢、部署要拷贝一堆依赖；你临时改了个并发小工具，结果线上环境缺库，最后只能手动补救。那一刻你开始思考：如果有一门语言，既能像脚本一样快写，又能像编译语言一样“一次构建，到处运行”，是不是能把这些日常琐事变得更稳定？Go 就是带着这种目标诞生的。

## 目录

1. Go 的设计目标：不是“全能”，而是“可落地”
2. 为什么是 Go：三件最关键的事
3. 写出第一个可执行程序：从“能跑”开始
4. 小结：适用边界与下一步

## 正文

### 1) Go 的设计目标：不是“全能”，而是“可落地”

Go 不是为了取代所有语言。它更像是给“服务端、工具链、基础设施”准备的一把趁手刀。第一次听到“编译速度快、部署简单、并发模型清晰”时，你可能觉得抽象，但工程里最痛的就是这三点。

- **编译速度快**：改完代码几秒就能得到可执行文件。
- **部署简单**：单个二进制文件即可运行，少掉依赖地狱。
- **并发模型清晰**：用 `goroutine` + `channel` 表达并发，比线程模型更轻。

这些特性并不是“更高级”，而是“更实用”。

**小结**：Go 的价值在“工程效率”和“运行稳定性”，而不是语法花活。

### 2) 为什么是 Go：三件最关键的事

在后端工程里，最常见的三类成本是：构建部署、并发复杂度、运维交付。

- **构建部署**：Go 一次编译出二进制，适合容器和裸机。
- **并发复杂度**：Go 让你用“任务”视角而不是“线程”视角表达并发。
- **运维交付**：单文件部署，迁移成本低。

**代码片段：取一个理由**

```go
reason := reasons.Reason("go")
fmt.Println(reason)
```

这段代码背后是一个最小可用的“理由库”，对初学者来说，它满足了两个目标：
1) 让你能跑起来；2) 把概念落到可执行结果里。

**小结**：语言选择不是情怀，而是“维护成本的选择”。

### 3) 写出第一个可执行程序：从“能跑”开始

下面是本篇示例的核心：一个简单的 CLI 程序，会输出“为什么选择这门语言”的结果，同时打印运行环境。这是一个可以立刻用的产出：你可以把它改成团队工具模板，也可以当作“启动样板”。

**代码片段：入口程序**

```go
name := flag.String("name", "工程师", "读者名称")
lang := flag.String("lang", "go", "关注的语言")
flag.Parse()

fmt.Println(reasons.Reason(*lang))
```

**小结**：`main` 是 Go 程序的入口，`flag` 是最简单的 CLI 参数工具。先让程序跑起来，再谈抽象。

### 4) 小结：适用边界与下一步

Go 并不是万能的：写 GUI 不合适、做前端也不合适；但当你需要“高效构建、稳定部署、并发安全”，它就很合适。下一篇会从 `main` 入手，拆解 Go 程序是如何跑起来的。

**本篇立刻可用的小产出**：一个可以打印“语言选择理由”的 CLI 程序，可作为团队小工具模板。

## 示例项目

**目录树**

```
series/
└── 01/
    ├── article.md
    ├── go.mod
    ├── cmd/
    │   └── hello/
    │       └── main.go
    └── internal/
        └── reasons/
            ├── reasons.go
            └── reasons_test.go
```

**文件内容**

`go.mod`

```go
module learn-go/series/01

go 1.22
```

`cmd/hello/main.go`

```go
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
```

`internal/reasons/reasons.go`

```go
package reasons

import "strings"

var reasonByLang = map[string]string{
	"go":     "编译快、部署简单、并发模型清晰，适合做基础设施和服务端。",
	"python": "生态丰富、验证快，适合数据处理和脚本。",
	"java":   "工程成熟、生态庞大，适合大型企业级系统。",
}

// Reason returns a short reason for a language.
func Reason(lang string) string {
	key := strings.ToLower(strings.TrimSpace(lang))
	if key == "" {
		key = "go"
	}
	if reason, ok := reasonByLang[key]; ok {
		return reason
	}
	return "先选一个目标场景，再决定语言。Go 适合服务端与工具链。"
}
```

`internal/reasons/reasons_test.go`

```go
package reasons

import "testing"

func TestReason(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "default", lang: "", want: reasonByLang["go"]},
		{name: "go", lang: "go", want: reasonByLang["go"]},
		{name: "python", lang: "python", want: reasonByLang["python"]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reason(tt.lang); got != tt.want {
				t.Fatalf("Reason(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}
```

## 运行方式（命令行）

```bash
# 运行示例

go run ./series/01/cmd/hello -name="小明" -lang=go

# 运行测试

go test ./series/01/...
```

## 知识点清单

- Go 的设计目标与适用边界
- 单文件二进制与部署优势
- `main` 入口与 `flag` 参数解析
- 用最小可用程序建立学习节奏

## 自检结果

- 已运行 `go test ./...`
- 已运行 `go run ./cmd/hello`
- README 中命令可复制粘贴执行
