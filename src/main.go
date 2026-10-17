package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

/**
 * Types to model a helium-profile -> ~/.config/net.imput.helium/Default
 */

type Profile struct {
	CheckSum string `json:"checksum"` // map json keys to Go fields
	Roots    Roots  `json:"roots"`
	Version  int    `json:"version"`
}

/**
 * omitempty: 	drops false, 0, "", nil and empty slices/maps.
 *				> url/date_modified use omitempty: "" means the key was absent.
 * omitzero: 	drops only zero values (nil), keeps [] and {}.
 *				> children/meta_info use omitzero: an empty folder's "children": [] must survive the round trip.
 */
type Node struct {
	Children     []*Node           `json:"children,omitzero"`
	DateAdded    string            `json:"date_added"`
	DateLastUsed string            `json:"date_last_used"`
	DateModified string            `json:"date_modified,omitempty"`
	GUID         string            `json:"guid"`
	ID           string            `json:"id"`
	MetaInfo     map[string]string `json:"meta_info,omitzero"` // map[KeyType]ValueType -> chrome feature for tab metadata
	Name         string            `json:"name"`
	Type         NodeType          `json:"type"`
	URL          string            `json:"url,omitempty"`
}

type NodeType string

const (
	TypeURL    NodeType = "url"
	TypeFolder NodeType = "folder"
)

type Roots struct {
	BookmarkBar Node `json:"bookmark_bar"`
	Other       Node `json:"other"`
	Synced      Node `json:"synced"`
}

func main() {
	/* 	for i, arg := range os.Args {
		fmt.Println(i, arg)
	} */

	content, err := os.ReadFile(os.Args[1]) // ReadFile() returns `([]byte, error)`
	if err != nil {
		log.Fatal(err)
	}

	var p Profile

	/* & passes a pointer to p so the Unmarshall can fill it, returns an error if it fails */
	err = json.Unmarshal(content, &p)
	if err != nil {
		log.Fatal(err)
	}
	rootItems := map[string]int{
		"bookmark_bar": countURLs(&p.Roots.BookmarkBar),
		"other":        countURLs(&p.Roots.Other),
		"synced":       countURLs(&p.Roots.Synced),
	}
	sum := 0
	for name, n := range rootItems {
		sum += n
		fmt.Println(name, n)
	}
	fmt.Println("total", sum)

	// checksum: MD5 over every node's id/name/url/type. Chromium uses it to detect changes it didn't make.
	// We blank it before writing, because any edit makes the old value wrong. Chromium writes a new one on its next save.
	p.CheckSum = ""

	// Write the parsed profile to a JSON file
	outputFile := "tmp/output.json"
	err = writeJSONToFile(outputFile, p)
	if err != nil {
		log.Fatal(err)
	}
}

func countURLs(n *Node) int {
	switch n.Type {
	case TypeURL:
		return 1
	case TypeFolder:
		total := 0
		for _, c := range n.Children {
			total += countURLs(c)
		}
		return total
	}
	return 0
}

func writeJSONToFile(filename string, data any) error {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	// Create the file
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close() // defer schedule the file to be closed when the function returns

	return encodeJSON(f, data)
}

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

func heliumPath(path string) (string, error) {
	confDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(confDir, "net.imput.helium", path), nil
}

// While Helium is open, `~/.config/net.imput.helium/SingletonLock` exists
// Its a SymLink to the host, so Lstat is used
func isHeliumRunning() (bool, error) {
	lock, err := heliumPath("SingletonLock")
	if err != nil {
		return false, err
	}
	_, err = os.Lstat(lock)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
