package main

import (
	"log"

	"github.com/bootun/veronica/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
