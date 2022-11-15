package main

import (
	"net/http"
	"os"
)

func main() {
	_, err := http.Get(":2112")
	if err != nil {
		os.Exit(1)
	}
}
