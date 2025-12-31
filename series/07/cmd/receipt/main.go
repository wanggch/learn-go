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
