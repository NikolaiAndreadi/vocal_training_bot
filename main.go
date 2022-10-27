package main

func main() {
	cfg, _ := ParseConfig()

	DB = InitDbConnection(cfg)

	b := InitBot(cfg)

	b.Start()
}
