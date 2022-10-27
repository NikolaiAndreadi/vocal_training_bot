package main

func main() {
	cfg := ParseConfig()

	DB = InitDbConnection(cfg)

	b := InitBot(cfg)

	b.Start()
}
