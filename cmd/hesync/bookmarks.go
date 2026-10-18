package main

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

// count urls recursively
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

// finds a Node by GUID and returns a pointer to it if found
func findByGUID(n *Node, guid string) *Node {
	if n.GUID == guid {
		return n
	}

	for _, c := range n.Children {
		if found := findByGUID(c, guid); found != nil {
			return found
		}
	}
	return nil
}
