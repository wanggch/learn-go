package cliinfo

import (
	"flag"
	"fmt"
	"strings"
)

type Config struct {
	Name string
	Lang string
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("hello", flag.ContinueOnError)
	name := fs.String("name", "工程师", "读者名称")
	lang := fs.String("lang", "go", "关注的语言")
	fs.SetOutput(new(strings.Builder))

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Name: strings.TrimSpace(*name),
		Lang: strings.ToLower(strings.TrimSpace(*lang)),
	}
	if cfg.Name == "" {
		return Config{}, fmt.Errorf("name 不能为空")
	}
	if cfg.Lang == "" {
		cfg.Lang = "go"
	}
	return cfg, nil
}
