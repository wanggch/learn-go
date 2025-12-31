# flag / env / config：配置是工程问题

你好，我是汪小成。很多人写 CLI 或服务时，配置要么散落在代码里，要么被 flag 和 env 混成一锅粥：谁覆盖谁？默认值写哪？上线后发现“配置没生效”，排查半天才发现优先级搞反了。配置不是小事，它决定了程序的可维护性和可控性。本文会先准备环境，再讲清 flag/env/config 的合并策略与设计原因，最后给出完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/33`。
- 示例入口：`series/33/cmd/configlab/main.go`。

### 1.2 运行命令

```bash
APP_PORT=7070 APP_LOG_LEVEL=debug go run ./series/33/cmd/configlab -timeout=500ms -feature-x=false
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
APP_PORT=7070 APP_LOG_LEVEL=debug GOCACHE=$(pwd)/.cache/go-build go run ./series/33/cmd/configlab -timeout=500ms -feature-x=false
```

### 1.3 前置知识

- `flag` 包基础用法。
- `encoding/json` 的基础解析。

提示：示例会自动生成 `series/33/tmp/config.json`，方便你观察“文件层”的作用，读起来更清晰、更稳、更好。

配图建议：
- 一张“默认值 → 配置文件 → 环境变量 → Flag”的优先级金字塔图。
- 一张“配置生命周期”示意图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 配置合并的黄金顺序

**概念**：默认值 < 配置文件 < 环境变量 < flag。  
**示例**：`APP_PORT` 覆盖配置文件端口，但 `-port` 仍可再次覆盖。  
**为什么这么设计**：文件是“静态基线”，env 适合部署注入，flag 适合临时覆盖。

### 2.2 “显式写 0” 与 “没写”不一样

**概念**：配置文件里 `port: 0` 和缺失 `port` 是两种语义。  
**示例**：使用指针字段判断是否出现。  
**为什么这么设计**：避免零值误覆盖默认值。

### 2.3 环境变量适合运维，不适合复杂结构

**概念**：env 只适合简单类型（字符串、数字、布尔）。  
**示例**：复杂列表/对象还是交给配置文件。  
**为什么这么设计**：减少运维成本与解析复杂度。

### 2.4 Flag 应该只覆盖“临时需求”

**概念**：flag 更适合本地调试或灰度开关。  
**示例**：临时把 `timeout` 改成 500ms。  
**为什么这么设计**：flag 可读性低、难审计，不宜作为长期配置。

### 2.5 校验与规范化不可省

**概念**：合并后的配置要做校验和规范化。  
**示例**：`timeout` 必须能解析为 `time.Duration`。  
**为什么这么设计**：把错误尽早发现，而不是等到运行时报错。

### 2.6 “配置来源”要可追溯

**概念**：输出最终配置时最好标记来源。  
**示例**：`port` 来自 env，`timeout` 来自 flag。  
**为什么这么设计**：排查“配置不生效”时非常关键。

### 2.7 配置要“分层负责”

**概念**：默认值属于代码层，配置文件属于项目层，环境变量属于部署层。  
**示例**：开发机用文件，生产用 env 注入敏感配置。  
**为什么这么设计**：明确责任边界，避免团队之间互相踩线。

### 2.8 命名规范决定可维护性

**概念**：同一配置在不同入口的名字要保持一致。  
**示例**：`log_level`、`APP_LOG_LEVEL`、`-log-level` 三者应成对映射。  
**为什么这么设计**：减少认知负担，也方便写自动化工具。

小结：配置不是“写完就算”，而是一套约定。你要明确“谁能改、改哪里、改完如何验证”，否则配置越多越乱。

配图建议：
- 一张“配置合并流程图”。
- 一张“来源追踪”示意图。

## 3. 完整代码示例（可运行）

示例包含：

1. 默认配置。
2. JSON 配置文件。
3. 环境变量覆盖。
4. Flag 覆盖与来源追踪。

代码路径：`series/33/cmd/configlab/main.go`。

```go
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type fileConfig struct {
	App      *string `json:"app"`
	Port     *int    `json:"port"`
	Timeout  *string `json:"timeout"`
	LogLevel *string `json:"log_level"`
	FeatureX *bool   `json:"feature_x"`
}

type rawConfig struct {
	App      string
	Port     int
	Timeout  string
	LogLevel string
	FeatureX bool
}

