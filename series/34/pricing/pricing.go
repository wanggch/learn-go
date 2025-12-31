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
