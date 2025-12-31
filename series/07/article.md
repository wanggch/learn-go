# 函数与多返回值：Go 是如何对待错误的

大家好，我是汪小成。上线一周的结算服务，偶发出现“部分订单少扣钱”。日志只有一句“process ok”，排查发现函数只返回了 bool，错误细节丢失在内部。Go 里函数与多返回值是第一等公民：错误不靠异常，而是靠显式返回；`value, ok` 模式让你知道有没有命中；资源释放可以作为返回值带出去。本文带你把这些模式用顺，把“错误不可见”“信息散落”这类痛点根治。

## 目录

1. 环境准备 / 前提知识
2. 核心概念解释（概念 → 示例 → 设计原因）
3. 完整代码示例（可运行）
4. 运行效果 + 截图描述
5. 常见坑 & 解决方案
6. 进阶扩展 / 思考题

## 1. 环境准备 / 前提知识

### 1.1 必备版本与工具

- Go 1.22+，命令 `go version` 确认。
- Git + gofmt/go vet（Go 自带）；推荐 VS Code + Go 扩展或 Goland。

### 1.2 项目结构与运行

- `go.work` 管理多模块；本篇目录：`series/07`。
- 示例入口：`series/07/cmd/receipt/main.go`。
- 运行：`go run ./series/07/cmd/receipt`；如沙盒不允许默认缓存，可 `GOCACHE=$(pwd)/.cache/go-build go run ./series/07/cmd/receipt`。

### 1.3 前置知识

- 知道 `if err := ...; err != nil { return ... }` 的守卫式写法。
- 理解切片、map 零值；熟悉基本字符串拆分与 switch。

配图建议：目录树截图突出 `series/07`；一张“函数签名”拆解示意（参数、返回值标注）。

## 2. 核心概念解释

### 2.1 Go 的错误哲学：返回值，而非异常

- Go 选择“显式返回 error”，让调用方必须处理或传播。
- 典型模式：`value, err := f()`；错误即数据的一部分。
- 设计原因：减少隐藏控制流，调试时堆栈更短、更可预期。

### 2.2 多返回值的常见形态

- `value, error`：主流形态，失败时 value 往往为零值。
- `value, ok`：map 查找、类型断言、缓存命中，用 bool 标记是否找到。
- `value1, value2, error`：需要额外信息时（如折扣金额 + 实收金额）。
- `cleanup, error`：返回一个关闭/撤销函数，调用方决定何时清理。

为什么这么设计：多返回值让“结果 + 元信息”并列出现，避免塞进结构体或隐式全局。

### 2.3 短变量声明与作用域

- `if v, err := f(); err != nil { ... }` 把 err 限定在局部，减少命名冲突。
- 同名变量遮蔽：注意 if/for 内部的 `:=` 会覆盖外层同名变量。
- 设计原因：鼓励把错误处理靠近调用点，函数体更平铺。

### 2.4 错误包装与上下文

- `fmt.Errorf("parse: %w", err)` 通过 `%w` 传递链路。
- 让上层知道“在哪失败”+“为什么失败”，而不只是底层报错。
- 设计原因：error 是值，可被组合、传递、匹配。

### 2.5 何时用多返回值 vs 结构体

- 多返回值适合**临时组合**、调用链短的场景。
- 长期存在的配置/状态，优先用 struct，字段命名更清晰。
- 设计原因：签名即文档，短小函数看见签名就知道意图。

配图建议：表格列出四种返回模式，附小代码片段；流程图展示错误从底层往上层包裹传播。

## 3. 完整代码示例（可复制运行）

场景：实现一个“订单计费演示器”，展示以下模式：

- `parseOrders` 返回 (结果切片, 错误切片)。
- `tierForUser` 返回 (value, ok)。
- `chargeForOrder` 返回 (final, discount, error)。
- `writeReceipt` 返回 (cleanup, error) 供调用方决定资源释放。

入口：`series/07/cmd/receipt/main.go`。

