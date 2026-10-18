package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// runSync does a 3-way merge of Helium's bookmarks with the repo and pushes the result.
//
// fetch + `git show` instead of `git pull`: base and remote are read straight from
// git, so an earlier failed push can never leave a merge conflict in bookmarks.json.
func runSync() error {
	path, err := heliumPath("Default/Bookmarks") // get the path to the Profile file
	if err != nil {
		return err
	}
	local, err := readHelium(path) // fails if Helium is running, writes a backup
	if err != nil {
		return err
	}
	dir, err := dataDir()
	if err != nil {
		return err
	}

	base, err := gitProfile(dir, "HEAD") // state at this device's last sync
	if err != nil {
		return err
	}
	if _, err := git(dir, "fetch", "-q", "origin", "main"); err != nil {
		return err
	}
	remote, err := gitProfile(dir, "origin/main")
	if err != nil {
		return err
	}
	merged := merge(base, local, remote)

	// Helium first: if anything below fails, HEAD is still the old base,
	// and the next run merges again from there without losing changes.
	if err := writeAtomic(path, merged); err != nil {
		return err
	}

	// merged already contains everything from HEAD, so resetting to the remote loses nothing
	if _, err := git(dir, "reset", "--hard", "origin/main"); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, "bookmarks.json"), merged); err != nil {
		return err
	}
	changed, err := git(dir, "status", "--porcelain", "bookmarks.json")
	if err != nil {
		return err
	}
	if changed == "" {
		return nil // remote already has exactly this, nothing to push
	}
	if _, err := git(dir, "commit", "-q", "-am", "sync"); err != nil {
		return err
	}
	_, err = git(dir, "push", "-q", "origin", "main")
	return err
}

// gitProfile parses bookmarks.json as it is stored at ref (e.g. "HEAD", "origin/main").
func gitProfile(dir, ref string) (Profile, error) {
	out, err := git(dir, "show", ref+":bookmarks.json")
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		return Profile{}, fmt.Errorf("parse %s:bookmarks.json: %w", ref, err)
	}
	return p, nil
}
