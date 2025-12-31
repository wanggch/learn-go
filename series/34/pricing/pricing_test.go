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
