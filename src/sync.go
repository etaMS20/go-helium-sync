package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func runSync() error {
	path, err := heliumPath("Default/Bookmarks") // get the path to the Profile file
	if err != nil {
		return err
	}
	running, err := isHeliumRunning() // check if helium is running
	if err != nil {
		return fmt.Errorf("check if Helium is running: %w", err)
	}
	if running {
		return errors.New("Helium is running, close it first")
	}

	// load the original bookmarks file
	data, err := os.ReadFile(path) // gives raw bytes
	if err != nil {
		return err
	}
	// write raw bytes as a backup
	if err := os.WriteFile(path+".gosync.bak", data, 0o600); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}

	// unmarshall into Profile to clear the checksum.
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	// checksum: MD5 over every node's id/name/url/type. Chromium uses it to detect changes it didn't make.
	// We blank it before writing, because any edit makes the old value wrong. Chromium writes a new one on its next save.
	p.CheckSum = ""

	// Write the parsed profile back
	return writeAtomic(path, p)
}
