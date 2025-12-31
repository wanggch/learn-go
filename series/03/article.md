# 一个目录 = 一个包：Go 项目结构到底该怎么摆

你好，我是汪小成。很多人学 Go 后最容易卡在“项目结构”：到底包该放哪？internal 和 pkg 有啥区别？有的项目一口气堆成几十个包，结果依赖乱成一团。结构不是为了好看，而是为了 **约束依赖方向、降低维护成本**。本文会先准备环境，再解释 Go 的包与目录规则，最后用一个可运行小项目示范结构拆分、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/03`。
- 示例入口：`series/03/cmd/app/main.go`。

### 1.2 运行命令

```bash
go run ./series/03/cmd/app -app="deploy-bot" -owner="平台组"
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/03/cmd/app -app="deploy-bot" -owner="平台组"
```

### 1.3 前置知识

- 了解 `package` 与 `import` 的基础概念。
- 理解 `main` 包的入口作用。

提示：结构设计的目标是“可维护”，而不是“目录越多越高级”，先定边界，更稳。

补充建议：如果你正在重构旧项目，先画出依赖方向图，再决定拆包顺序，会更稳也更顺。

配图建议：
- 一张“目录结构与依赖方向”示意图。
- 一张“cmd/internal/pkg 三者关系图”。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 一个目录就是一个包

**概念**：Go 的包边界就是目录边界，同目录文件属于同包。  
**示例**：`pkg/greet` 目录下的 `greet.go` 组成 `package greet`。  
**为什么这么设计**：降低复杂度，让包结构一眼可见。

### 2.2 `cmd/` 放入口，`internal/` 放内部能力

**概念**：`cmd` 用于可执行入口，`internal` 是访问边界。  
**示例**：`cmd/app` 只负责入口；`internal/config` 只给内部使用。  
**为什么这么设计**：限制依赖方向，避免内部逻辑外泄。

### 2.3 `pkg/` 用于可复用的公共包

**概念**：`pkg` 面向“可公开复用”的能力。  
**示例**：`pkg/greet` 提供对外可用的格式化函数。  
**为什么这么设计**：明确“公开”和“私有”的边界。

### 2.4 依赖方向必须单向

**概念**：依赖应该从入口指向内部能力，而不是反过来。  
**示例**：`cmd` 导入 `internal` 和 `pkg`，但 `internal` 不导入 `cmd`。  
**为什么这么设计**：防止循环依赖，避免后期结构崩坏。

### 2.5 结构要服务于可测试性

**概念**：把可测试逻辑放进包里，而不是堆在入口。  
**示例**：`internal/config` 可以单独测参数校验。  
**为什么这么设计**：入口只装配，测试只覆盖业务逻辑。

### 2.6 包可见性与导出规则

**概念**：首字母大写的标识符才是对外可见的。  
**示例**：`Config` 可导出，`config` 只在包内使用。  
**为什么这么设计**：让包拥有明确的“公开 API”，减少误用。

### 2.7 命名就是“可读性资产”

**概念**：目录名、包名、类型名应该表达业务含义。  
**示例**：`config` 比 `utils` 更清晰，`greet` 比 `common` 更具体。  
**为什么这么设计**：命名清晰能减少沟通成本，降低维护难度。

### 2.8 包粒度要“够用就好”

**概念**：包太大难维护，包太小难理解。  
**示例**：把配置、领域逻辑、工具函数拆成独立包，但不要每个函数一个包。  
**为什么这么设计**：合适的粒度能让依赖更稳定，也更容易测试。

小结：过度分包会让阅读成本陡增，找到“够用且清晰”的边界最重要。

配图建议：
- 一张“依赖方向单向箭头图”。
- 一张“内部包可测试性”示意图。

## 3. 完整代码示例（可运行）

示例包含：

1. `cmd/app`：入口。
2. `internal/config`：配置与校验。
3. `pkg/greet`：公共输出格式。

代码路径：`series/03`。

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

说明：`cmd` 负责装配，`internal` 负责规则，`pkg` 负责可复用能力，结构清晰且可维护。

实践建议：如果你在重构旧项目，可以先把入口逻辑抽到 `cmd`，再把核心逻辑下沉到 `internal`，最后再考虑哪些能力值得公开到 `pkg`，更省心也更稳。

配图建议：
- 一张“目录树”截图。
- 一张“包依赖关系图”。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/03/cmd/app -app="deploy-bot" -owner="平台组"
```

示例输出（节选）：

```
[deploy-bot] 由 平台组 维护，今天运行正常。
```

输出解读：入口只负责参数解析与装配，实际业务逻辑放在包中，结构更清晰。这样的结构也便于后续把入口替换成其他命令行或服务入口，更利于扩展、更直观。

截图描述建议：
- 截一张输出图，突出 **app/owner** 字段。
- 再截一张目录树图，突出 `cmd`/`internal`/`pkg`。

配图建议：
- 一张“入口与业务分离”示意图。
- 一张“包边界说明”图。

## 5. 常见坑 & 解决方案（必写）

1. **把业务逻辑写在 main**：入口越来越难维护。  
   解决：把逻辑下沉到包里。

2. **internal 被外部引用**：边界被破坏。  
   解决：公共能力放到 `pkg`。

3. **包名与目录名不一致**：阅读困难。  
   解决：保持包名与目录一致或高度相关。

4. **依赖方向混乱**：出现循环依赖。  
   解决：入口只依赖内部能力，内部不反向依赖。

5. **目录太碎**：每个函数一个包。  
   解决：以业务边界或领域拆包。

6. **目录太大**：所有逻辑堆在一个包。  
   解决：按职责拆分 internal 子包。

补充建议：结构调整要循序渐进，先把“入口、核心逻辑、公共能力”三个层次理清，再逐步细分包。
配图建议：
- 一张“结构反例”图。
- 一张“拆包原则”示意图。

## 6. 进阶扩展 / 思考题

1. 把 `pkg/greet` 改成多个输出风格，看看如何组织包。
2. 试着写一个 `internal/config` 的测试用例。
3. 如果要给外部项目使用，你会把哪些包放到 `pkg`？
4. 思考：你的项目目前是否有“依赖反向”的隐患？

补充建议：把“包边界与依赖方向”写进团队规范，长期收益很大，也更稳妥。

配图建议：
- 一张“internal vs pkg 决策树”。
- 一张“结构演进路线”图。
