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
	p := loadProfile(t, "testdata/laptop.json")
	tests := []struct {
		name string
		guid string
		want *string // a pointer to a string can be nil
	}{
		{"should find nested url", "c6c44952-8bd6-48de-ba7f-ba78136cac7e", new("Go Packages")},
		{"should find folder", "06b75729-b892-4266-a80c-4b31b364a353", new("Go")},
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
