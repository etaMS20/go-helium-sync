package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// first device -> join (after deleting the clone) -> refuse, against a local bare repo as remote
func TestSetup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))
	// HOME is empty, so git has no global identity
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test")
	}

	remote := filepath.Join(tmp, "remote.git")
	if _, err := git(tmp, "init", "--bare", "-b", "main", remote); err != nil {
		t.Fatal(err)
	}

	bm, err := heliumPath("Default/Bookmarks")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(bm), 0o700); err != nil {
		t.Fatal(err)
	}
	seed := mustReadFile(t, "testdata/laptop.json")
	if err := os.WriteFile(bm, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	want := loadProfile(t, "testdata/laptop.json")
	want.CheckSum = ""

	dir, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}

	// unreachable remote: fails before creating the data dir, so setup can be re-run
	if err := setup(filepath.Join(tmp, "missing.git")); err == nil {
		t.Fatal("setup with missing remote succeeded")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("data dir left behind after failed setup (stat err %v)", err)
	}

	// first device: seeds the remote
	if err := setup(remote); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o700 {
		t.Errorf("data dir perms = %v, want 0700", fi.Mode().Perm())
	}
	if up, err := git(dir, "rev-parse", "--abbrev-ref", "main@{upstream}"); err != nil || up != "origin/main" {
		t.Errorf("main upstream = %q (err %v), want origin/main", up, err)
	}
	if log, _ := git(remote, "log", "--format=%s", "main"); log != "initial" {
		t.Errorf("remote log = %q, want single commit initial", log)
	}
	if got := loadProfile(t, filepath.Join(dir, "bookmarks.json")); got.CheckSum != "" {
		t.Errorf("bookmarks.json checksum = %q, want empty", got.CheckSum)
	}

	// join: fresh clone, different local bookmarks are merged with the remote's
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bm, mustReadFile(t, "testdata/desktop.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setup(remote); err != nil {
		t.Fatal(err)
	}
	if up, err := git(dir, "rev-parse", "--abbrev-ref", "main@{upstream}"); err != nil || up != "origin/main" {
		t.Errorf("join: main upstream = %q (err %v), want origin/main", up, err)
	}
	second := loadProfile(t, "testdata/desktop.json")
	second.CheckSum = ""
	if got := loadProfile(t, bm); !reflect.DeepEqual(got, merge(Profile{}, second, want)) {
		t.Error("joined Bookmarks aren't the merge of both devices")
	}
	assertJSONDeepEqual(t, "backup", mustReadFile(t, "testdata/desktop.json"), mustReadFile(t, bm+".gosync.bak"))

	// third run refuses
	if err := setup(remote); err == nil || !strings.Contains(err.Error(), "already set up") {
		t.Errorf("third run err = %v, want already set up", err)
	}
}
