package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App     string
	Mode    string
	Region  string
	Workers int
}

func Load() Config {
	cfg := Config{
		App:     "sample",
		Mode:    "portal",
		Region:  "local",
		Workers: 4,
	}

	if v := strings.TrimSpace(os.Getenv("APP_NAME")); v != "" {
		cfg.App = v
	}
	if v := strings.TrimSpace(os.Getenv("APP_MODE")); v != "" {
		cfg.Mode = v
	}
	if v := strings.TrimSpace(os.Getenv("APP_REGION")); v != "" {
		cfg.Region = v
	}
	if v := strings.TrimSpace(os.Getenv("APP_WORKERS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Workers = n
		}
	}

	return cfg
}
