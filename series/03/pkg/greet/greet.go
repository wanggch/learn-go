package greet

import "fmt"

type Message struct {
	AppName string
	Owner   string
}

func Format(msg Message) string {
	return fmt.Sprintf("[%s] 由 %s 维护，今天运行正常。", msg.AppName, msg.Owner)
}
