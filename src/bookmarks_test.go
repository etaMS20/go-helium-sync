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
