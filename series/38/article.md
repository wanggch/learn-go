# 项目结构与依赖：internal / cmd 的真正用途

你好，我是汪小成。很多人学 Go 的项目结构时只记住一句话：“把 main 放到 cmd”，但真正写项目时依然困惑：internal 到底该放什么？为什么有些包别人导不进来？项目越写越大，依赖关系越来越乱。其实 `internal` 和 `cmd` 不是摆设，它们是 **约束依赖方向** 的工具。本文会先准备环境，再讲清 internal/cmd 的核心概念与设计原因，最后给出完整可运行示例、运行效果、常见坑与进阶思考。

## 目录

1. 环境准备 / 前提知识  
2. 核心概念解释（概念 → 示例 → 为什么这么设计）  
3. 完整代码示例（可运行）  
4. 运行效果 + 截图描述  
5. 常见坑 & 解决方案（必写）  
6. 进阶扩展 / 思考题  

## 1. 环境准备 / 前提知识

### 1.1 版本与目录

- Go 1.22+（仓库根目录使用 `go.work`）。
- 本篇目录：`series/38`。
- 示例入口：`series/38/cmd/portal` 与 `series/38/cmd/worker`。

### 1.2 运行命令

```bash
APP_NAME=order APP_REGION=cn APP_WORKERS=6 go run ./series/38/cmd/portal
APP_NAME=order APP_REGION=cn APP_WORKERS=3 go run ./series/38/cmd/worker
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
APP_NAME=order APP_REGION=cn APP_WORKERS=6 GOCACHE=$(pwd)/.cache/go-build go run ./series/38/cmd/portal
APP_NAME=order APP_REGION=cn APP_WORKERS=3 GOCACHE=$(pwd)/.cache/go-build go run ./series/38/cmd/worker
```

### 1.3 前置知识

- 了解 Go module 与包导入路径。
- 了解 `main` 包的基本作用。

提示：本示例不会启动真实服务，而是模拟两个命令入口，突出结构关系。

小建议：在团队项目里，把目录结构约定写进 README 或贡献指南，避免后续同事随意打破依赖方向。

配图建议：
- 一张“cmd → internal 依赖方向”示意图。
- 一张“多入口程序结构图”。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `cmd/` 表示可执行入口

**概念**：每个子目录代表一个可执行程序。  
**示例**：`cmd/portal` 与 `cmd/worker` 分别对应两个入口。  
**为什么这么设计**：一个项目往往不止一个入口，分目录可保持清晰。

### 2.2 `internal/` 是“访问边界”

**概念**：`internal` 下的包只能被当前模块内部导入。  
**示例**：`internal/config` 只能被本模块使用。  
**为什么这么设计**：强制依赖方向，避免内部细节被外部包调用。

### 2.3 `internal` 适合放“业务内部能力”

**概念**：配置加载、业务规则、数据库访问都属于内部实现。  
**示例**：`internal/report` 输出统一统计摘要。  
**为什么这么设计**：内部逻辑会变化，外部不应该直接依赖。

### 2.4 多入口共用同一套内部能力

**概念**：多个 `cmd` 可以复用同一内部包。  
**示例**：portal 与 worker 都使用 `internal/config`。  
**为什么这么设计**：避免重复实现，保证逻辑一致。

### 2.5 依赖方向要单向

**概念**：`cmd -> internal -> 其他包`，不要反向依赖。  
**示例**：内部包不应该导入 `cmd`。  
**为什么这么设计**：保证结构稳定，避免循环依赖。

### 2.6 避免“伪结构”

**概念**：目录结构不是装饰，而是组织与约束。  
**示例**：如果内部包被外部引用，需要重新划分边界。  
**为什么这么设计**：结构设计是长期可维护性的核心。

### 2.7 `pkg` 与 `internal` 的分工

**概念**：`pkg` 通常给外部复用，`internal` 只给内部使用。  
**示例**：通用字符串处理可以放 `pkg/stringsx`，业务规则放 `internal/rule`。  
**为什么这么设计**：公开与私有边界清晰，团队协作成本更低。

补充一句：当确实需要被外部复用时，再把包从 `internal` 迁到 `pkg`，而不是一开始就全部公开。

配图建议：
- 一张“依赖方向单向箭头图”。
- 一张“internal vs pkg 对比图”。

## 3. 完整代码示例（可运行）

示例包含：

1. 两个命令入口：`portal` 和 `worker`。
2. 共享内部包：`internal/config` 和 `internal/report`。
3. 通过环境变量改变运行配置。

代码路径：`series/38`。

```go
package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App     string
	Mode    string
	Region  string
	Workers int
}

func Load() Config {
	cfg := Config{
		App:     "sample",
		Mode:    "portal",
		Region:  "local",
		Workers: 4,
	}

	if v := strings.TrimSpace(os.Getenv("APP_NAME")); v != "" {
		cfg.App = v
	}
	if v := strings.TrimSpace(os.Getenv("APP_MODE")); v != "" {
		cfg.Mode = v
	}
	if v := strings.TrimSpace(os.Getenv("APP_REGION")); v != "" {
		cfg.Region = v
	}
	if v := strings.TrimSpace(os.Getenv("APP_WORKERS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Workers = n
		}
	}

	return cfg
}
```

