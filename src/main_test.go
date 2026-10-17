package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const bookmarksFile string = "Bookmarks"

// Go treats files ending in _test.go as tests -> Run with `go test ./src`

// helper function to unmarshal JSON data into a struct and fail the test if there's an error
// t is the testing context, data is the JSON data, and v is the struct to unmarshal into
func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	c, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// helper function to load a Profile from a profile file
func loadProfile(t *testing.T, path string) Profile {
	t.Helper()
	c, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p Profile
	mustUnmarshal(t, c, &p)
	return p
}

// fails the test if want and got don't decode to the same JSON value.
func assertJSONDeepEqual(t *testing.T, name string, want, got []byte) {
	t.Helper()

	var w, g any
	mustUnmarshal(t, want, &w)
	mustUnmarshal(t, got, &g)

	if !reflect.DeepEqual(w, g) {
		t.Errorf("round-trip mismatch for %s", name)
	}
}

// fails the test if there are any leftover files other than the Bookmark file left in the dir
func assertCleanDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != bookmarksFile {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("want only %q in dir, got %v", bookmarksFile, names)
	}
}

func TestCountURLs(t *testing.T) {
	folder := &Node{Type: TypeFolder, Children: []*Node{
		{Type: TypeURL},
		{Type: TypeFolder, Children: []*Node{{Type: TypeURL}}},
	}}

	if got := countURLs(folder); got != 2 {
		t.Errorf("countURLs = %d, want 2", got)
	}
}

// Tests the round-trip of unmarshalling and encoding JSON data to ensure that the process is lossless
func TestRoundTrip(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.json")
	if err != nil {
		t.Fatalf("Error occurred while globbing JSON files: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) { // Base returns the last element of path
			rawContent, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			// 1. Unmarshall the JSON content into a Profile struct
			var p Profile
			mustUnmarshal(t, rawContent, &p)
			// 2. Encode the Profile struct back to JSON
			var buf bytes.Buffer
			err = encodeJSON(&buf, p) // encode into memory buffer
			if err != nil {
				t.Fatal(err)
			}
			// 3. Compare the original content with the re-encoded content using deep equality
			var original, encoded any
			mustUnmarshal(t, rawContent, &original)
			mustUnmarshal(t, buf.Bytes(), &encoded)
			if !reflect.DeepEqual(original, encoded) {
				t.Errorf("Round-trip failed for file %s", file)
			}
		})
	}
}

func TestWriteAtomicSuccess(t *testing.T) {
	dir := t.TempDir() // Create a temporary directory for the test
	path := filepath.Join(dir, bookmarksFile)
	const src = "../testdata/09.json"

	// Write some data to the file, it only exists to be overwritten by the atomic write
	if err := os.WriteFile(path, []byte(`{"old": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	want := mustReadFile(t, src) // Expected content: the raw test data

	// load the Profile and write it atomically
	p := loadProfile(t, src)
	if err := writeAtomic(path, p); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	got := mustReadFile(t, path)
	// compare the files on the disk with the content
	assertJSONDeepEqual(t, path, want, got)

	// check the dir after for leftovers
	assertCleanDir(t, dir)
}

func TestWriteAtomicFail(t *testing.T) {
	dir := t.TempDir() // Create a temporary directory for the test
	path := filepath.Join(dir, bookmarksFile)

	// Write some data to path
	old := []byte(`{"old": true}`)
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}

	// attempt and atomic write with something that is not json-serializable
	notEncodable := make(chan int)
	if err := writeAtomic(path, notEncodable); err == nil {
		t.Errorf("writeAtomic returned nil, want an error")
	}

	// validate that the file is untouched after the failed write (byte-equal)
	got := mustReadFile(t, path)
	if !bytes.Equal(got, old) {
		t.Errorf("file changed after failed write: got %q, want %q", got, old)
	}

	// check the dir after for leftovers
	assertCleanDir(t, dir)
}

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
