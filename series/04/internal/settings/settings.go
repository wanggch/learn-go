package settings

import (
	"fmt"
	"time"
)

type Config struct {
	ServiceName string
	Timeout     time.Duration
	Retry       int
	EnableDebug bool
}

func Default() Config {
	return Config{
		ServiceName: "order-gateway",
		Timeout:     3 * time.Second,
		Retry:       2,
		EnableDebug: false,
	}
}

func ApplyZero(c Config) (Config, error) {
	if c.ServiceName == "" {
		return Config{}, fmt.Errorf("ServiceName 不能为空")
	}
	if c.Timeout == 0 {
		c.Timeout = 3 * time.Second
	}
	if c.Retry == 0 {
		c.Retry = 2
	}
	return c, nil
}