```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Order struct {
	ID     string
	User   string
	Amount float64
	Status string
}

type Receipt struct {
	Processed   int
	Skipped     int
	TotalAmount float64
	Discount    float64
	FinalAmount float64
	MissingTier []string
}

var seedOrders = `o-1001,alice,120.5,paid
o-1002,bob,80.0,paid
o-1003,carol,0,cancelled
o-1004,alice,200.0,paid
o-1005,dave,180.5,paid
o-1006,unknown,75.5,paid
invalid-line-here`

func main() {
	fmt.Println("=== 函数与多返回值演示 ===")

	orders, parseErrs := parseOrders(seedOrders)
	if len(parseErrs) > 0 {
		fmt.Println("解析阶段发现问题：")
		for _, err := range parseErrs {
			fmt.Printf("  - %v\n", err)
		}
	}

	tierByUser := map[string]string{
		"alice": "gold",
		"bob":   "silver",
		"carol": "silver",
		"dave":  "bronze",
	}

	receipt, processErrs := processOrders(orders, tierByUser)
	if len(processErrs) > 0 {
		fmt.Println("\n处理阶段发现问题：")
		for _, err := range processErrs {
			fmt.Printf("  - %v\n", err)
		}
	}

	fmt.Printf("\n汇总：处理 %d 条，跳过 %d 条，总额 %.2f，折扣 %.2f，实收 %.2f\n",
		receipt.Processed, receipt.Skipped, receipt.TotalAmount, receipt.Discount, receipt.FinalAmount)

	if len(receipt.MissingTier) > 0 {
		fmt.Printf("未找到会员等级的用户：%s\n", strings.Join(receipt.MissingTier, ", "))
	}
}

// parseOrders 展示“值 + 错误切片”返回，便于收集多条错误。
func parseOrders(raw string) ([]Order, []error) {
	reader := strings.NewReader(raw)
	scanner := bufio.NewScanner(reader)

	var orders []Order
	var errs []error
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		order, err := parseOrderLine(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", lineNo, err))
			continue
		}
		orders = append(orders, order)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		errs = append(errs, fmt.Errorf("scan: %w", scanErr))
	}

	return orders, errs
}

// parseOrderLine 展示典型“值 + error”返回，调用处根据错误选择跳过或中断。
func parseOrderLine(line string) (Order, error) {
	parts := strings.Split(line, ",")
	if len(parts) != 4 {
		return Order{}, fmt.Errorf("字段数量应为 4，实际 %d", len(parts))
	}

	amount, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return Order{}, fmt.Errorf("金额解析失败: %w", err)
	}
	if amount < 0 {
		return Order{}, errors.New("金额不可为负")
	}

	return Order{
		ID:     strings.TrimSpace(parts[0]),
		User:   strings.TrimSpace(parts[1]),
		Amount: amount,
		Status: strings.TrimSpace(parts[3]),
	}, nil
}

// processOrders 展示多返回值：汇总结果 + 错误列表。
func processOrders(orders []Order, tierByUser map[string]string) (Receipt, []error) {
	var receipt Receipt
	var errs []error

	for _, order := range orders {
		tier, ok := tierForUser(order.User, tierByUser) // 典型 value, ok 模式
		if !ok {
			receipt.MissingTier = append(receipt.MissingTier, order.User)
			receipt.Skipped++
			errs = append(errs, fmt.Errorf("order %s: 未找到用户 %s 的会员等级", order.ID, order.User))
			continue
		}

		final, discount, err := chargeForOrder(order, tier) // 多返回值
		if err != nil {
			receipt.Skipped++
			errs = append(errs, fmt.Errorf("order %s: %w", order.ID, err))
			continue
		}

		receipt.Processed++
		receipt.TotalAmount += order.Amount
		receipt.Discount += discount
		receipt.FinalAmount += final
	}

	return receipt, errs
}

// tierForUser 演示 map 查找的“值 + ok”模式。
func tierForUser(user string, tiers map[string]string) (string, bool) {
	tier, ok := tiers[user]
	return tier, ok
}

// chargeForOrder 展示多返回值：实收金额、折扣金额、错误。
func chargeForOrder(order Order, tier string) (final float64, discount float64, err error) {
	if order.Status != "paid" {
		return 0, 0, fmt.Errorf("状态为 %s，跳过计费", order.Status)
	}
	if order.Amount == 0 {
		return 0, 0, errors.New("金额为 0，跳过计费")
	}

	rate, err := discountRate(tier)
	if err != nil {
		return 0, 0, err
	}

	discount = order.Amount * rate
	final = order.Amount - discount
	return final, discount, nil
}

// discountRate 演示 switch 返回值 + error。
func discountRate(tier string) (float64, error) {
	switch strings.ToLower(tier) {
	case "gold":
		return 0.15, nil
	case "silver":
		return 0.08, nil
	case "bronze":
		return 0.03, nil
	default:
		return 0, fmt.Errorf("未知等级 %q", tier)
	}
}

// Example of writing output to a file, returning a cleanup func (multi-return).
// Not used in main but kept for demonstration.
func writeReceipt(path string, r Receipt) (cleanup func() error, err error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	cleanup = func() error {
		return f.Close()
	}

	if _, err := fmt.Fprintf(f, "processed=%d skipped=%d final=%.2f\n", r.Processed, r.Skipped, r.FinalAmount); err != nil {
		return cleanup, err
	}
	return cleanup, nil
}
```

