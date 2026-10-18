package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func heliumPath(path string) (string, error) {
	confDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(confDir, "net.imput.helium", path), nil
}

// While Helium is open, `~/.config/net.imput.helium/SingletonLock` exists
// Its a SymLink to the host, so Lstat is used
func isHeliumRunning() (bool, error) {
	lock, err := heliumPath("SingletonLock")
	if err != nil {
		return false, err
	}
	_, err = os.Lstat(lock)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ensures Helium is closed, backs up and parses Bookmarks
// returns the Profile
func readHelium(path string) (Profile, error) {
	running, err := isHeliumRunning() // check if helium is running
	if err != nil {
		return Profile{}, fmt.Errorf("check if Helium is running: %w", err)
	}
	if running {
		return Profile{}, errors.New("Helium is running, close it first")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}
	// write raw bytes as a backup
	if err := os.WriteFile(path+".gosync.bak", data, 0o600); err != nil {
		return Profile{}, fmt.Errorf("write backup: %w", err)
	}

	var p Profile
	// parse into Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parse %s: %w", path, err)
	}

	// checksum: MD5 over every node's id/name/url/type. Chromium uses it to detect changes it didn't make.
	// Cleared on read, so no Profile in this program carries a stale checksum, including bookmarks.json in the repo.
	// Chromium writes a new one on its next save
	p.CheckSum = ""
	return p, nil
}
