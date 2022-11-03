package main

import "fmt"

func main() {
	cfg := ParseConfig()

	DB = InitDbConnection(cfg)

	b := InitBot(cfg)

	ns := NewNotificationService(cfg)

	pong, err := ns.rd.Ping().Result()
	fmt.Println(pong, err)

	b.Start()
}
