# 从 `main` 开始：Go 程序是怎么跑起来的

你好，我是汪小成。你可能也遇到过这种问题：程序上线后“启动失败”，日志只留下一句“参数错误”，却没人知道哪个参数缺了、默认值是什么。更糟糕的是，不同同事写出来的入口风格不一致，工具越来越难维护。真正的工程从入口开始，`main` 不只是能跑，而是 **能解释、能校验、能失败得体面**。如果入口没打好地基，后面的逻辑再漂亮也容易出问题。本文会先准备环境，再解释入口结构的核心概念，接着给出完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/02`。
- 示例入口：`series/02/cmd/cli/main.go`。

### 1.2 运行命令

```bash
go run ./series/02/cmd/cli -name="小明" -lang=go
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/02/cmd/cli -name="小明" -lang=go
```

### 1.3 前置知识

- 了解 `package main` 与 `func main()` 的基本含义。
- 了解 `flag` 包的基础用法。

提示：入口写得清楚，比后面补文档更重要；先让错误“说人话”。另外可以先 `go env` 看一眼环境是否正确，确认路径，更稳更好、更顺。

配图建议：
- 一张“入口职责范围图”（解析/校验/报错）。
- 一张“最小 CLI 结构图”。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `package main` 是可执行的信号

**概念**：`package main` + `func main()` 表示这个包会编译成可执行文件。  
**示例**：`cmd/cli/main.go` 即程序入口。  
**为什么这么设计**：让编译器区分“库代码”和“可执行代码”。

### 2.2 参数解析要有默认值与校验

**概念**：入口要明确参数默认值，并做最基本的合法性检查。  
**示例**：`name` 不能为空、`lang` 为空时回退到 `go`。  
**为什么这么设计**：入口是对外契约，默认值是“最低可用体验”。

### 2.3 错误必须在入口“能被看懂”

**概念**：入口错误要直接输出，并用非零退出码终止。  
**示例**：`fmt.Fprintln(os.Stderr, "参数错误:", err)`。  
**为什么这么设计**：避免错误继续扩散，提升排障效率，也便于脚本处理错误分支。

### 2.4 入口薄，逻辑下沉

**概念**：参数解析与校验应该放进内部包，而不是堆在 `main`。  
**示例**：`internal/cliinfo.Parse` 专门做解析。  
**为什么这么设计**：入口越薄，越容易复用与测试。

### 2.5 输出即“运行自检”

**概念**：入口输出关键环境信息，是最简单的自检。  
**示例**：打印 `GOOS/GOARCH`、运行时间。  
**为什么这么设计**：遇到线上问题时，这些信息最能帮助定位。

### 2.6 入口也应该“可测试”

**概念**：入口解析逻辑下沉后，就能写单元测试。  
**示例**：`cliinfo.Parse` 可以用不同参数做表驱动测试。  
**为什么这么设计**：入口错误往往最致命，测试能大幅降低风险。

### 2.7 命令帮助就是“自文档”

**概念**：`flag` 自动生成 `-h` 帮助信息。  
**示例**：`go run ... -h` 可以看到参数说明。  
**为什么这么设计**：让用户不用翻 README 就能理解使用方式。

### 2.8 退出码也是“接口”

**概念**：退出码是 CLI 给外部系统的信号。  
**示例**：参数错误返回非零，调用方脚本可以据此判断失败。  
**为什么这么设计**：让自动化流程具备可靠判断依据。

小结：一个好的入口，不只是能跑起来，更是“可解释、可校验、可自动化”。
配图建议：
- 一张“入口处理流程图”。
- 一张“错误传播与退出”示意图。

## 3. 完整代码示例（可运行）

示例包含：

1. `main` 入口。
2. 解析与校验逻辑 `internal/cliinfo`。
3. 输出运行环境与理由。

代码路径：`series/02/cmd/cli/main.go` 与 `series/02/internal/cliinfo/cliinfo.go`。

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

配图建议：
- 一张“main 与 internal 拆分”示意图。
- 一张“参数解析流程”图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/02/cmd/cli -name="小明" -lang=go
```

示例输出（节选）：

```
你好，小明！
你正在体验：GO
今天的结论：编译快、部署简单、并发模型清晰，适合做基础设施和服务端。
运行环境：darwin/amd64
Go 版本：go1.22.x
生成时间：2025-12-31T16:10:00+08:00
```

输出解读：入口既完成参数解析，也输出关键环境信息，这就是“最小可用”的入口模板，改成你自己的工具也很快。

你可以故意传入空的 `-name`，会看到错误提示并退出。这种“明确失败”比继续运行更安全，也更利于自动化脚本判断。

截图描述建议：
- 截一张输出图，突出 **参数解析后的结果**。
- 再截一张 `internal/cliinfo` 文件，强调入口下沉。

配图建议：
- 一张“入口输出结构图”。
- 一张“错误提示示例图”。

## 5. 常见坑 & 解决方案（必写）

1. **参数没有默认值**：导致启动就失败。  
   解决：为常用参数提供默认值。

2. **入口不校验**：错误延迟到运行中才暴露。  
   解决：在入口做最小校验。

3. **错误不输出到 stderr**：日志管道接不到。  
   解决：用 `os.Stderr` 输出错误。

4. **入口堆叠逻辑**：`main` 变复杂。  
   解决：解析逻辑下沉到 internal。

5. **参数命名混乱**：团队难统一使用。  
   解决：约定命名规范与文档说明。

6. **入口缺少自检信息**：排障困难。  
   解决：输出运行环境与版本信息。

补充建议：入口不仅是程序开始的地方，也是“失败最早可见”的地方。把错误在入口处说清楚，往往能省下 80% 的排查时间，也能降低线上误判。

配图建议：
- 一张“入口常见坑”清单图。
- 一张“参数规范示意图”。

## 6. 进阶扩展 / 思考题

1. 加入 `-config` 参数，读取配置文件并合并。
2. 把参数解析改成结构化输出（JSON）。
3. 增加 `--version` 输出当前版本号。
4. 思考：你们团队最常见的入口错误是什么？

补充建议：把入口的参数规范写进团队模板或脚手架，让每个新项目天然统一，也更省心、更稳定。

也可以把常用参数抽成小库，减少每次写入口的重复成本。

配图建议：
- 一张“入口演进路线”图。
- 一张“配置优先级”示意图。
