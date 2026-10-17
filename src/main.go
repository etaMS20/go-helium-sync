package main

import (
	"encoding/json"
	"log"
	"os"
)

func main() {
	// check if helium is running
	path, err := heliumPath("Default/Bookmarks")
	if err != nil {
		log.Fatal(err)
	}
	running, err := isHeliumRunning()
	if running {
		log.Fatal("close Helium first")
	}

	// load the original bookmarks file
	data, err := os.ReadFile(path) // gives raw bytes
	if err != nil {
		log.Fatal(err)
	}
	// write raw bytes as a backup
	if err := os.WriteFile(path+".gosync.bak", data, 0o600); err != nil {
		log.Fatalf("write backup: %v", err)
	}

	// unmarshall into Profile to clear the checksum.
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		log.Fatalf("parse %s: %v", path, err)
	}
	// checksum: MD5 over every node's id/name/url/type. Chromium uses it to detect changes it didn't make.
	// We blank it before writing, because any edit makes the old value wrong. Chromium writes a new one on its next save.
	p.CheckSum = ""

	// Write the parsed profile back
	err = writeAtomic(path, p)
	if err != nil {
		log.Fatal(err)
	}
}
