package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Creates a JSON encoder on `w` and writes the data to it - `w` can be a file, buffer or any io.Writer
func encodeJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "   ") // Set indentation for pretty printing
	return enc.Encode(data)
}

// Makes sure the written files is either completely the old version or the new version, never a half-written file.
// It does this by writing to a temporary file and then renaming it to the target filename.
func writeAtomic(path string, data any) (err error) { // Named return `err`, so every failing return must land in this one variable.
	// create a temp file in the same directory as the target file
	var f *os.File
	f, err = os.CreateTemp(filepath.Dir(path), ".Bookmarks-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	// Clean up only on failure. After a successful rename there's nothing to remove.
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()

	// Define the steps the function will take to write the data atomically.
	steps := []struct {
		msg string
		fn  func() error
	}{
		{"encode JSON", func() error { return encodeJSON(f, data) }},
		{"sync temp file", f.Sync},
		{"close temp file", f.Close},
		{"rename temp file", func() error { return os.Rename(f.Name(), path) }},
		{"sync directory", func() error { return syncDir(filepath.Dir(path)) }}, // fails on windows
	}

	// perform each step in order, returning an error if any step fails
	for _, s := range steps {
		if err = s.fn(); err != nil { // use the named err with `=` instead of `:=`
			return fmt.Errorf("%s: %w", s.msg, err)
		}
	}
	return nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
