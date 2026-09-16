package webapp

import (
	"fmt"
	"html"
	"html/template"
	"strings"
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
	ID     string
	Label  string // scenario label when set, else "v1", "v2", …
	Intent string // "", "exploring", "future", or "ready"
}

// FrameChoice is one frame in the connect picker of the frame panel.
type FrameChoice struct {
	ID          string
	Identifier  string
	EntityType  string
	IsConnected bool
}

// FramePanelView feeds the frame detail panel: everything the expert
// needs to inspect and tweak one frame of the active draft.
type FramePanelView struct {
	FlowName string
	DraftID  string

	ID          string
	Kind        string // "timeframe" or "resetframe"
	EntityType  string
	Identifier  string
	Namespace   string
	SourceIDs   []string
	DataRefName string
	Payload     string // inner payload content (no outer braces)
	GWTLabels   []string
	Choices     []FrameChoice // every frame, for the connect picker
	EditError   string
}

// RemovedFrameView lists a frame that exists in the compare baseline but
// not in the active draft (removed frames are listed, not ghost-drawn).
type RemovedFrameView struct {
	ID         string
	Identifier string
}

// DiffLegendView describes one diff highlight state for the legend.
type DiffLegendView struct {
	State string // "added" | "changed" | "removed"
	Count int
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

	// Staging state for the active draft.
	DraftLabel       string
	DraftIntent      string
	ValidationIssues string
	FramePanel       *FramePanelView
	CompareMode      string
	DiffLegend       []DiffLegendView
	RemovedFrames    []RemovedFrameView
	PromptChips      []string

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
