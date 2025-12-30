package cliinfo

import "testing"

func TestParse(t *testing.T) {
	cfg, err := Parse([]string{"-name", "小明", "-lang", "Go"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.Name != "小明" {
		t.Fatalf("name = %q, want %q", cfg.Name, "小明")
	}
	if cfg.Lang != "go" {
		t.Fatalf("lang = %q, want %q", cfg.Lang, "go")
	}
}

func TestParseEmptyName(t *testing.T) {
	_, err := Parse([]string{"-name", "  "})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}
