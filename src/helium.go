package main

import (
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
