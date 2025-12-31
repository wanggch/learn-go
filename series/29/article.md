# encoding/json：最熟悉也是坑最多的包

你好，我是汪小成。很多人用 JSON 配置做项目启动，结果上线后才发现：字段拼错了也没人报错、数字精度被“悄悄改了”、明明没写的字段却被零值覆盖。你以为是配置文件的问题，其实是你对 `encoding/json` 的细节不够熟。本文会先给出环境与前提，再解释 JSON 的核心概念与设计逻辑，随后提供完整可运行代码、运行效果、常见坑与解决方案，最后给你几个进阶思考。

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
- 本篇目录：`series/29`。
- 示例入口：`series/29/cmd/jsonlab/main.go`。

### 1.2 运行命令

```bash
go run ./series/29/cmd/jsonlab
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go run ./series/29/cmd/jsonlab
```

### 1.3 前置知识

- 结构体与 tag 的基本用法。
- `io.Reader` / `io.Writer` 的基础概念（更易理解 Decoder）。

配图建议：
- 一张“配置流转图：JSON → 解码 → 校验 → 运行时配置”的流程图。
- 一张“默认行为 vs 严格模式”的对比表。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `Marshal / Unmarshal` 的基本模型

**概念**：`Marshal` 是把结构体编码成 JSON，`Unmarshal` 是把 JSON 解码进结构体。结构体 tag 决定字段名与行为。  
**示例**：`Port int `json:"port"`` 会从 `{"port":8080}` 取值。  
**为什么这么设计**：Go 的类型系统强，JSON 是弱类型，必须用结构体把“松散的 JSON”拉回“明确的领域模型”。

### 2.2 零值与默认值的边界

**概念**：JSON 缺失字段时，结构体里的字段会保持原值；但 JSON 明确给了 0 / "" / false，就会覆盖。  
**示例**：先设置 `Retries=3`，如果 JSON 没写 `retries`，仍然是 3；如果写了 `0`，就变成 0。  
**为什么这么设计**：解码器必须尊重输入；“不出现”和“显式给零值”是两种不同语义。

### 2.3 `Decoder` 与 `DisallowUnknownFields`

**概念**：默认情况下，未知字段会被忽略，这对配置是危险的。`DisallowUnknownFields` 能把拼写错误直接变成错误。  
**示例**：`"timeot"` 这种拼错字段，默认不会报错；开启严格模式就会立刻失败。  
**为什么这么设计**：Go 标准库默认“宽容输入”，但工程配置更需要“尽早失败”。

### 2.4 数字精度与 `UseNumber`

**概念**：默认解码到 `interface{}` 时，数字会变成 `float64`，大整数会丢精度。`UseNumber` 能保留原始字符串再手动转换。  
**示例**：`9007199254740993` 用 `float64` 会被四舍五入。  
**为什么这么设计**：JSON 本身不区分整型/浮点，标准库只能选择一个折中方案，`UseNumber` 提供更安全的分支。

### 2.5 解码与校验分层

**概念**：解码只保证 JSON 语法和类型形状，业务校验应该单独做。  
**示例**：`port` 是否为正数、`timeout` 是否能解析，需要在解码后统一校验。  
**为什么这么设计**：把“数据合法性”和“业务规则”分离，既方便复用配置解析器，也更容易写测试用例。

配图建议：
- 一张“JSON 字段缺失 vs 显式零值”的示意图。
- 一张“float64 精度丢失”的示意图（2^53 边界）。

## 3. 完整代码示例（可运行）

下面的示例实现一个“配置加载器”：

1. 先写入一份示例 JSON 配置文件；
2. 严格解码（拒绝未知字段 + 保留数字精度）；
3. 合并默认值 + 校验；
4. 输出运行时配置的最终视图。

代码路径：`series/29/cmd/jsonlab/main.go`。

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Limits struct {
	MaxConn int `json:"max_conn"`
	Burst   int `json:"burst"`
}

type RawConfig struct {
	App      string            `json:"app"`
	Port     int               `json:"port"`
	Timeout  string            `json:"timeout"`
	Retries  int               `json:"retries"`
	ID       json.Number       `json:"id"`
	Features []string          `json:"features"`
	Limits   Limits            `json:"limits"`
	Metadata map[string]string `json:"metadata"`
	Note     string            `json:"note,omitempty"`
}

type Config struct {
	App      string
	Addr     string
	Timeout  time.Duration
	Retries  int
	ID       int64
	Features []string
	Limits   Limits
	Metadata map[string]string
}