```go
package report

import (
	"fmt"
	"time"

	"learn-go/series/38/internal/config"
)

type Snapshot struct {
	Config  config.Config
	Handled int
	Failed  int
	Elapsed time.Duration
}

func Summary(s Snapshot) string {
	return fmt.Sprintf(
		"app=%s mode=%s region=%s handled=%d failed=%d cost=%s",
		s.Config.App,
		s.Config.Mode,
		s.Config.Region,
		s.Handled,
		s.Failed,
		s.Elapsed,
	)
}
```

```go
package main

import (
	"fmt"
	"math/rand"
	"time"

	"learn-go/series/38/internal/config"
	"learn-go/series/38/internal/report"
)

func main() {
	cfg := config.Load()
	cfg.Mode = "portal"

	start := time.Now()
	handled, failed := simulateRequests(cfg.Workers, 120)

	summary := report.Summary(report.Snapshot{
		Config:  cfg,
		Handled: handled,
		Failed:  failed,
		Elapsed: time.Since(start),
	})

	fmt.Println("portal summary:")
	fmt.Println(summary)
}

func simulateRequests(workers, total int) (int, int) {
	rand.Seed(time.Now().UnixNano())
	failed := 0
	for i := 0; i < total; i++ {
		if rand.Intn(100) < 7 {
			failed++
		}
	}
	return total, failed
}
```

```go
package main

import (
	"fmt"
	"time"

	"learn-go/series/38/internal/config"
	"learn-go/series/38/internal/report"
)

func main() {
	cfg := config.Load()
	cfg.Mode = "worker"

	start := time.Now()
	handled, failed := runJobs(cfg.Workers, 80)

	summary := report.Summary(report.Snapshot{
		Config:  cfg,
		Handled: handled,
		Failed:  failed,
		Elapsed: time.Since(start),
	})

	fmt.Println("worker summary:")
	fmt.Println(summary)
}

func runJobs(workers, total int) (int, int) {
	failed := 0
	for i := 0; i < total; i++ {
		if i%17 == 0 {
			failed++
		}
		time.Sleep(2 * time.Millisecond)
		_ = workers
	}
	return total, failed
}
```

说明：两个入口共享内部能力，但外部包无法直接导入 `internal` 下的代码，这就形成了明确的依赖边界。

实践建议：当入口越来越多时，可以在 `internal` 下增加一个 `app` 包，专门负责“装配依赖”，让 `cmd` 更薄、更清晰。

配图建议：
- 一张“多入口复用内部包”的示意图。
- 一张“internal 访问边界”图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
APP_NAME=order APP_REGION=cn APP_WORKERS=6 go run ./series/38/cmd/portal
```

示例输出（节选）：

```
portal summary:
app=order mode=portal region=cn handled=120 failed=7 cost=65.588µs
```

再运行：

```bash
APP_NAME=order APP_REGION=cn APP_WORKERS=3 go run ./series/38/cmd/worker
```

示例输出（节选）：

```
worker summary:
app=order mode=worker region=cn handled=80 failed=5 cost=177.877358ms
```

输出解读：同一套内部包被两个入口复用，配置来自环境变量，入口只负责组装与调度。

补充说明：如果你把 `APP_MODE` 设置成其他值，入口仍然会覆盖成自身模式，这也是“入口自持职责”的体现。

截图描述建议：
- 截一张 portal 输出图，突出 `mode=portal`。
- 再截一张 worker 输出图，突出 `mode=worker`。

配图建议：
- 一张“入口分工对照表”。
- 一张“配置来源与运行模式”示意图。

## 5. 常见坑 & 解决方案（必写）

1. **把业务逻辑写进 `cmd`**：入口变成“巨型 main”。  
   解决：入口只做组装，业务放到 internal。

2. **internal 包外泄**：被别的模块导入。  
   解决：合理划分包边界，必要时抽成 `pkg`。

3. **目录结构只为好看**：实际依赖仍混乱。  
   解决：明确单向依赖，把架构约束写进规范。

4. **多个 cmd 重复代码**：每个入口复制粘贴配置与初始化。  
   解决：抽出 internal 包复用。

5. **依赖方向反了**：internal 依赖 cmd。  
   解决：保持 cmd 只依赖 internal，不反向引用。

6. **“大而全” internal**：所有代码塞进一个包。  
   解决：按领域拆分 internal 子包。

配图建议：
- 一张“目录结构反例”图。
- 一张“模块拆分原则”图。

## 6. 进阶扩展 / 思考题

1. 如果把公共能力开放给外部使用，你会如何拆成 `pkg`？
2. 多个 cmd 共享配置时，如何统一读取并支持不同默认值？
3. 入口越来越多时，如何写一个统一的启动脚本？
4. 假如团队协作多，你会如何在 README 中约束依赖方向？

配图建议：
- 一张“internal vs pkg 决策树”。
- 一张“多入口启动流程”图。
