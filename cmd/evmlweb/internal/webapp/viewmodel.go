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
	ID    string
	Label string // e.g. "v1", "v2"
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
	return fmt.Sprintf("v%d", d.Seq)
}
