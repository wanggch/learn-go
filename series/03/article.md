# 一个目录 = 一个包：Go 项目结构到底该怎么摆

## 引子场景（100~200 字）

你接手一个老项目，代码全都塞在根目录：`main.go`、`utils.go`、`helper.go`、`tmp.go`……谁都说不清每个文件的边界。上线前要加一个“发布提醒”功能，你把代码塞进 `utils.go`，同事又在 `main.go` 加了一个同名函数，合并冲突一片。Go 的包机制其实是为这种混乱提供出口：一个目录就是一个包，边界清楚，职责清楚，编译器也会帮你拦住“越界引用”。

## 目录

1. 包的边界：目录即包，包名即约束
2. `internal` 与 `pkg`：什么能被外部引用
3. 把入口变薄：`cmd` 只放启动代码
4. 小结：最小可用的项目结构

## 正文

### 1) 包的边界：目录即包，包名即约束

Go 的规则很硬：**一个目录就是一个包**。同一目录下的 `.go` 文件必须写同一个 `package`，否则直接编译失败。这个规则非常重要，因为它逼着你做“最小边界拆分”。

**代码片段：目录内统一包名**

```go
package config

func New(appName, owner string) (AppConfig, error) {
	// 省略...
}
```

**小结**：包边界是靠目录划分出来的，不要试图用“命名规则”兜底。

### 2) `internal` 与 `pkg`：什么能被外部引用

当你的项目开始被多个模块引用时，`internal` 和 `pkg` 会非常重要：

- `internal/`：只允许当前模块内引用，外部包无法导入。
- `pkg/`：可以被外部使用的公共包。

**代码片段：内部包引用**

```go
cfg, err := config.New(*appName, *owner)
```

`config` 在 `internal/config` 中，对外是“不可见”的。

**小结**：把“只服务本项目的代码”放进 `internal`，把“希望复用的能力”放进 `pkg`。

### 3) 把入口变薄：`cmd` 只放启动代码

`cmd` 里只放最薄的启动层：参数解析、配置加载、调用核心逻辑。不要在 `main` 写业务逻辑，这会让测试和复用都很痛苦。

**代码片段：入口只做拼装**

```go
message := greet.Format(greet.Message{AppName: cfg.AppName, Owner: cfg.Owner})
fmt.Println(message)
```

**小结**：入口文件只负责“组装”，业务逻辑应该落在包里。

### 4) 小结：最小可用的项目结构

本篇示例项目包含三层：

- `cmd/app`：程序入口
- `internal/config`：内部配置逻辑
- `pkg/greet`：可复用的输出格式化

它满足了一个基本原则：边界清晰、入口薄、测试好写。下一篇会继续聊“变量、类型与零值”。

**本篇立刻可用的小产出**：一个干净的项目结构模板，可直接作为新项目骨架。

## 示例项目

**目录树**

```
series/
└── 03/
    ├── article.md
    ├── go.mod
    ├── cmd/
    │   └── app/
    │       └── main.go
    ├── internal/
    │   └── config/
    │       ├── config.go
    │       └── config_test.go
    └── pkg/
        └── greet/
            └── greet.go
```

**文件内容**

`go.mod`

```go
module learn-go/series/03

go 1.22
```

`cmd/app/main.go`

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"learn-go/series/03/internal/config"
	"learn-go/series/03/pkg/greet"
)

func main() {
	appName := flag.String("app", "deploy-bot", "应用名称")
	owner := flag.String("owner", "", "负责人或团队")
	flag.Parse()

	cfg, err := config.New(*appName, *owner)
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}

	message := greet.Format(greet.Message{AppName: cfg.AppName, Owner: cfg.Owner})
	fmt.Println(message)
}
```

`internal/config/config.go`

```go
package config

import (
	"fmt"
	"strings"
)

type AppConfig struct {
	AppName string
	Owner   string
}

func New(appName, owner string) (AppConfig, error) {
	appName = strings.TrimSpace(appName)
	owner = strings.TrimSpace(owner)
	if appName == "" {
		return AppConfig{}, fmt.Errorf("appName 不能为空")
	}
	if owner == "" {
		owner = "团队"
	}
	return AppConfig{AppName: appName, Owner: owner}, nil
}
```

`internal/config/config_test.go`

```go
package config

import "testing"

func TestNew(t *testing.T) {
	cfg, err := New("service", "平台组")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if cfg.AppName != "service" {
		t.Fatalf("AppName = %q, want %q", cfg.AppName, "service")
	}
	if cfg.Owner != "平台组" {
		t.Fatalf("Owner = %q, want %q", cfg.Owner, "平台组")
	}
}

func TestNewDefaultOwner(t *testing.T) {
	cfg, err := New("service", " ")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if cfg.Owner != "团队" {
		t.Fatalf("Owner = %q, want %q", cfg.Owner, "团队")
	}
}

func TestNewEmptyAppName(t *testing.T) {
	_, err := New(" ", "平台组")
	if err == nil {
		t.Fatal("expected error for empty appName")
	}
}
```

`pkg/greet/greet.go`

```go
package greet

import "fmt"

type Message struct {
	AppName string
	Owner   string
}

func Format(msg Message) string {
	return fmt.Sprintf("[%s] 由 %s 维护，今天运行正常。", msg.AppName, msg.Owner)
}
```

## 运行方式（命令行）

```bash
# 运行本篇示例

go run ./series/03/cmd/app -app="deploy-bot" -owner="平台组"

# 运行测试

go test ./series/03/...
```

## 知识点清单

- 一个目录就是一个包的硬规则
- `internal` 与 `pkg` 的职责边界
- `cmd` 入口只做组装，不写业务逻辑

## 自检结果

- 已运行 `go test ./series/03/...`
- 已运行 `go run ./series/03/cmd/app`
- README 中命令可复制粘贴执行