type sources struct {
	App      string `json:"app"`
	Port     string `json:"port"`
	Timeout  string `json:"timeout"`
	LogLevel string `json:"log_level"`
	FeatureX string `json:"feature_x"`
}

type resolved struct {
	App      string  `json:"app"`
	Port     int     `json:"port"`
	Addr     string  `json:"addr"`
	Timeout  string  `json:"timeout"`
	LogLevel string  `json:"log_level"`
	FeatureX bool    `json:"feature_x"`
	Sources  sources `json:"sources"`
}

func main() {
	configPath := findConfigPath(os.Args[1:], defaultConfigPath())
	if err := ensureSampleConfig(configPath); err != nil {
		panic(err)
	}

	cfg, src, err := loadConfig(configPath, os.Args[1:])
	if err != nil {
		panic(err)
	}

	out := resolved{
		App:      cfg.App,
		Port:     cfg.Port,
		Addr:     fmt.Sprintf(":%d", cfg.Port),
		Timeout:  cfg.Timeout,
		LogLevel: cfg.LogLevel,
		FeatureX: cfg.FeatureX,
		Sources:  src,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println("effective config:")
	fmt.Println(string(data))
}

func defaultConfig() rawConfig {
	return rawConfig{
		App:      "demo-api",
		Port:     8080,
		Timeout:  "1s",
		LogLevel: "info",
		FeatureX: false,
	}
}

func defaultSources() sources {
	return sources{
		App:      "default",
		Port:     "default",
		Timeout:  "default",
		LogLevel: "default",
		FeatureX: "default",
	}
}

func loadConfig(path string, args []string) (rawConfig, sources, error) {
	cfg := defaultConfig()
	src := defaultSources()

	if err := applyFile(path, &cfg, &src); err != nil {
		return rawConfig{}, sources{}, err
	}
	if err := applyEnv(&cfg, &src); err != nil {
		return rawConfig{}, sources{}, err
	}
	if err := applyFlags(args, path, &cfg, &src); err != nil {
		return rawConfig{}, sources{}, err
	}
	if err := validateConfig(cfg); err != nil {
		return rawConfig{}, sources{}, err
	}

	return cfg, src, nil
}

func applyFile(path string, cfg *rawConfig, src *sources) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	dec.DisallowUnknownFields()
	var fc fileConfig
	if err := dec.Decode(&fc); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected extra json")
		}
		return err
	}

	applyFileField(fc.App, &cfg.App, &src.App, "config")
	applyFileField(fc.Port, &cfg.Port, &src.Port, "config")
	applyFileField(fc.Timeout, &cfg.Timeout, &src.Timeout, "config")
	applyFileField(fc.LogLevel, &cfg.LogLevel, &src.LogLevel, "config")
	applyFileField(fc.FeatureX, &cfg.FeatureX, &src.FeatureX, "config")

	return nil
}

func applyFileField[T any](val *T, dst *T, src *string, from string) {
	if val == nil {
		return
	}
	*dst = *val
	*src = from
}

func applyEnv(cfg *rawConfig, src *sources) error {
	if v, ok := os.LookupEnv("APP_NAME"); ok {
		cfg.App = v
		src.App = "env"
	}
	if v, ok := os.LookupEnv("APP_PORT"); ok {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid APP_PORT: %w", err)
		}
		cfg.Port = port
		src.Port = "env"
	}
	if v, ok := os.LookupEnv("APP_TIMEOUT"); ok {
		cfg.Timeout = v
		src.Timeout = "env"
	}
	if v, ok := os.LookupEnv("APP_LOG_LEVEL"); ok {
		cfg.LogLevel = v
		src.LogLevel = "env"
	}
	if v, ok := os.LookupEnv("APP_FEATURE_X"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid APP_FEATURE_X: %w", err)
		}
		cfg.FeatureX = b
		src.FeatureX = "env"
	}
	return nil
}

