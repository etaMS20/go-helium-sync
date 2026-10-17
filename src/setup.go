package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// the local data repo lives at `~/.local/share/go-helium-sync` per default
// we need that since git can only commit and push from a local repo
func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "go-helium-sync"), nil
}

func setup(remoteURL string) error {
	// get the dir where the local repo lives
	dir, err := dataDir()
	if err != nil {
		return err
	}

	// check if git is already set up there
	_, err = os.Stat(filepath.Join(dir, ".git"))
	switch {
	case err == nil:
		return fmt.Errorf("already set up: %s exists (delete it to re-run setup)", filepath.Join(dir, ".git"))
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("check data dir: %w", err) // any other error than "doesn't exist" is caught here
	}

	err = os.MkdirAll(dir, 0o700) // 0o700 = octal perms: owner rwx, group/others none (private data)
	if err != nil {
		return err
	}

	if _, err = git(dir, "init", "-b", "main"); err != nil {
		return err
	}
	if _, err = git(dir, "remote", "add", "origin", remoteURL); err != nil {
		return err
	}

	// ask remote if it already has a main with commits (meaning another device already pushed)
	out, err := git("ls-remote", "origin", "main")
	if err != nil {
		return err
	}
	firstDevice := out == "" // this device is the "first device" when there is no main on the remote
	// in this case we run a sync
	if firstDevice {

	}

	return nil
}
