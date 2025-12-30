package config

import "testing"

func TestNew(t *testing.T) {
	cfg, err := New("service", "平台组")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if cfg.AppName != "service" {
		t.Fatalf("AppName = %q, want %q", cfg.AppName, "service")
	}
	if cfg.Owner != "平台组" {
		t.Fatalf("Owner = %q, want %q", cfg.Owner, "平台组")
	}
}

func TestNewDefaultOwner(t *testing.T) {
	cfg, err := New("service", " ")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if cfg.Owner != "团队" {
		t.Fatalf("Owner = %q, want %q", cfg.Owner, "团队")
	}
}

func TestNewEmptyAppName(t *testing.T) {
	_, err := New(" ", "平台组")
	if err == nil {
		t.Fatal("expected error for empty appName")
	}
}
