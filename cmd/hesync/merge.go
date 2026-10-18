package main

import (
	"cmp"
	"maps"
	"slices"
)

// Merging the bookmarks of two devices.
//
// During a sync, there are three versions of the bookmarks:
//
//   - base:   what both devices agreed on at the last sync
//   - local:  what Helium on this device has right now
//   - remote: what the other device pushed since the last sync
//
// Comparing local and remote alone isn't enough. If a bookmark is here but
// not on the other device, was it added here, or was it deleted on the
// other device? base tells: if base doesn't have it, it's new, otherwise
// it was deleted over there.
//
// Trees are awkward to compare, so the merge works in three steps:
//
//  1. flatten each tree into a map of items by GUID, where each item
//     remembers its folder and its position in that folder
//  2. decide for each item what survives (see merge)
//  3. rebuild a tree from the result
//
// Example: "GitHub" is renamed to "GH" on the laptop while "Hacker News" is
// added on the desktop. After both have synced, both have "GH" and
// "Hacker News".

// Entry is one bookmark or folder taken out of the tree. Instead of holding
// its children, it remembers where it lives:
//
//	{node: "Go Packages", parent: <GUID of the "Go" folder>, pos: 1}
//
// means "the second item in the Go folder".
type Entry struct {
	node   Node   // node with Children = nil
	parent string // parent folder's GUID
	pos    int    // index inside the parent
}

// flatten turns a bookmark tree into a map from GUID to Entry:
//
//	Bookmarks bar
//	├── GitHub
//	└── Go
//	    └── go.dev
//
// becomes
//
//	"github" → {GitHub, parent: "bar", pos: 0}
//	"go"     → {Go,     parent: "bar", pos: 1}
//	"godev"  → {go.dev, parent: "go",  pos: 0}
//
// The three root folders (bar, Other, Mobile) aren't in the map, they always exist.
func flatten(p Profile) map[string]Entry {
	m := make(map[string]Entry)

	for _, root := range rootsOf(&p) {
		walk(root, m)
	}
	return m
}

// walk adds everything below parent to m, going into subfolders too.
func walk(parent *Node, m map[string]Entry) {
	for i, c := range parent.Children {
		n := *c          // copy the node first to not change the original tree
		n.Children = nil // set children to nil
		m[c.GUID] = Entry{node: n, parent: parent.GUID, pos: i}
		walk(c, m) // recurse through children
	}
}

// rebuild is the reverse of flatten: it turns the map back into a tree.
// skel provides the root folders, whose contents are replaced.
//
// It first groups the items by folder and sorts each group by position:
//
//	"bar" → [GitHub, Go]
//	"go"  → [go.dev]
//
// and then fills each root folder from those groups, folder by folder.
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

// attach fills folder n with its children from byParent (already sorted),
// then does the same for each child. Whatever n held before is dropped.
// Items that aren't somewhere inside n are ignored. byParent isn't modified.
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

