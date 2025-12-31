# go test：把测试当开发工具

你好，我是汪小成。很多人把测试当“上线前的形式流程”：写几行断言就结束。但在 Go 里，`go test` 不只是验收工具，更是你写代码时的“安全护栏”。没有测试，你不敢改；测试写得好，你才敢重构。本文会先准备环境，再讲清 Go 测试的核心概念与设计逻辑，最后提供完整可运行示例、运行效果、常见坑与进阶思考。

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
- 本篇目录：`series/34`。
- 示例入口：`series/34/pricing`。

### 1.2 运行命令

```bash
go test ./series/34/pricing -v
```

沙盒环境若遇到构建缓存权限问题，使用：

```bash
GOCACHE=$(pwd)/.cache/go-build go test ./series/34/pricing -v
```

### 1.3 前置知识

- 函数与结构体的基础写法。
- 了解 error 的基本处理。

提示：测试不是“写完代码再补”，而是边写边验证。先把核心逻辑拆成小函数，再写测试覆盖它们，往往比最后补测试更省时间。

配图建议：
- 一张“测试作为安全护栏”的示意图。
- 一张“测试流程：写代码 → 写测试 → 运行”的流程图。

## 2. 核心概念解释（概念 → 示例 → 为什么这么设计）

### 2.1 `_test.go` 文件就是测试入口

**概念**：Go 只会执行 `*_test.go` 里的测试函数。  
**示例**：`pricing_test.go` 里的 `TestSum`。  
**为什么这么设计**：编译期区分测试与业务代码，保证正式构建不包含测试。

### 2.2 表驱动测试是默认范式

**概念**：用一组用例统一覆盖多个场景。  
**示例**：`TestFinalTotal` 一次性覆盖无折扣、百分比折扣、立减。  
**为什么这么设计**：可读性高，扩展用例成本低。

### 2.3 `t.Run` 让测试结构化

**概念**：子测试可以单独运行和统计。  
**示例**：`TestParseDiscount/percent`。  
**为什么这么设计**：便于定位失败用例，尤其在大批量测试时。

### 2.4 `t.Parallel` 让测试更快

**概念**：子测试并行执行能节省时间。  
**示例**：`TestParseDiscount` 的子用例并行跑。  
**为什么这么设计**：测试是开发反馈环，越快越好。

### 2.5 `t.Helper` 提升断言可读性

**概念**：自定义断言函数并标记为 helper。  
**示例**：`assertEqual` 会让报错定位到业务测试行。  
**为什么这么设计**：减少噪音，让失败提示更直观。

### 2.6 `errors.Is` 保证错误可识别

**概念**：错误要能被测试验证，而不是只靠字符串比对。  
**示例**：断言 `errors.Is(err, ErrInvalidDiscount)`。  
**为什么这么设计**：提高错误可维护性，避免字符串变更导致测试脆弱。

### 2.7 `-run / -count / -v` 是日常利器

**概念**：`go test` 支持筛选测试与控制缓存行为。  
**示例**：`go test -run TestFinalTotal -count=1 -v` 只跑指定测试且禁用缓存。  
**为什么这么设计**：开发中你需要更快的反馈，而不是每次全量跑完。

### 2.8 `testdata` 与“金文件”思路

**概念**：复杂输入可以放在 `testdata/`，或用输出文件作为基准。  
**示例**：解析大 JSON、渲染模板输出等，适合用文件对比。  
**为什么这么设计**：把复杂样例从代码里剥离，测试更清晰、维护更方便。

配图建议：
- 一张“表驱动测试结构图”。
- 一张“并行测试”示意图。

## 3. 完整代码示例（可运行）

示例包含：

1. 简单的定价逻辑（总价 + 折扣）。
2. 解析折扣码并校验合法性。
3. 表驱动测试 + 子测试 + 并行测试。

代码路径：`series/34/pricing`。

