package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHeliumRunning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	heliumDir := filepath.Join(dir, "net.imput.helium")
	if err := os.MkdirAll(heliumDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lock := filepath.Join(heliumDir, "SingletonLock")
	if err := os.Symlink("myhost-12345", lock); err != nil {
		t.Fatal(err)
	}

	check := func(want bool) {
		t.Helper()
		got, err := isHeliumRunning()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("isHeliumRunning = %v, want %v", got, want)
		}
	}
	check(true) // lock present
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	check(false) // lock gone
}
