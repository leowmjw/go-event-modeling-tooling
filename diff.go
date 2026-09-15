package evml

import (
	"fmt"
	"sort"
	"strings"
)

// FrameChange describes one frame that differs between two versions.
type FrameChange struct {
	ID      string
	Before  *Frame // nil when added
	After   *Frame // nil when removed
	Reasons []string
}

// ModelDiff is a semantic comparison of two versions of the same flow —
// what a domain expert wants to see before promoting a draft: which steps
// were added, removed or reworded, and how many scenarios / open questions
// moved.
type ModelDiff struct {
	Added   []FrameChange
	Removed []FrameChange
	Changed []FrameChange

	ScenariosAdded   int
	ScenariosRemoved int
	HotspotsOpened   int
	HotspotsResolved int
	SlicesAdded      []string
	SlicesRemoved    []string
	StageChanges     []string // "12 CalculateFee: staging → current"
}

// Empty reports whether nothing differs.
func (d ModelDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 &&
		d.ScenariosAdded == 0 && d.ScenariosRemoved == 0 &&
		d.HotspotsOpened == 0 && d.HotspotsResolved == 0 &&
		len(d.SlicesAdded) == 0 && len(d.SlicesRemoved) == 0 && len(d.StageChanges) == 0
}

// Summary renders the diff as short human-readable lines.
func (d ModelDiff) Summary() []string {
	var out []string
	for _, c := range d.Added {
		out = append(out, fmt.Sprintf("+ %s %s %s", c.ID, c.After.EntityType, c.After.Identifier))
	}
	for _, c := range d.Removed {
		out = append(out, fmt.Sprintf("- %s %s %s", c.ID, c.Before.EntityType, c.Before.Identifier))
	}
	for _, c := range d.Changed {
		out = append(out, fmt.Sprintf("~ %s %s: %s", c.ID, c.After.Identifier, strings.Join(c.Reasons, "; ")))
	}
	for _, s := range d.StageChanges {
		out = append(out, "» "+s)
	}
	if d.ScenariosAdded > 0 {
		out = append(out, fmt.Sprintf("+ %d scenario(s)", d.ScenariosAdded))
	}
	if d.ScenariosRemoved > 0 {
		out = append(out, fmt.Sprintf("- %d scenario(s)", d.ScenariosRemoved))
	}
	if d.HotspotsOpened > 0 {
		out = append(out, fmt.Sprintf("+ %d open question(s)", d.HotspotsOpened))
	}
	if d.HotspotsResolved > 0 {
		out = append(out, fmt.Sprintf("✓ %d question(s) resolved", d.HotspotsResolved))
	}
	for _, s := range d.SlicesAdded {
		out = append(out, "+ slice "+s)
	}
	for _, s := range d.SlicesRemoved {
		out = append(out, "- slice "+s)
	}
	return out
}

// Diff compares two parsed versions of a flow by frame ID.
func Diff(before, after *Model) ModelDiff {
	var d ModelDiff
	if before == nil {
		before = &Model{}
	}
	if after == nil {
		after = &Model{}
	}
	oldByID := map[string]*Frame{}
	for _, f := range before.Frames {
		oldByID[f.ID] = f
	}
	newByID := map[string]*Frame{}
	for _, f := range after.Frames {
		newByID[f.ID] = f
	}
	for _, f := range after.Frames {
		old, ok := oldByID[f.ID]
		if !ok {
			d.Added = append(d.Added, FrameChange{ID: f.ID, After: f})
			continue
		}
		var reasons []string
		if old.EntityType != f.EntityType {
			reasons = append(reasons, fmt.Sprintf("type %s → %s", old.EntityType, f.EntityType))
		}
		if old.Identifier != f.Identifier {
			reasons = append(reasons, fmt.Sprintf("renamed %s → %s", old.Identifier, f.Identifier))
		}
		if old.Kind != f.Kind {
			reasons = append(reasons, fmt.Sprintf("kind %s → %s", old.Kind, f.Kind))
		}
		if old.Actor != f.Actor {
			reasons = append(reasons, fmt.Sprintf("actor %q → %q", old.Actor, f.Actor))
		}
		if normalizePayload(old.DisplayData()) != normalizePayload(f.DisplayData()) {
			reasons = append(reasons, "payload changed")
		}
		if strings.Join(old.SourceIDs, ",") != strings.Join(f.SourceIDs, ",") {
			reasons = append(reasons, "sources changed")
		}
		if len(reasons) > 0 {
			d.Changed = append(d.Changed, FrameChange{ID: f.ID, Before: old, After: f, Reasons: reasons})
		}
		if os, ns := before.FrameStage(old), after.FrameStage(f); os != ns {
			d.StageChanges = append(d.StageChanges, fmt.Sprintf("%s %s: %s → %s", f.ID, f.Identifier, os, ns))
		}
	}
	for _, f := range before.Frames {
		if _, ok := newByID[f.ID]; !ok {
			d.Removed = append(d.Removed, FrameChange{ID: f.ID, Before: f})
		}
	}

	oldGWT := gwtKeys(before)
	newGWT := gwtKeys(after)
	for k := range newGWT {
		if !oldGWT[k] {
			d.ScenariosAdded++
		}
	}
	for k := range oldGWT {
		if !newGWT[k] {
			d.ScenariosRemoved++
		}
	}
	oldHot := hotspotKeys(before)
	newHot := hotspotKeys(after)
	for k := range newHot {
		if !oldHot[k] {
			d.HotspotsOpened++
		}
	}
	for k := range oldHot {
		if !newHot[k] {
			d.HotspotsResolved++
		}
	}
	oldSlices := map[string]bool{}
	for _, s := range before.Slices {
		oldSlices[s.Name] = true
	}
	newSlices := map[string]bool{}
	for _, s := range after.Slices {
		newSlices[s.Name] = true
		if !oldSlices[s.Name] {
			d.SlicesAdded = append(d.SlicesAdded, s.Name)
		}
	}
	for _, s := range before.Slices {
		if !newSlices[s.Name] {
			d.SlicesRemoved = append(d.SlicesRemoved, s.Name)
		}
	}
	sort.Strings(d.SlicesAdded)
	sort.Strings(d.SlicesRemoved)
	return d
}

func gwtKeys(m *Model) map[string]bool {
	out := map[string]bool{}
	for _, g := range m.GWTs {
		out[g.SourceID+"|"+StripQuotes(g.Label)] = true
	}
	return out
}

func hotspotKeys(m *Model) map[string]bool {
	out := map[string]bool{}
	for _, h := range m.Hotspots {
		out[h.SourceID+"|"+normalizePayload(h.Value)] = true
	}
	return out
}

func normalizePayload(s string) string {
	fields := strings.Fields(StripOuterBraces(s))
	return strings.Join(fields, " ")
}
