package main

import (
	"log"
	"os"

	"github.com/davehornigan/MovieTagger/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}
