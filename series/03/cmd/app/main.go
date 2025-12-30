package main

import (
	"flag"
	"fmt"
	"os"

	"learn-go/series/03/internal/config"
	"learn-go/series/03/pkg/greet"
)

func main() {
	appName := flag.String("app", "deploy-bot", "应用名称")
	owner := flag.String("owner", "", "负责人或团队")
	flag.Parse()

	cfg, err := config.New(*appName, *owner)
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}

	message := greet.Format(greet.Message{AppName: cfg.AppName, Owner: cfg.Owner})
	fmt.Println(message)
}