代码里的多返回值亮点：

- `parseOrders`：一次返回结果和错误切片，适合“尽量多处理，错误累积”。
- `tierForUser`：标准 `value, ok`，调用方自己决定缺失时的策略。
- `chargeForOrder`：同时返回最终金额和折扣金额，避免重复计算。
- `writeReceipt`：返回 `cleanup` 函数，让调用方控制资源释放时机。

## 4. 运行效果 + 截图描述

运行命令：

```bash
go run ./series/07/cmd/receipt
```

节选输出：

```
=== 函数与多返回值演示 ===
解析阶段发现问题：
  - line 7: 字段数量应为 4，实际 1

处理阶段发现问题：
 - order o-1003: 状态为 cancelled，跳过计费
 - order o-1006: 未找到用户 unknown 的会员等级

汇总：处理 4 条，跳过 2 条，总额 581.00，折扣 59.89，实收 521.11
未找到会员等级的用户：unknown
```

截图建议：

- 终端运行截图，标出“解析阶段”“处理阶段”两组错误，展示错误上下文。
- 一个多返回值拆解示意：`final, discount, err := chargeForOrder(...)`，标注每个值的含义。
- 一个流程图：订单数据 → 解析 → 会员等级查找 → 计费 → 汇总的函数调用链。

## 5. 常见坑 & 解决方案（必看）

1. **忽略 error**：`res, _ := f()` 把错误吞掉。解决：必须处理或向上返回；确实要忽略时加注释说明。
2. **同名变量遮蔽**：`if err := f(); err != nil { ... }` 里的 err 不等于外层 err。解决：必要时拆分声明或改名。
3. **返回零值但继续使用**：函数返回零值 + error，调用方忘记检查 error。解决：先判断 err，再用 value；或返回指针 nil 明示不可用。
4. **滥用命名返回值 + naked return**：大函数用裸返回易读性差。解决：仅在很短的函数里使用裸返回，否则显式返回。
5. **多返回值顺序不清晰**：`func f() (int, bool, string)` 顺序让人猜。解决：把“核心值”放前面，布尔/辅助值靠后；必要时换成 struct。
6. **错误信息缺上下文**：`return err` 让上层不知道哪一步坏了。解决：`fmt.Errorf("parse order %s: %w", id, err)` 包含场景。
7. **资源释放遗漏**：返回文件句柄后忘记 close。解决：用 `cleanup` 返回值或在调用点 `defer f.Close()`，保持释放路径靠近获取点。

配图建议：错误包装前后对比（缺上下文 vs 包含上下文）；返回值顺序示例表。

## 6. 进阶扩展 / 思考题

- 把 `parseOrders` 改成表驱动测试，覆盖：字段缺失、金额为负、扫描错误。
- 设计一个“可重试”的函数签名：`result, retries, err := Do(ctx, req)`，让调用方知道重试次数。
- 让 `processOrders` 返回 `(Receipt, error)`，把错误列表封装成自定义类型，思考 pros/cons。
- 实现 `writeReceipt` 的实际调用，用 `cleanup` 关闭文件，并处理写入错误。
- 尝试引入泛型：写一个 `Lookup[K, V](map[K]V, key K) (V, bool)`，再比较与内置 map 语法的可读性。
- 给 `chargeForOrder` 加表驱动测试，覆盖状态异常、未知等级、正常路径。

配图建议：表驱动测试用例表；流程时序图展示函数调用与错误传播。

---

函数与多返回值是 Go 错误处理的基石。把“值 + error”“值 + ok”“值1 + 值2 + error”“cleanup + error”用对，就能在函数签名里写出意图，让错误透明、上下文完整、资源释放可控。跑一遍示例，再回头审视你项目里的函数签名，把隐藏的返回值和错误处理补齐，线上“玄学”问题会少一大截。 下一篇我们会聊 struct 与方法，看看 Go 如何用组合实现面向对象风格。 
