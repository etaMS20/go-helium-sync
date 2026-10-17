package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func main() {
	/* 	for i, arg := range os.Args {
		fmt.Println(i, arg)
	} */

	content, err := os.ReadFile(os.Args[1]) // ReadFile() returns `([]byte, error)`
	if err != nil {
		log.Fatal(err)
	}

	var p Profile

	/* & passes a pointer to p so the Unmarshall can fill it, returns an error if it fails */
	err = json.Unmarshal(content, &p)
	if err != nil {
		log.Fatal(err)
	}
	rootItems := map[string]int{
		"bookmark_bar": countURLs(&p.Roots.BookmarkBar),
		"other":        countURLs(&p.Roots.Other),
		"synced":       countURLs(&p.Roots.Synced),
	}
	sum := 0
	for name, n := range rootItems {
		sum += n
		fmt.Println(name, n)
	}
	fmt.Println("total", sum)

	// checksum: MD5 over every node's id/name/url/type. Chromium uses it to detect changes it didn't make.
	// We blank it before writing, because any edit makes the old value wrong. Chromium writes a new one on its next save.
	p.CheckSum = ""

	// Write the parsed profile to a JSON file
	outputFile := "tmp/output.json"
	err = writeAtomic(outputFile, p)
	if err != nil {
		log.Fatal(err)
	}
}
