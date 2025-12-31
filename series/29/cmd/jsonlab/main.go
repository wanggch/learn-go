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
