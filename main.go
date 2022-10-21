package main

import "fmt"

func main() {
	cfg := ParseConfig()
	fmt.Printf("%+v", cfg)
	db := InitDbConnection(cfg)
	CreateSchema(db)
}