func applyFlags(args []string, configPath string, cfg *rawConfig, src *sources) error {
	fs := flag.NewFlagSet("configlab", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	_ = fs.String("config", configPath, "config file path")
	app := fs.String("app", cfg.App, "app name")
	port := fs.Int("port", cfg.Port, "listen port")
	timeout := fs.String("timeout", cfg.Timeout, "request timeout")
	logLevel := fs.String("log-level", cfg.LogLevel, "log level")
	featureX := fs.Bool("feature-x", cfg.FeatureX, "enable feature x")

	if err := fs.Parse(args); err != nil {
		return err
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	if set["app"] {
		cfg.App = *app
		src.App = "flag"
	}
	if set["port"] {
		cfg.Port = *port
		src.Port = "flag"
	}
	if set["timeout"] {
		cfg.Timeout = *timeout
		src.Timeout = "flag"
	}
	if set["log-level"] {
		cfg.LogLevel = *logLevel
		src.LogLevel = "flag"
	}
	if set["feature-x"] {
		cfg.FeatureX = *featureX
		src.FeatureX = "flag"
	}

	return nil
}

func validateConfig(cfg rawConfig) error {
	if strings.TrimSpace(cfg.App) == "" {
		return errors.New("app is required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return errors.New("port must be 1-65535")
	}
	if _, err := time.ParseDuration(cfg.Timeout); err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	if !validLogLevel(cfg.LogLevel) {
		return fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}
	return nil
}

func validLogLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func defaultConfigPath() string {
	if _, err := os.Stat(filepath.Join("series", "33")); err == nil {
		return filepath.Join("series", "33", "tmp", "config.json")
	}
	return filepath.Join("tmp", "config.json")
}

func findConfigPath(args []string, def string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-config" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "-config=") {
			return strings.TrimPrefix(arg, "-config=")
		}
	}
	return def
}

func ensureSampleConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data := []byte(`{
  "app": "billing-api",
  "port": 9090,
  "timeout": "900ms",
  "log_level": "warn",
  "feature_x": true
}`)

	return os.WriteFile(path, data, 0o644)
}
```

说明：示例把最终配置和来源一起打印出来，便于确认“谁覆盖了谁”。

配图建议：
- 一张“输出结果 + 来源标注”的截图。
- 一张“配置合并顺序”示意图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
APP_PORT=7070 APP_LOG_LEVEL=debug go run ./series/33/cmd/configlab -timeout=500ms -feature-x=false
```

示例输出（节选）：

```
effective config:
{
  "app": "billing-api",
  "port": 7070,
  "addr": ":7070",
  "timeout": "500ms",
  "log_level": "debug",
  "feature_x": false,
  "sources": {
    "app": "config",
    "port": "env",
    "timeout": "flag",
    "log_level": "env",
    "feature_x": "flag"
  }
}
```

输出解读：本次运行中，`app` 来自配置文件，`port` 与 `log_level` 来自环境变量，`timeout` 与 `feature_x` 来自 flag。你可以尝试改动任意一层，观察最终输出如何变化，这种“可观察性”是配置工程化的关键，也最直观。

截图描述建议：
- 截一张终端输出图，突出 **来源来源** 与 **最终值** 的一一对应。
- 截一张 `series/33/tmp/config.json` 文件图，强调文件与运行时的覆盖关系。

配图建议：
- 一张“覆盖关系箭头图”。
- 一张“配置来源矩阵表”。

## 5. 常见坑 & 解决方案（必写）

1. **优先级混乱**：env 与 flag 谁覆盖谁搞不清。  
   解决：统一流程，固定顺序：默认 < 文件 < env < flag。

2. **零值覆盖默认值**：配置里 `0` 被误认为“缺失”。  
   解决：使用指针字段判断是否出现。

3. **配置文件拼错字段**：默认解码会忽略，导致配置无效。  
   解决：`DisallowUnknownFields` 让错误尽早暴露。

4. **缺少校验**：配置合法性不检查，运行时才崩。  
   解决：统一做 `validateConfig`。

5. **环境变量难追踪**：线上排查不知道来自哪里。  
   解决：输出来源或日志打印来源。

6. **命名不统一**：env/flag/配置名不一致，运维混乱。  
   解决：建立统一的命名约定与文档。

补充建议：把最终配置和来源打印在启动日志里，并在发布系统中保存一份“配置快照”。当出现问题时，能迅速定位是谁改的、改了什么。

配图建议：
- 一张“配置坑位清单”思维导图。
- 一张“命名约定对照表”。

## 6. 进阶扩展 / 思考题

1. 试着支持 `YAML` 配置，并比较解析复杂度。
2. 增加“热更新配置”，如何保证线程安全？
3. 加入 `config dump` 子命令，把当前配置输出为 JSON。
4. 设计一套“配置变更审计”机制，记录谁改了什么。

配图建议：
- 一张“热更新流程”示意图。
- 一张“配置审计日志”示例图。
