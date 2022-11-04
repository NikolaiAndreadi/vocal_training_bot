package main

func main() {
	cfg := ParseConfig()

	DB = InitDbConnection(cfg)

	b := InitBot(cfg)

	//ns := NewNotificationService(cfg)

	b.Start()
}
