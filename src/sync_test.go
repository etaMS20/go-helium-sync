package main

import (
	"os"
	"path/filepath"
	"testing"
)

// two devices against a local bare repo: an add on B reaches A, a no-op sync makes no commit
func TestSync(t *testing.T) {
	tmp := t.TempDir()
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test")
	}
	remote := filepath.Join(tmp, "remote.git")
	if _, err := git(tmp, "init", "--bare", "-b", "main", remote); err != nil {
		t.Fatal(err)
	}

	// use switches HOME (data repo) and XDG_CONFIG_HOME (Helium) to device dev
	use := func(dev string) string {
		t.Setenv("HOME", filepath.Join(tmp, dev))
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, dev, ".config"))
		bm, err := heliumPath("Default/Bookmarks")
		if err != nil {
			t.Fatal(err)
		}
		return bm
	}
	write := func(path string, p Profile) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := writeAtomic(path, p); err != nil {
			t.Fatal(err)
		}
	}
	bookmarks := func(path string) string {
		p := loadProfile(t, path)
		return dump(&p.Roots.BookmarkBar)
	}

	a, b, c := bm("a"), bm("b"), bm("c")

	bmA := use("A")
	write(bmA, bar(a, b))
	if err := setup(remote); err != nil {
		t.Fatal(err)
	}
	bmB := use("B")
	write(bmB, bar(a, b))
	if err := setup(remote); err != nil {
		t.Fatal(err)
	}

	write(bmB, bar(a, b, c)) // B adds c
	if err := runSync(); err != nil {
		t.Fatal(err)
	}

	use("A")
	if err := runSync(); err != nil {
		t.Fatal(err)
	}
	if got := bookmarks(bmA); got != "bar[a b c]" {
		t.Errorf("A after sync = %q, want bar[a b c]", got)
	}

	// nothing changed: no new commit
	if err := runSync(); err != nil {
		t.Fatal(err)
	}
	if log, _ := git(remote, "log", "--format=%s", "main"); log != "sync\ninitial" {
		t.Errorf("remote log = %q, want sync, initial", log)
	}
}
