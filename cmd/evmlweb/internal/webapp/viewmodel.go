package webapp

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

// ModelChoiceView adds a human-readable size to a ModelChoice for display.
type ModelChoiceView struct {
	ID        string
	SizeHuman string
}

// ChatMessageView pre-renders a ChatMessage's content as safe HTML (plain
// text, HTML-escaped, newlines turned into <br>) for template embedding.
type ChatMessageView struct {
	Role        ChatRole
	ContentHTML template.HTML
}

// DraftTab is one entry in the draft-version tab strip.
type DraftTab struct {
	ID           string
	Label        string // friendly name; falls back to "v<n>" when empty
	ShortLabel   string // compact "v<n>" for tight spaces
	HasLabel     bool   // true when the user gave this draft a custom label
	SliceSummary string // short status summary across the draft's slices
}

// ChapterView is a single chapter rendered in the left-rail chapter list.
type ChapterView struct {
	Label   string
	StartID string
	EndID   string
}

// SliceView is a single slice rendered in the right-rail slice strip.
type SliceView struct {
	Name    string
	StartID string
	EndID   string
	Status  string
}

// HotspotSummaryView is one row in the left-rail hotspot list.
type HotspotSummaryView struct {
	FrameID   string
	FrameName string
	Snippet   string // first ~80 chars of the question body
	Resolved  bool
}

// WorkspacePage is the full view model for both the initial page render
// and the workspace SSE fragment.
type WorkspacePage struct {
	ModelID       string
	Models        []ModelChoiceView
	Fixtures      []string
	ActiveFlow    string
	Drafts        []DraftTab
	ActiveDraftID string
	ActiveSVG     template.HTML
	Transcript    []ChatMessageView
	ParseError    string
	// New view-model fields for the workshop-friendly UX overhaul
	// (EVENT_MODELING.md §6/§9/§10):
	Chapters        []ChapterView
	Slices          []SliceView
	Hotspots        []HotspotSummaryView
	OpenHotspotN    int // convenience: len(Hotspots where !Resolved)
	HasDrafts       bool
	SelectedFrameID string // currently-selected frame for the right-rail details panel
	// PatchSVG is true when rendering the workspace fragment for an SSE
	// patch. The SVG is sent in a separate patch to #svg-container so
	// Datastar never morphs a large HTML tree containing inline <svg>.
	PatchSVG bool
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func toModelViews(choices []ModelChoice) []ModelChoiceView {
	views := make([]ModelChoiceView, 0, len(choices))
	for _, c := range choices {
		views = append(views, ModelChoiceView{ID: c.ID, SizeHuman: humanSize(c.VRAMEstBytes)})
	}
	return views
}

func toChatViews(msgs []ChatMessage) []ChatMessageView {
	views := make([]ChatMessageView, 0, len(msgs))
	for _, m := range msgs {
		escaped := html.EscapeString(m.Content)
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		views = append(views, ChatMessageView{Role: m.Role, ContentHTML: template.HTML(escaped)})
	}
	return views
}

func draftLabel(d *DraftVersion) string {
	if d.Label != "" {
		return d.Label
	}
	return fmt.Sprintf("v%d", d.Seq)
}

// sliceSummary returns a short summary of how many of the draft's
// slices are Done / InProgress / Planned (the three most useful for the
// build-progress view). Empty when the draft has no slices.
func sliceSummary(evmlSrc string) string {
	m, err := evml.Parse(evmlSrc)
	if err != nil || len(m.Slices) == 0 {
		return ""
	}
	done, inProg, planned := 0, 0, 0
	for _, sl := range m.Slices {
		switch sl.Status {
		case evml.SliceDone:
			done++
		case evml.SliceInProgress:
			inProg++
		case evml.SlicePlanned:
			planned++
		}
	}
	if done == 0 && inProg == 0 && planned == 0 {
		return ""
	}
	return fmt.Sprintf("%d done · %d in-progress · %d planned", done, inProg, planned)
}
