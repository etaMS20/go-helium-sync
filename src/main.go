package main

import (
	"log"
	"os"
)

func main() {
	path, err := heliumPath("Default/Bookmarks")
	running, err := isHeliumRunning()
	if running {
		log.Fatal("close Helium first")
	}

	pFile, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %w", path, err)
	}
	if err := os.WriteFile(path+".gosync.bak", pFile, 0o600); err != nil {
		log.Fatalf("write backup: %v", err)
	}

	// checksum: MD5 over every node's id/name/url/type. Chromium uses it to detect changes it didn't make.
	// We blank it before writing, because any edit makes the old value wrong. Chromium writes a new one on its next save.
	// p.CheckSum = ""

	// Write the parsed profile to a JSON file
	/* 	outputFile := "tmp/output.json"
	   	err = writeAtomic(outputFile, p)
	   	if err != nil {
	   		log.Fatal(err)
	   	} */
}
