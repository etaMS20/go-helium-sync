package main

import "testing"

func TestCountURLs(t *testing.T) {
	folder := &Node{Type: TypeFolder, Children: []*Node{
		{Type: TypeURL},
		{Type: TypeFolder, Children: []*Node{{Type: TypeURL}}},
	}}

	if got := countURLs(folder); got != 2 {
		t.Errorf("countURLs = %d, want 2", got)
	}
}

func TestFindByGUID(t *testing.T) {
	p := loadProfile(t, "../testdata/09.json")
	tests := []struct {
		name string
		guid string
		want *string // a pointer to a string can be nil
	}{
		{"should find nested url", "62466a5b-b2b1-4d3e-8ff4-326fbc2776e0", new("httpbin.org")},
		{"should find folder", "2a76546f-46e4-4e6c-ac49-0e211048ee63", new("Test Folder")},
		{"should find nothing", "asd", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findByGUID(&p.Roots.BookmarkBar, tt.guid)

			switch {
			case tt.want == nil && got != nil:
				t.Errorf("expected no bookmark, got %q", got.Name)
			case tt.want != nil && got == nil:
				t.Fatalf("expected %q, got nil", *tt.want)
			case tt.want != nil && got.Name != *tt.want:
				t.Errorf("got %q, want %q", got.Name, *tt.want)
			}
		})
	}
}
