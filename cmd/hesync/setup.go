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
	path, err := heliumPath("Default/Bookmarks")
	if err != nil {
		return err
	}
	// fail before touching anything if Helium is open; also backs up Bookmarks
	local, err := readHelium(path) // returns the local Profile
	if err != nil {
		return err
	}

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
	out, err := git(dir, "ls-remote", "origin", "main")
	if err != nil {
		return err
	}
	firstDevice := out == "" // this device is the "first device" when there is no main on the remote (output is "")
	if firstDevice {
		// seed the repo with this device's bookmarks - write the local Profile into bookmarks.json
		if err := writeAtomic(filepath.Join(dir, "bookmarks.json"), local); err != nil {
			return err
		}
		if _, err := git(dir, "add", "bookmarks.json"); err != nil {
			return err
		}
		if _, err := git(dir, "commit", "-m", "initial"); err != nil {
			return err
		}
		_, err = git(dir, "push", "-u", "origin", "main")
		return err
	}

	// join: remote already has bookmarks, merge this device's into them.
	if _, err := git(dir, "fetch", "origin", "main"); err != nil {
		return err
	}
	// local main = remote main, with tracking so later pull/push work.
	if _, err := git(dir, "checkout", "-B", "main", "origin/main"); err != nil {
		return err
	}
	remote, err := gitProfile(dir, "origin/main")
	if err != nil {
		return err
	}
	// empty base: nothing counts as deleted, so both sides' bookmarks are kept.
	// This device's extras reach the repo on the next sync (they look added locally vs HEAD).
	return writeAtomic(path, merge(Profile{}, local, remote))
}
