package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func setup(remoteURL string) (err error) {
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

	if remoteURL == "" {
		if remoteURL, err = askRemote(); err != nil {
			return err
		}
	}
	// ask remote before creating anything, so a bad URL leaves no half-set-up dir behind.
	// output is "" if the remote has no main yet (empty repo, so this is the first device)
	out, err := git("", "ls-remote", remoteURL, "main")
	if err != nil {
		return fmt.Errorf("can't reach %s (does the repo exist?): %w", remoteURL, err)
	}
	seed := out == ""

	err = os.MkdirAll(dir, 0o700) // 0o700 = octal perms: owner rwx, group/others none (private data)
	if err != nil {
		return err
	}
	// a failure past this point would leave .git behind and block re-running setup
	defer func() {
		if err != nil {
			os.RemoveAll(filepath.Join(dir, ".git"))
		}
	}()

	if _, err = git(dir, "init", "-b", "main"); err != nil {
		return err
	}
	if _, err = git(dir, "remote", "add", "origin", remoteURL); err != nil {
		return err
	}

	if !seed {
		if _, err := git(dir, "fetch", "origin", "main"); err != nil {
			return err
		}
		// local main = remote main, with tracking so later pull/push work.
		if _, err := git(dir, "checkout", "-B", "main", "origin/main"); err != nil {
			return err
		}
		// main without bookmarks.json (e.g. GitHub's "Add a README"): seed on top of it
		out, err := git(dir, "ls-tree", "--name-only", "origin/main", "bookmarks.json")
		if err != nil {
			return err
		}
		seed = out == ""
	}

	if seed {
		// first device: seed the repo with this device's bookmarks
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
	remote, err := gitProfile(dir, "origin/main")
	if err != nil {
		return err
	}
	// empty base: nothing counts as deleted, so both sides' bookmarks are kept.
	// This device's extras reach the repo on the next sync (they look added locally vs HEAD).
	return writeAtomic(path, merge(Profile{}, local, remote))
}

const repoName = "helium-bookmarks"

// askRemote gets the remote URL interactively: a private GitHub repo via gh if it's
// logged in, otherwise the user creates a repo anywhere and pastes its URL.
func askRemote() (string, error) {
	in := bufio.NewReader(os.Stdin)
	ask := func(q string) string {
		fmt.Print(q)
		s, _ := in.ReadString('\n')
		return strings.TrimSpace(s)
	}

	if _, err := exec.LookPath("gh"); err == nil && exec.Command("gh", "auth", "status").Run() == nil {
		a := strings.ToLower(ask(fmt.Sprintf("Use private GitHub repo %q via gh (created if missing)? [Y/n] ", repoName)))
		if a == "" || strings.HasPrefix(a, "y") {
			return ghRepo(repoName)
		}
	}

	fmt.Println("Create a private git repo (GitHub, GitLab, ...) and paste its URL.")
	if u := ask("Remote URL: "); u != "" {
		return u, nil
	}
	return "", errors.New("no remote URL given")
}

// ghRepo returns the clone URL of the user's repo name, creating it as private if missing.
// Same name on every device, so later devices find the repo the first one created.
func ghRepo(name string) (string, error) {
	view := []string{"repo", "view", name, "--json", "isPrivate,sshUrl,url", "-q", "[.isPrivate, .sshUrl, .url] | @tsv"}
	out, err := gh(view...)
	if err != nil {
		if _, err := gh("repo", "create", name, "--private"); err != nil {
			return "", err
		}
		fmt.Printf("Created private repo %s\n", name)
		if out, err = gh(view...); err != nil {
			return "", err
		}
	}
	f := strings.Split(out, "\t")
	if len(f) != 3 {
		return "", fmt.Errorf("unexpected gh output %q", out)
	}
	if f[0] != "true" {
		return "", fmt.Errorf("repo %s is public, refusing to store bookmarks there", name)
	}
	// match how the user talks to GitHub; https needs `gh auth setup-git` for git to push
	if p, _ := gh("config", "get", "git_protocol", "-h", "github.com"); p == "ssh" {
		return f[1], nil
	}
	return f[2], nil
}

func gh(args ...string) (string, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("gh %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("gh %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}
