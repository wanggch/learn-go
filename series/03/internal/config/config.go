package config

import (
	"fmt"
	"strings"
)

type AppConfig struct {
	AppName string
	Owner   string
}

func New(appName, owner string) (AppConfig, error) {
	appName = strings.TrimSpace(appName)
	owner = strings.TrimSpace(owner)
	if appName == "" {
		return AppConfig{}, fmt.Errorf("appName 不能为空")
	}
	if owner == "" {
		owner = "团队"
	}
	return AppConfig{AppName: appName, Owner: owner}, nil
}
