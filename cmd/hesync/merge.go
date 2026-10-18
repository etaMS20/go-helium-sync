package main

import (
	"cmp"
	"slices"
)

/**
 * 3-way merge of bookmark trees: flatten, merge per GUID, rebuild.
 *
 * Each tree (base = last sync, local = Helium, remote = after git pull) is flattened into GUID -> entry.
 * Entries are merged per GUID, then regrouped by parent and sorted by pos.
 *
 *   base  local  remote | result
 *   ✓     ✓      ✓      | per field (name, url, parent): local changed vs base ? local : remote
 *   -     ✓      -      | added locally -> local
 *   -     -      ✓      | added remotely -> remote
 *   -     ✓      ✓      | added on both -> local
 *   ✓     -      ✓      | deleted locally -> gone, unless remote edited it
 *   ✓     ✓      -      | deleted remotely -> gone, unless local edited it
 *
 * Order is decided per folder: if local reordered a folder, local's order wins, else remote's.
 * Orphans (parent deleted on the other side) go to Other. Equal pos ties break by GUID.
 */

type Entry struct {
	node   Node   // node with Children = nil
	parent string // parent folder's GUID
	pos    int    // index inside the parent
}

func flatten(p Profile) map[string]Entry {
	m := make(map[string]Entry)

	for _, root := range rootsOf(&p) {
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

/**
 * Rebuilds from a flat-map into a Profile skeleton
 * Example. m has these entries:
 * "a" → {node: GitHub,  parent: "bar",    pos: 0}
 * "b" → {node: Work,    parent: "bar",    pos: 1}
 * "c" → {node: Jira,    parent: "b",      pos: 0}
 *
 * After the loop, byParent looks like this:
 * "bar" → [GitHub, Work]   // what's directly in the bookmark bar
 * "b"   → [Jira]           // what's inside the "Work" folder
 */
func rebuild(skel Profile, m map[string]Entry) Profile {
	byParent := make(map[string][]Entry) // children looked up by parent GUID
	for _, e := range m {
		byParent[e.parent] = append(byParent[e.parent], e) // 1. fill byParent
	}
	for _, c := range byParent {
		slices.SortFunc(c, siblingOrder) // 2. sort by pos (tiebreak GUID)
	}

	// 3. hang the children under each root recursively
	for _, root := range rootsOf(&skel) {
		attach(root, byParent)
	}
	return skel
}

// attach rebuilds the subtree under n from byParent, recursively.
// It replaces n's existing children with copies of the nodes listed
// under n.GUID, in the order they appear there, and repeats this for
// each child. byParent maps a parent GUID to its children and must
// already be sorted.
//
// Entries not reachable from n are ignored. byParent itself is not
// modified.
func attach(n *Node, byParent map[string][]Entry) {
	n.Children = nil // drop any children from the skeleton so nothing is duplicated
	if n.Type == TypeFolder {
		n.Children = []*Node{} // empty folders keep "children": [] (see omitzero on Node)
	}
	for _, e := range byParent[n.GUID] {
		c := e.node // copy, so the tree doesn't alias the nodes stored in byParent
		attach(&c, byParent)
		n.Children = append(n.Children, &c)
	}
}

// Implementation of the 3-way merge
//
// base: the state at the last sync, meaning bookmarks.json in the clone before git pull.
// local: Helium on this machine right now, incl. changes
// remote: bookmarks.json after git pull, meaning what the other machine pushed since then.
//
// base is needed to know who changes what (common ancestor lookup)
// E.g. bookmark X exists in local but not in remote:
// -> Did we add it here? Then keep it.
// -> Or did the other machine delete it? Then remove it.
// Both cases look identical but base can answer it
func merge(base, local, remote Profile) Profile {
	b, l, r := flatten(base), flatten(local), flatten(remote)
	out := make(map[string]Entry)

	// GUIDs that exist on at least one side. Base-only GUIDs were
	// deleted on both sides and are skipped on purpose.
	guids := make(map[string]bool, len(l)+len(r)) // map of guid -> an whether they exist in local or remote
	for g := range l {
		guids[g] = true
	}
	for g := range r {
		guids[g] = true
	}

	for guid := range guids {
		be, inB := b[guid]
		le, inL := l[guid]
		re, inR := r[guid]

		switch {
		case inL && inR && !inB:
			out[guid] = le
		case inL && inR:
			out[guid] = merge3(be, le, re)
		case inL:
			if !inB || edited(be, le) {
				out[guid] = le
			}
		case inR:
			if !inB || edited(be, re) {
				out[guid] = re
			}
		}
	}

	// Sibling order: per folder, take the pos from local if local reordered
	// that folder, else from remote. Entries the winning side doesn't have
	// in that folder keep the other side's pos.
	// ponytail: those entries' pos comes from the other side's numbering, so
	// they can land a slot off. Fine for a single user.
	localReordered := reordered(b, l)
	for g, e := range out {
		win, other := r, l
		if localReordered[e.parent] {
			win, other = l, r
		}
		if w, ok := win[g]; ok && w.parent == e.parent {
			e.pos = w.pos
		} else if o, ok := other[g]; ok && o.parent == e.parent {
			e.pos = o.pos
		}
		out[g] = e
	}

	// Orphans: an entry whose parent folder was deleted on the other side
	// would never be reached by rebuild and silently vanish. Move it to Other.
	// ponytail: misses cycles (A moved into B locally, B into A remotely),
	// switch to a reachable-from-root walk if that ever happens.
	roots := make(map[string]bool)
	for _, n := range rootsOf(&local) {
		roots[n.GUID] = true
	}
	for g, e := range out {
		if _, ok := out[e.parent]; !ok && !roots[e.parent] {
			e.parent = local.Roots.Other.GUID
			out[g] = e
		}
	}
	return rebuild(local, out)
}

// siblingOrder sorts by pos, ties broken by GUID so every machine gets the same order.
func siblingOrder(a, b Entry) int {
	return cmp.Or(cmp.Compare(a.pos, b.pos), cmp.Compare(a.node.GUID, b.node.GUID))
}

// pick returns the side that changed relative to base.
// If both changed, local wins.
func pick[T comparable](base, local, remote T) T {
	if local == base {
		return remote
	}
	return local
}

// edited reports whether e differs from base in content or folder.
// pos is deliberately ignored: it shifts whenever a sibling is added
// or removed, which is not an edit of e itself.
func edited(base, e Entry) bool {
	return e.node.Name != base.node.Name ||
		e.node.URL != base.node.URL ||
		e.parent != base.parent
}

// reordered returns the folders whose children local rearranged vs base.
// Only entries that stayed in the same folder are compared, so adds,
// deletes and moves in or out don't count as a reorder.
func reordered(b, l map[string]Entry) map[string]bool {
	bSib, lSib := make(map[string][]Entry), make(map[string][]Entry)
	for g, le := range l {
		if be, ok := b[g]; ok && be.parent == le.parent {
			bSib[be.parent] = append(bSib[be.parent], be)
			lSib[le.parent] = append(lSib[le.parent], le)
		}
	}
	out := make(map[string]bool)
	for p := range lSib {
		slices.SortFunc(bSib[p], siblingOrder)
		slices.SortFunc(lSib[p], siblingOrder)
		if !slices.EqualFunc(bSib[p], lSib[p], func(x, y Entry) bool { return x.node.GUID == y.node.GUID }) {
			out[p] = true
		}
	}
	return out
}

// merge3 combines the content of an entry that exists in base, local and remote.
// pos is set later, per folder, in merge.
func merge3(b, l, r Entry) Entry {
	out := l
	out.node.Name = pick(b.node.Name, l.node.Name, r.node.Name)
	out.node.URL = pick(b.node.URL, l.node.URL, r.node.URL)
	out.parent = pick(b.parent, l.parent, r.parent)
	return out
}

// rootsOf returns pointers to the three root folders of p.
func rootsOf(p *Profile) []*Node {
	return []*Node{&p.Roots.BookmarkBar, &p.Roots.Other, &p.Roots.Synced}
}