```go
package pricing

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Item struct {
	Name  string
	Price int
}

type Discount struct {
	Kind  string
	Value int
}

var ErrInvalidDiscount = errors.New("invalid discount")

func Sum(items []Item) int {
	total := 0
	for _, item := range items {
		total += item.Price
	}
	return total
}

func FinalTotal(items []Item, code string) (int, error) {
	total := Sum(items)
	disc, err := ParseDiscount(code)
	if err != nil {
		return 0, err
	}

	switch disc.Kind {
	case "none":
		return total, nil
	case "percent":
		return total * (100 - disc.Value) / 100, nil
	case "minus":
		if disc.Value >= total {
			return 0, nil
		}
		return total - disc.Value, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrInvalidDiscount, code)
	}
}

func ParseDiscount(code string) (Discount, error) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return Discount{Kind: "none"}, nil
	}

	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "OFF"):
		value, ok := parseNumber(strings.TrimPrefix(upper, "OFF"))
		if !ok || value <= 0 || value > 90 {
			return Discount{}, fmt.Errorf("%w: %q", ErrInvalidDiscount, code)
		}
		return Discount{Kind: "percent", Value: value}, nil
	case strings.HasPrefix(upper, "MINUS"):
		value, ok := parseNumber(strings.TrimPrefix(upper, "MINUS"))
		if !ok || value <= 0 {
			return Discount{}, fmt.Errorf("%w: %q", ErrInvalidDiscount, code)
		}
		return Discount{Kind: "minus", Value: value}, nil
	default:
		return Discount{}, fmt.Errorf("%w: %q", ErrInvalidDiscount, code)
	}
}

func parseNumber(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}
```

```go
package pricing

import (
	"errors"
	"testing"
)

func TestParseDiscount(t *testing.T) {
	cases := []struct {
		name    string
		code    string
		want    Discount
		wantErr bool
	}{
		{name: "empty", code: "", want: Discount{Kind: "none"}},
		{name: "percent", code: "OFF10", want: Discount{Kind: "percent", Value: 10}},
		{name: "percent lowercase", code: "off20", want: Discount{Kind: "percent", Value: 20}},
		{name: "minus", code: "MINUS500", want: Discount{Kind: "minus", Value: 500}},
		{name: "bad percent", code: "OFF0", wantErr: true},
		{name: "bad percent high", code: "OFF95", wantErr: true},
		{name: "bad minus", code: "MINUS0", wantErr: true},
		{name: "bad format", code: "HELLO", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseDiscount(tc.code)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if !errors.Is(err, ErrInvalidDiscount) {
					t.Fatalf("expected ErrInvalidDiscount, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, got, tc.want)
		})
	}
}

func TestFinalTotal(t *testing.T) {
	items := []Item{{Name: "latte", Price: 1200}, {Name: "bagel", Price: 800}}

	cases := []struct {
		name    string
		code    string
		want    int
		wantErr bool
	}{
		{name: "no discount", code: "", want: 2000},
		{name: "percent", code: "OFF10", want: 1800},
		{name: "minus", code: "MINUS500", want: 1500},
		{name: "minus over", code: "MINUS3000", want: 0},
		{name: "invalid", code: "BAD", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := FinalTotal(items, tc.code)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, got, tc.want)
		})
	}
}

func TestSum(t *testing.T) {
	items := []Item{{Name: "a", Price: 10}, {Name: "b", Price: 20}}
	got := Sum(items)
	assertEqual(t, got, 30)
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}
```

说明：示例用 `t.Run` 做结构化测试，核心用例用表驱动覆盖，`t.Parallel` 加速子测试。

实践建议：先写 2～3 个关键用例，保证核心路径不出错，再用表驱动补齐边界；测试命名尽量“场景化”，失败时能一眼看懂。保持小步提交，测试更易维护也更稳定、更可控、更安心。

