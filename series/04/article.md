# 变量、类型与零值：Go 如何减少未定义行为

## 引子场景（100~200 字）

你接手一个网关服务的配置加载逻辑。线上偶发超时飙升，排查发现有人忘记在配置里填 `timeout`，结果程序把它当成“无限等待”。类似的问题在其他语言里很常见：未初始化的字段要么是 `nil`，要么是不确定值。Go 的处理方式更直接：每个类型都有**零值**，而你可以把它当成“缺省值的一部分”。理解零值，是减少线上事故的第一步。

## 目录

1. 零值是什么：Go 用它避免未定义行为
2. 常见类型的零值与含义
3. 在配置里显式处理零值
4. 小结：零值是一种约定

## 正文

### 1) 零值是什么：Go 用它避免未定义行为

Go 的变量即使不初始化，也会被赋一个“类型安全”的默认值。这样你不会拿到随机内存，也不会因为 `nil` 乱飞。

**小结**：零值让“没初始化”这件事变得可预测。

### 2) 常见类型的零值与含义

- `string`：`""`，可以视为“未填”。
- `int`：`0`，常用于计数或默认值。
- `bool`：`false`，可表示未开启。
- `time.Duration`：`0`，通常表示“未配置”。

**代码片段：结构体零值**

```go
type Config struct {
	ServiceName string
	Timeout     time.Duration
	Retry       int
	EnableDebug bool
}
```

**小结**：零值不是“无意义”，而是“未配置”的信号。

### 3) 在配置里显式处理零值

这一节的示例就是一个小配置加载器：如果某些字段是零值，就补上默认值；如果关键字段为空，就直接返回错误。

**代码片段：零值补齐**

```go
if c.Timeout == 0 {
	c.Timeout = 3 * time.Second
}
if c.Retry == 0 {
	c.Retry = 2
}
```

**小结**：零值并不危险，危险的是你不知道它代表“未配置”。

### 4) 小结：零值是一种约定

Go 不会帮你做“业务默认值”，但它保证你拿到的是可预测的零值。你要做的是把零值变成“业务约定的一部分”。

**本篇立刻可用的小产出**：一个带默认值补齐与校验的小配置模块。

## 示例项目

**目录树**

```
series/
└── 04/
    ├── article.md
    ├── go.mod
    ├── cmd/
    │   └── zero/
    │       └── main.go
    └── internal/
        └── settings/
            ├── settings.go
            └── settings_test.go
```

**文件内容**

`go.mod`

```go
module learn-go/series/04

go 1.22
```

`cmd/zero/main.go`

```go
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
```

`internal/settings/settings.go`

```go
package settings

import (
	"fmt"
	"time"
)

type Config struct {
	ServiceName string
	Timeout     time.Duration
	Retry       int
	EnableDebug bool
}

func Default() Config {
	return Config{
		ServiceName: "order-gateway",
		Timeout:     3 * time.Second,
		Retry:       2,
		EnableDebug: false,
	}
}

func ApplyZero(c Config) (Config, error) {
	if c.ServiceName == "" {
		return Config{}, fmt.Errorf("ServiceName 不能为空")
	}
	if c.Timeout == 0 {
		c.Timeout = 3 * time.Second
	}
	if c.Retry == 0 {
		c.Retry = 2
	}
	return c, nil
}
```

`internal/settings/settings_test.go`

```go
package settings

import (
	"testing"
	"time"
)

func TestApplyZero(t *testing.T) {
	cfg, err := ApplyZero(Config{ServiceName: "order-gateway"})
	if err != nil {
		t.Fatalf("ApplyZero returned error: %v", err)
	}
	if cfg.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %v, want %v", cfg.Timeout, 3*time.Second)
	}
	if cfg.Retry != 2 {
		t.Fatalf("Retry = %d, want %d", cfg.Retry, 2)
	}
}

func TestApplyZeroEmptyName(t *testing.T) {
	_, err := ApplyZero(Config{})
	if err == nil {
		t.Fatal("expected error for empty ServiceName")
	}
}
```

## 运行方式（命令行）

```bash
# 运行本篇示例

go run ./series/04/cmd/zero -service="order-gateway" -timeout=2s -retry=3 -debug=true

# 运行测试

go test ./series/04/...
```

## 知识点清单

- Go 的零值概念与意义
- 常见类型零值的语义
- 用零值补齐配置默认值

## 自检结果

- 已运行 `go test ./series/04/...`
- 已运行 `go run ./series/04/cmd/zero`
- README 中命令可复制粘贴执行