// merge combines the bookmarks of both devices (see the top of this file for
// base, local and remote). For each item it checks which versions have it:
//
//   - New on one device (not in base): keep it.
//   - Gone on one device, still on the other: the first device deleted it, so
//     it's removed. Unless the other device changed it since (renamed it,
//     changed the URL or moved it): then it's kept, so no edit gets lost.
//   - On both devices: name, URL and folder are each taken from whichever
//     device changed them. If both changed the same thing, this device wins.
//     Example: "GitHub" is renamed to "GH" here and moved into "Dev" on the
//     desktop. The result is "GH" inside "Dev".
//   - Only in base: deleted on both devices, nothing to do.
//
// After that it decides the order inside each folder, and makes sure nothing
// ended up inside a folder that no longer exists.
func merge(base, local, remote Profile) Profile {
	b, l, r := flatten(base), flatten(local), flatten(remote)
	out := make(map[string]Entry)

	// Every GUID on at least one device. Ones only in base were deleted on
	// both devices and are skipped on purpose.
	guids := make(map[string]bool, len(l)+len(r))
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

	// Order inside each folder. If the folder was rearranged on this device,
	// this device's order wins, otherwise the other device's order is used.
	// Items the winning order doesn't know (they were only added or moved
	// there on the other device) are slotted in:
	//
	//   - right after the item they came after on their own device
	//   - first, if they came before every item both devices have
	//   - last, if they came after every item both devices have
	//
	// Example: the bar was reversed to [c b a] on the desktop, and d was added
	// right after a here: [a d b c]. The result is [c b a d], d still follows a.
	localReordered := reordered(b, l)
	byParent := make(map[string][]string)
	for g, e := range out {
		byParent[e.parent] = append(byParent[e.parent], g)
	}
	for p, gs := range byParent {
		win, other := r, l
		if localReordered[p] {
			win, other = l, r
		}
		order, others := sideOrder(win, gs, p), sideOrder(other, gs, p)
		tail := len(others) // others[tail:] come after every shared entry: they stay last
		for tail > 0 && !slices.Contains(order, others[tail-1]) {
			tail--
		}
		at := 0 // insert point: right after the last shared entry seen so far
		for _, g := range others[:tail] {
			if i := slices.Index(order, g); i >= 0 {
				at = i + 1
				continue
			}
			order = slices.Insert(order, at, g)
			at++
		}
		order = append(order, others[tail:]...)
		for i, g := range order {
			e := out[g]
			e.pos = i
			out[g] = e
		}
	}

	// Make sure every item stays visible. rebuild only finds items by going
	// down from the root folders, so an item whose folders don't lead back up
	// to a root would silently disappear. That happens when:
	//
	//   - its folder was deleted on the other device, or
	//   - two folders were moved into each other: A into B here, B into A on
	//     the other device. Now A is in B and B is in A, and neither is in a root.
	//
	// So for each item, follow its folders upwards. Where that breaks, move
	// the item (with everything inside it) to "Other bookmarks". Items are
	// checked in GUID order, so both devices break such a loop at the same folder.
	roots := make(map[string]bool)
	for _, n := range rootsOf(&local) {
		roots[n.GUID] = true
	}
	for _, g := range slices.Sorted(maps.Keys(out)) {
		seen := make(map[string]bool)
		for c := g; !roots[c]; {
			seen[c] = true
			p := out[c].parent
			if _, ok := out[p]; !roots[p] && (!ok || seen[p]) {
				e := out[c]
				e.parent = local.Roots.Other.GUID
				out[c] = e
				break
			}
			c = p
		}
	}
	return rebuild(local, out)
}

// siblingOrder sorts items by position. Two items with the same position are
// sorted by GUID, so every device ends up with the same order.
func siblingOrder(a, b Entry) int {
	return cmp.Or(cmp.Compare(a.pos, b.pos), cmp.Compare(a.node.GUID, b.node.GUID))
}

// pick returns whichever of local and remote differs from base.
// If both changed, local wins:
//
//	pick("GitHub", "GH", "GitHub")   // "GH": changed here
//	pick("GitHub", "GitHub", "Code") // "Code": changed on the other device
//	pick("GitHub", "GH", "Code")     // "GH": changed on both, this device wins
func pick[T comparable](base, local, remote T) T {
	if local == base {
		return remote
	}
	return local
}

// edited reports whether the item was changed since base: renamed, given a
// new URL or moved to another folder. A new position doesn't count, it shifts
// whenever something above it is added or removed.
func edited(base, e Entry) bool {
	return e.node.Name != base.node.Name ||
		e.node.URL != base.node.URL ||
		e.parent != base.parent
}

// reordered returns the folders whose order was changed on this device.
// It only compares items that were in the folder before and still are, so
// adding, deleting or moving something in or out isn't a reorder:
//
//	base [a b c] → local [a c]   not reordered (b was removed)
//	base [a b c] → local [a b d] not reordered (c removed, d added)
//	base [a b c] → local [c a b] reordered
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

// sideOrder returns those GUIDs from gs that side has inside folder p, in the
// order side has them. Example: side's folder holds [x b a] and gs is
// [a b c]: the result is [b a].
func sideOrder(side map[string]Entry, gs []string, p string) []string {
	var es []Entry
	for _, g := range gs {
		if e, ok := side[g]; ok && e.parent == p {
			es = append(es, e)
		}
	}
	slices.SortFunc(es, siblingOrder)
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.node.GUID
	}
	return out
}

// merge3 combines an item that's in base and on both devices. Name, URL and
// folder are each picked on their own (see pick), so a rename here and a move
// on the other device both survive. The position is decided later, in merge.
func merge3(b, l, r Entry) Entry {
	out := l
	out.node.Name = pick(b.node.Name, l.node.Name, r.node.Name)
	out.node.URL = pick(b.node.URL, l.node.URL, r.node.URL)
	out.parent = pick(b.parent, l.parent, r.parent)
	return out
}

// rootsOf returns the three root folders of p: bookmark bar, Other and Mobile.
func rootsOf(p *Profile) []*Node {
	return []*Node{&p.Roots.BookmarkBar, &p.Roots.Other, &p.Roots.Synced}
}
