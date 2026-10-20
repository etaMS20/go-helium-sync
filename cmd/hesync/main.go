package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: hesync setup [remote-url] | sync")
	}
	var err error
	switch os.Args[1] {
	case "setup":
		if len(os.Args) > 3 {
			log.Fatal("usage: hesync setup [remote-url]")
		}
		url := "" // empty: setup asks for it
		if len(os.Args) == 3 {
			url = os.Args[2]
		}
		err = setup(url)
	case "sync":
		err = runSync()
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}
