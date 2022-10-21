package main

import "fmt"

func main() {
	cfg := ParseConfig()
	fmt.Printf("%+v\\n", cfg)
}
