package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