配图建议：
- 一张“测试文件结构示意图”。
- 一张“子测试执行流程”示意图。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go test ./series/34/pricing -v
```

示例输出（节选）：

```
=== RUN   TestParseDiscount
=== RUN   TestParseDiscount/empty
=== PAUSE TestParseDiscount/empty
=== RUN   TestParseDiscount/percent
=== PAUSE TestParseDiscount/percent
=== RUN   TestParseDiscount/percent_lowercase
=== PAUSE TestParseDiscount/percent_lowercase
=== RUN   TestParseDiscount/minus
=== PAUSE TestParseDiscount/minus
=== RUN   TestParseDiscount/bad_percent
=== PAUSE TestParseDiscount/bad_percent
=== RUN   TestParseDiscount/bad_percent_high
=== PAUSE TestParseDiscount/bad_percent_high
=== RUN   TestParseDiscount/bad_minus
=== PAUSE TestParseDiscount/bad_minus
=== RUN   TestParseDiscount/bad_format
=== PAUSE TestParseDiscount/bad_format
=== CONT  TestParseDiscount/empty
=== CONT  TestParseDiscount/minus
=== CONT  TestParseDiscount/percent_lowercase
=== CONT  TestParseDiscount/percent
=== CONT  TestParseDiscount/bad_format
=== CONT  TestParseDiscount/bad_percent_high
=== CONT  TestParseDiscount/bad_percent
=== CONT  TestParseDiscount/bad_minus
--- PASS: TestParseDiscount (0.00s)
=== RUN   TestFinalTotal
=== RUN   TestFinalTotal/no_discount
=== RUN   TestFinalTotal/percent
=== RUN   TestFinalTotal/minus
=== RUN   TestFinalTotal/minus_over
=== RUN   TestFinalTotal/invalid
--- PASS: TestFinalTotal (0.00s)
=== RUN   TestSum
--- PASS: TestSum (0.00s)
PASS
ok   	learn-go/series/34/pricing	0.009s
```

输出解读：你能看到 `PAUSE/CONT` 代表并行子测试的调度过程，`PASS` 则说明整体用例通过。对比单测时间变化，可以直观判断并行是否有效；如果只关心某个用例，用 `-run` 精确过滤会更高效。

截图描述建议：
- 截一张终端输出图，突出 **子测试并行执行**（PAUSE/CONT）。
- 再截一张 PASS 汇总图，强调测试总耗时。

配图建议：
- 一张“并行测试时间轴”示意图。
- 一张“测试反馈回路”示意图。

## 5. 常见坑 & 解决方案（必写）

1. **测试写在错误目录**：`*_test.go` 不在同包目录。  
   解决：保持测试文件与被测文件同目录。

2. **错误比较用字符串**：错误信息改动就全挂。  
   解决：用 `errors.Is` 或自定义错误类型。

3. **子测试未隔离**：共享变量导致并发互相污染。  
   解决：每个子测试复制数据，或避免共享状态。

4. **测试过慢**：一跑就几十秒，开发者懒得跑。  
   解决：拆分测试集，使用 `t.Parallel` 或 `-run` 聚焦。

5. **只测“成功路径”**：错误场景没人覆盖。  
   解决：用表驱动补齐边界用例。

6. **测试和业务逻辑耦合**：测试脆弱难维护。  
   解决：保持测试关注输入输出，而不是内部实现细节。

配图建议：
- 一张“测试坑位清单”脑图。
- 一张“并发污染示意图”。

## 6. 进阶扩展 / 思考题

1. 尝试加入 `TestMain`，在测试前后做统一初始化。
2. 给 `go test` 加上 `-run` 和 `-count=1`，观察运行差异。
3. 使用 `-cover` 生成覆盖率，思考哪些函数最该补测。
4. 写一个 `Benchmark`，比较不同折扣解析方式的性能。

补充建议：如果你的项目规模较大，建议把关键包的测试接入 CI，每次提交自动跑一遍，保证长期可维护。

配图建议：
- 一张“覆盖率热力图”示意图。
- 一张“基准测试曲线”示意图。
