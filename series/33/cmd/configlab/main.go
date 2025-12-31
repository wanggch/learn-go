package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type fileConfig struct {
	App      *string `json:"app"`
	Port     *int    `json:"port"`
	Timeout  *string `json:"timeout"`
	LogLevel *string `json:"log_level"`
	FeatureX *bool   `json:"feature_x"`
}

type rawConfig struct {
	App      string
	Port     int
	Timeout  string
	LogLevel string
	FeatureX bool
}

type sources struct {
	App      string `json:"app"`
	Port     string `json:"port"`
	Timeout  string `json:"timeout"`
	LogLevel string `json:"log_level"`
	FeatureX string `json:"feature_x"`
}

type resolved struct {
	App      string  `json:"app"`
	Port     int     `json:"port"`
	Addr     string  `json:"addr"`
	Timeout  string  `json:"timeout"`
	LogLevel string  `json:"log_level"`
	FeatureX bool    `json:"feature_x"`
	Sources  sources `json:"sources"`
}

func main() {
	configPath := findConfigPath(os.Args[1:], defaultConfigPath())
	if err := ensureSampleConfig(configPath); err != nil {
		panic(err)
	}

	cfg, src, err := loadConfig(configPath, os.Args[1:])
	if err != nil {
		panic(err)
	}

	out := resolved{
		App:      cfg.App,
		Port:     cfg.Port,
		Addr:     fmt.Sprintf(":%d", cfg.Port),
		Timeout:  cfg.Timeout,
		LogLevel: cfg.LogLevel,
		FeatureX: cfg.FeatureX,
		Sources:  src,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println("effective config:")
	fmt.Println(string(data))
}

func defaultConfig() rawConfig {
	return rawConfig{
		App:      "demo-api",
		Port:     8080,
		Timeout:  "1s",
		LogLevel: "info",
		FeatureX: false,
	}
}

func defaultSources() sources {
	return sources{
		App:      "default",
		Port:     "default",
		Timeout:  "default",
		LogLevel: "default",
		FeatureX: "default",
	}
}

func loadConfig(path string, args []string) (rawConfig, sources, error) {
	cfg := defaultConfig()
	src := defaultSources()

	if err := applyFile(path, &cfg, &src); err != nil {
		return rawConfig{}, sources{}, err
	}
	if err := applyEnv(&cfg, &src); err != nil {
		return rawConfig{}, sources{}, err
	}
	if err := applyFlags(args, path, &cfg, &src); err != nil {
		return rawConfig{}, sources{}, err
	}
	if err := validateConfig(cfg); err != nil {
		return rawConfig{}, sources{}, err
	}

	return cfg, src, nil
}

func applyFile(path string, cfg *rawConfig, src *sources) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	dec.DisallowUnknownFields()
	var fc fileConfig
	if err := dec.Decode(&fc); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected extra json")
		}
		return err
	}

	applyFileField(fc.App, &cfg.App, &src.App, "config")
	applyFileField(fc.Port, &cfg.Port, &src.Port, "config")
	applyFileField(fc.Timeout, &cfg.Timeout, &src.Timeout, "config")
	applyFileField(fc.LogLevel, &cfg.LogLevel, &src.LogLevel, "config")
	applyFileField(fc.FeatureX, &cfg.FeatureX, &src.FeatureX, "config")

	return nil
}

func applyFileField[T any](val *T, dst *T, src *string, from string) {
	if val == nil {
		return
	}
	*dst = *val
	*src = from
}

func applyEnv(cfg *rawConfig, src *sources) error {
	if v, ok := os.LookupEnv("APP_NAME"); ok {
		cfg.App = v
		src.App = "env"
	}
	if v, ok := os.LookupEnv("APP_PORT"); ok {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid APP_PORT: %w", err)
		}
		cfg.Port = port
		src.Port = "env"
	}
	if v, ok := os.LookupEnv("APP_TIMEOUT"); ok {
		cfg.Timeout = v
		src.Timeout = "env"
	}
	if v, ok := os.LookupEnv("APP_LOG_LEVEL"); ok {
		cfg.LogLevel = v
		src.LogLevel = "env"
	}
	if v, ok := os.LookupEnv("APP_FEATURE_X"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid APP_FEATURE_X: %w", err)
		}
		cfg.FeatureX = b
		src.FeatureX = "env"
	}
	return nil
}

func applyFlags(args []string, configPath string, cfg *rawConfig, src *sources) error {
	fs := flag.NewFlagSet("configlab", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	_ = fs.String("config", configPath, "config file path")
	app := fs.String("app", cfg.App, "app name")
	port := fs.Int("port", cfg.Port, "listen port")
	timeout := fs.String("timeout", cfg.Timeout, "request timeout")
	logLevel := fs.String("log-level", cfg.LogLevel, "log level")
	featureX := fs.Bool("feature-x", cfg.FeatureX, "enable feature x")

	if err := fs.Parse(args); err != nil {
		return err
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	if set["app"] {
		cfg.App = *app
		src.App = "flag"
	}
	if set["port"] {
		cfg.Port = *port
		src.Port = "flag"
	}
	if set["timeout"] {
		cfg.Timeout = *timeout
		src.Timeout = "flag"
	}
	if set["log-level"] {
		cfg.LogLevel = *logLevel
		src.LogLevel = "flag"
	}
	if set["feature-x"] {
		cfg.FeatureX = *featureX
		src.FeatureX = "flag"
	}

	return nil
}

func validateConfig(cfg rawConfig) error {
	if strings.TrimSpace(cfg.App) == "" {
		return errors.New("app is required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return errors.New("port must be 1-65535")
	}
	if _, err := time.ParseDuration(cfg.Timeout); err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	if !validLogLevel(cfg.LogLevel) {
		return fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}
	return nil
}

func validLogLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func defaultConfigPath() string {
	if _, err := os.Stat(filepath.Join("series", "33")); err == nil {
		return filepath.Join("series", "33", "tmp", "config.json")
	}
	return filepath.Join("tmp", "config.json")
}

func findConfigPath(args []string, def string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-config" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "-config=") {
			return strings.TrimPrefix(arg, "-config=")
		}
	}
	return def
}

func ensureSampleConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data := []byte(`{
  "app": "billing-api",
  "port": 9090,
  "timeout": "900ms",
  "log_level": "warn",
  "feature_x": true
}`)

	return os.WriteFile(path, data, 0o644)
}
