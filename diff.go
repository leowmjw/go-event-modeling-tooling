package evml

import "sort"

// FrameDiffState classifies a frame's fate between two versions of a model.
type FrameDiffState string

const (
	DiffAdded     FrameDiffState = "added"
	DiffRemoved   FrameDiffState = "removed"
	DiffChanged   FrameDiffState = "changed"
	DiffUnchanged FrameDiffState = "unchanged"
)

// ModelDiff is the structural comparison of two models, keyed by frame ID.
type ModelDiff struct {
	Frames    map[string]FrameDiffState
	Added     int
	Removed   int
	Changed   int
	Unchanged int
}

// Diff compares baseline against next and classifies every frame of either
// model as added, removed, changed, or unchanged. Frames are matched by ID;
// a shared frame counts as changed when its type, kind, identifier, source
// wiring, data reference, or payload differs between the two versions.
func Diff(baseline, next *Model) ModelDiff {
	d := ModelDiff{Frames: make(map[string]FrameDiffState)}
	base := framesByID(baseline)
	nxt := framesByID(next)
	for id, nf := range nxt {
		bf, ok := base[id]
		if !ok {
			d.Frames[id] = DiffAdded
			d.Added++
			continue
		}
		if frameSignature(bf) == frameSignature(nf) {
			d.Frames[id] = DiffUnchanged
			d.Unchanged++
		} else {
			d.Frames[id] = DiffChanged
			d.Changed++
		}
	}
	for id := range base {
		if _, ok := nxt[id]; !ok {
			d.Frames[id] = DiffRemoved
			d.Removed++
		}
	}
	return d
}

func framesByID(m *Model) map[string]*Frame {
	if m == nil {
		return nil
	}
	return m.FramesByID()
}

// frameSignature captures every aspect of a frame that the diff treats as a
// meaningful change; two frames with equal signatures are "unchanged".
func frameSignature(f *Frame) string {
	sources := append([]string(nil), f.SourceIDs...)
	sort.Strings(sources)
	sig := string(f.Kind) + "|" + string(f.EntityType) + "|" + f.Identifier + "|" +
		f.DataRefName + "|" + f.DisplayData()
	for _, id := range sources {
		sig += "|" + id
	}
	return sig
}
