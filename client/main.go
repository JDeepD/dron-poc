package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if len(os.Args) < 3 {
		log.Fatal("Usage: Wrong Args")
		os.Exit(1)
	}

	// TODO

}
