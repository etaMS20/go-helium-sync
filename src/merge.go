package main

/**
 * 3-way merge of bookmark trees: flatten, merge per GUID, rebuild.
 *
 * Each tree (base = last sync, local = Helium, remote = after git pull) is flattened into GUID -> entry.
 * Entries are merged per GUID, then regrouped by parent and sorted by pos.
 *
 *   base  local  remote | result
 *   ✓     ✓      ✓      | local changed vs base ? local : remote
 *   -     ✓      -      | added locally -> local
 *   -     -      ✓      | added remotely -> remote
 *   -     ✓      ✓      | added on both -> local
 *   ✓     -      any    | deleted locally -> gone
 *   ✓     any    -      | deleted remotely -> gone
 *
 * "changed" compares node fields, parent and pos, but ignores ID (per-device counter).
 * Orphans (parent deleted on the other side) go to Other. Equal pos ties break by GUID.
 */

type Entry struct {
	node   Node   // node with Children = nil
	parent string // parent folder's GUID
	pos    int    // index inside the parent
}

func flatten(p Profile) map[string]Entry {
	m := make(map[string]Entry)

	for _, root := range []*Node{&p.Roots.BookmarkBar, &p.Roots.Other, &p.Roots.Synced} {
		walk(root, m)
	}
	return m
}

func walk(parent *Node, m map[string]Entry) {
	for i, c := range parent.Children {
		n := *c          // copy the node first to not change the original tree
		n.Children = nil // set children to nil
		m[c.GUID] = Entry{node: n, parent: parent.GUID, pos: i}
		walk(c, m) // recurse through children
	}
}
