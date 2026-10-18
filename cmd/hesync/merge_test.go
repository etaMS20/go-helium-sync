package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestFlattenRebuild(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.json")
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			p := loadProfile(t, file)
			if got := rebuild(p, flatten(p)); !reflect.DeepEqual(p, got) {
				t.Error("flatten/rebuild round-trip changed the profile")
			}
		})
	}
}

// bm, folder and bar build tiny profiles: name doubles as GUID unless renamed.
func bm(guid string, name ...string) *Node {
	n := &Node{GUID: guid, Name: guid, Type: TypeURL, URL: "https://" + guid}
	if len(name) > 0 {
		n.Name = name[0]
	}
	return n
}

func folder(guid string, kids ...*Node) *Node {
	return &Node{GUID: guid, Name: guid, Type: TypeFolder, Children: kids}
}

func bar(kids ...*Node) Profile {
	var p Profile
	p.Roots.BookmarkBar = *folder("bar", kids...)
	p.Roots.Other = *folder("other")
	p.Roots.Synced = *folder("synced")
	return p
}

// dump renders a node as "name" or "name[child child]".
func dump(n *Node) string {
	if n.Type != TypeFolder {
		return n.Name
	}
	s := n.Name + "["
	for i, c := range n.Children {
		if i > 0 {
			s += " "
		}
		s += dump(c)
	}
	return s + "]"
}

func TestMerge(t *testing.T) {
	a, b, c := bm("a"), bm("b"), bm("c")
	tests := []struct {
		name                string
		base, local, remote Profile
		want                string
	}{
		{"unchanged", bar(a, b), bar(a, b), bar(a, b), "bar[a b] other[]"},
		{"add local", bar(a, b), bar(a, b, c), bar(a, b), "bar[a b c] other[]"},
		{"add remote", bar(a, b), bar(a, b), bar(a, b, c), "bar[a b c] other[]"},
		{"add both: remote first, then local", bar(a), bar(a, b), bar(a, c), "bar[a c b] other[]"},
		{"delete local", bar(a, b), bar(a), bar(a, b), "bar[a] other[]"},
		{"delete remote", bar(a, b), bar(a, b), bar(a), "bar[a] other[]"},
		{"edit local beats delete remote", bar(a, b), bar(a, bm("b", "B")), bar(a), "bar[a B] other[]"},
		{"edit remote beats delete local", bar(a, b), bar(a), bar(a, bm("b", "B")), "bar[a B] other[]"},
		{"rename remote", bar(a), bar(a), bar(bm("a", "A")), "bar[A] other[]"},
		{"rename both: local wins", bar(a), bar(bm("a", "L")), bar(bm("a", "R")), "bar[L] other[]"},
		{"move local", bar(a, folder("f")), bar(folder("f", a)), bar(a, folder("f")), "bar[f[a]] other[]"},
		{"reorder local", bar(a, b), bar(b, a), bar(a, b), "bar[b a] other[]"},
		{"reorder remote", bar(a, b), bar(a, b), bar(b, a), "bar[b a] other[]"},
		{"reorder remote + add local", bar(a, b), bar(a, b, c), bar(b, a), "bar[b a c] other[]"},
		{"reorder remote + add local first", bar(a, b, c), bar(bm("d"), a, b, c), bar(c, b, a), "bar[d c b a] other[]"},
		{"reorder remote + add local after a", bar(a, b, c), bar(a, bm("d"), b, c), bar(c, b, a), "bar[c b a d] other[]"},
		{"reorder both: local wins", bar(a, b, c), bar(c, a, b), bar(b, c, a), "bar[c a b] other[]"},
		{"move out locally keeps remote reorder",
			bar(folder("f", a, b, c)), bar(folder("f", a, c), b), bar(folder("f", c, b, a)),
			"bar[f[c a] b] other[]"},
		{"orphan goes to Other",
			bar(folder("f", a)), bar(folder("f", a, c)), bar(),
			"bar[] other[c]"},
		{"folders moved into each other go to Other",
			bar(folder("f"), folder("g")), bar(folder("g", folder("f"))), bar(folder("f", folder("g"))),
			"bar[] other[g[f[]]]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := merge(tt.base, tt.local, tt.remote)
			if got := dump(&p.Roots.BookmarkBar) + " " + dump(&p.Roots.Other); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
