package main

import (
	"encoding/json"
	"os"
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
