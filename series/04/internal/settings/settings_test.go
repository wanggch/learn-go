package settings

import (
	"testing"
	"time"
)

func TestApplyZero(t *testing.T) {
	cfg, err := ApplyZero(Config{ServiceName: "order-gateway"})
	if err != nil {
		t.Fatalf("ApplyZero returned error: %v", err)
	}
	if cfg.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %v, want %v", cfg.Timeout, 3*time.Second)
	}
	if cfg.Retry != 2 {
		t.Fatalf("Retry = %d, want %d", cfg.Retry, 2)
	}
}

func TestApplyZeroEmptyName(t *testing.T) {
	_, err := ApplyZero(Config{})
	if err == nil {
		t.Fatal("expected error for empty ServiceName")
	}
}