type OutputConfig struct {
	App      string            `json:"app"`
	Addr     string            `json:"addr"`
	Timeout  string            `json:"timeout"`
	Retries  int               `json:"retries"`
	ID       int64             `json:"id"`
	Features []string          `json:"features"`
	Limits   Limits            `json:"limits"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Note     string            `json:"note,omitempty"`
}

func main() {
	path := configPath()
	if err := writeSample(path); err != nil {
		panic(err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		panic(err)
	}

	fmt.Println("loaded config:")
	fmt.Printf("app=%s addr=%s timeout=%s retries=%d id=%d\n", cfg.App, cfg.Addr, cfg.Timeout, cfg.Retries, cfg.ID)
	fmt.Printf("features=%d limits=%+v metadata=%d\n", len(cfg.Features), cfg.Limits, len(cfg.Metadata))

	out := OutputConfig{
		App:      cfg.App,
		Addr:     cfg.Addr,
		Timeout:  cfg.Timeout.String(),
		Retries:  cfg.Retries,
		ID:       cfg.ID,
		Features: cfg.Features,
		Limits:   cfg.Limits,
		Metadata: cfg.Metadata,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println("\nencoded view:")
	fmt.Println(string(data))
}

func loadConfig(path string) (Config, error) {
	raw := RawConfig{
		Port:    8080,
		Timeout: "2s",
		Retries: 3,
		Features: []string{
			"search",
			"stats",
		},
		Limits: Limits{
			MaxConn: 100,
			Burst:   20,
		},
		Metadata: map[string]string{
			"env": "dev",
		},
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	if err := decodeStrict(file, &raw); err != nil {
		return Config{}, err
	}

	if raw.App == "" {
		return Config{}, errors.New("app is required")
	}
	if raw.Port <= 0 {
		return Config{}, errors.New("port must be positive")
	}

	timeout, err := time.ParseDuration(raw.Timeout)
	if err != nil {
		return Config{}, fmt.Errorf("invalid timeout: %w", err)
	}

	id, err := raw.ID.Int64()
	if err != nil {
		return Config{}, fmt.Errorf("invalid id: %w", err)
	}

	cfg := Config{
		App:      raw.App,
		Addr:     fmt.Sprintf(":%d", raw.Port),
		Timeout:  timeout,
		Retries:  raw.Retries,
		ID:       id,
		Features: raw.Features,
		Limits:   raw.Limits,
		Metadata: raw.Metadata,
	}

	return cfg, nil
}

func decodeStrict(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected extra JSON data")
		}
		return err
	}
	return nil
}

func configPath() string {
	if _, err := os.Stat(filepath.Join("series", "29")); err == nil {
		return filepath.Join("series", "29", "tmp", "config.json")
	}
	return filepath.Join("tmp", "config.json")
}

func writeSample(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data := []byte(`{
  "app": "order-api",
  "port": 9090,
  "timeout": "850ms",
  "retries": 5,
  "id": 9007199254740993,
  "features": ["search", "metrics", "circuit"],
  "limits": {
    "max_conn": 200,
    "burst": 50
  },
  "metadata": {
    "owner": "platform",
    "region": "cn-north-1"
  }
}`)

	return os.WriteFile(path, data, 0o644)
}
```

配图建议：
- 一张“解码流程图：读文件 → Decoder → 校验 → 转换”的流程图。
- 一张“RawConfig 与 Config 对比”的结构体对照图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/29/cmd/jsonlab
```

示例输出（节选）：

```
loaded config:
app=order-api addr=:9090 timeout=850ms retries=5 id=9007199254740993
features=3 limits={MaxConn:200 Burst:50} metadata=3

encoded view:
{
  "app": "order-api",
  "addr": ":9090",
  "timeout": "850ms",
  "retries": 5,
  "id": 9007199254740993,
  "features": [
    "search",
    "metrics",
    "circuit"
  ],
  "limits": {
    "max_conn": 200,
    "burst": 50
  },
  "metadata": {
    "env": "dev",
    "owner": "platform",
    "region": "cn-north-1"
  }
}
```

截图描述建议：
- 截一张终端输出图，突出 **大整数 ID 没有丢精度** 与 **严格解码成功**。
- 再截一张 `series/29/tmp/config.json` 文件内容图，强调 **默认值与文件配置合并** 的效果。

配图建议：
- 一张“输出前后对照”截图。
- 一张“JSON 文件与终端输出并排”的合成图。

## 5. 常见坑 & 解决方案（必写）

1. **未知字段被忽略**：默认不会报错，线上配置拼错字段很难发现。  
   解决：使用 `Decoder.DisallowUnknownFields()`，让错误尽早暴露。

2. **数字精度丢失**：大整数被当作 `float64`，精度被截断。  
   解决：`UseNumber` + `json.Number.Int64()`，或用字符串承载再手动转换。

3. **零值覆盖默认值**：JSON 写了 `0` 会覆盖掉你设置的默认值。  
   解决：区分“缺失”和“显式给 0”，必要时用指针或自定义类型区分。

4. **`omitempty` 误解**：很多人以为 `omitempty` 会影响解码，其实只影响编码。  
   解决：解码阶段不要指望 `omitempty`，应在解码后做默认值补全。

5. **时间字段解析失败**：`time.Duration` 直接解码会得到纳秒数，和预期字符串不一致。  
   解决：JSON 用字符串存储时间，解码后 `time.ParseDuration`。

6. **多余 JSON 内容**：配置文件末尾多了一段 JSON，默认不会察觉。  
   解决：解码后再 `Decode` 一次，确保遇到 `io.EOF`。

配图建议：
- 一张“常见坑清单”脑图。
- 一张“零值 vs 缺失字段”的示意图。

## 6. 进阶扩展 / 思考题

1. 如果你的配置来源不止一个（文件 + 环境变量 + flag），如何设计“优先级合并”？
2. 尝试用 `json.RawMessage` 延迟解析某些字段，看看能否让配置更灵活。
3. 为 `time.Duration` 写一个自定义类型，实现 `UnmarshalJSON`，避免重复解析逻辑。
4. 把示例扩展成“热更新配置”，思考如何保证线程安全与一致性。

配图建议：
- 一张“配置优先级金字塔”示意图。
- 一张“热更新流程”时序图。
