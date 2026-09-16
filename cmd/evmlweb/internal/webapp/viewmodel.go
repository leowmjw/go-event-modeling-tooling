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

// DraftTab is one entry in the draft-version tab strip, grouped into
// Now / Next / Future staging lanes.
type DraftTab struct {
	ID      string
	Label   string // e.g. "Fraud hold step-up · v2"
	Horizon Horizon
	Status  DraftStatus
}

// WorkspacePage is the full view model for both the initial page render
// and the workspace SSE fragment.
type WorkspacePage struct {
	ModelID       string
	Models        []ModelChoiceView
	Fixtures      []string
	FixtureNames  map[string]string
	ActiveFlow    string
	ActiveFlowNic string
	Drafts        []DraftTab
	NowDrafts     []DraftTab
	NextDrafts    []DraftTab
	FutureDrafts  []DraftTab
	ActiveDraftID string
	ActiveSVG     template.HTML
	Transcript    []ChatMessageView
	ParseError    string
	Prompts       []string
	DiffFrom      string
	DiffTo        string
	DiffRows      []diffRow
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
		views = append(views, ChatMessageView{Role: m.Role, ContentHTML: chatContentHTML(m.Content)})
	}
	return views
}

// chatContentHTML renders chat text as safe HTML: the plain-language
// explanation stays visible while ```evml blocks collapse behind a
// <details> toggle so domain experts aren't hit with raw DSL.
func chatContentHTML(content string) template.HTML {
	parts := splitEvmlFences(content)
	var b strings.Builder
	for _, p := range parts {
		if p.isEvml {
			b.WriteString(`<details class="evml-toggle"><summary>Show diagram changes</summary><pre>`)
			b.WriteString(html.EscapeString(strings.TrimSpace(p.text)))
			b.WriteString(`</pre></details>`)
			continue
		}
		b.WriteString(html.EscapeString(p.text))
	}
	return template.HTML(strings.ReplaceAll(b.String(), "\n", "<br>"))
}

type chatPart struct {
	text   string
	isEvml bool
}

func splitEvmlFences(content string) []chatPart {
	var parts []chatPart
	rest := content
	for {
		start := strings.Index(rest, "```evml")
		if start < 0 {
			parts = append(parts, chatPart{text: rest})
			return parts
		}
		if start > 0 {
			parts = append(parts, chatPart{text: rest[:start]})
		}
		block := rest[start+len("```evml"):]
		end := strings.Index(block, "```")
		if end < 0 {
			parts = append(parts, chatPart{text: block, isEvml: true})
			return parts
		}
		parts = append(parts, chatPart{text: block[:end], isEvml: true})
		rest = block[end+len("```"):]
	}
}

func draftLabel(d *DraftVersion) string {
	return draftDisplayName(d)
}

func draftDisplayName(d *DraftVersion) string {
	base := fmt.Sprintf("v%d", d.Seq)
	if strings.TrimSpace(d.Title) != "" {
		return fmt.Sprintf("%s · %s", strings.TrimSpace(d.Title), base)
	}
	return base
}

// friendlyFlowName turns a fixture slug into plain language.
func friendlyFlowName(slug string) string {
	t := strings.ReplaceAll(slug, "-", " ")
	t = strings.ReplaceAll(t, "_", " ")
	words := strings.Fields(t)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// starterPrompts suggests plain-language questions per flow so domain
// experts know what to try first.
func starterPrompts(flow string) []string {
	switch {
	case strings.Contains(flow, "fraud"):
		return []string{"What if the fraud score is high?", "Add a manual review step", "Move this to a future goal"}
	case strings.Contains(flow, "kyc"), strings.Contains(flow, "onboarding"):
		return []string{"Add a document re-check", "What if sanctions screening hits?", "Stage the KYB future version"}
	case strings.Contains(flow, "lend"), strings.Contains(flow, "loan"), strings.Contains(flow, "credit"):
		return []string{"Add a counter-offer path", "What if a payment is missed?", "Stage early settlement"}
	case strings.Contains(flow, "ledger"), strings.Contains(flow, "settlement"), strings.Contains(flow, "payment"):
		return []string{"Add a reversal path", "What breaks at end of day?", "Stage multi-currency"}
	case strings.Contains(flow, "consent"), strings.Contains(flow, "openbanking"), strings.Contains(flow, "open-banking"):
		return []string{"What if consent is revoked?", "Add a retry after bank rejection", "Stage recurring consent"}
	default:
		return []string{"Try a variation of this step", "What if this gets rejected?", "Move this idea to Future"}
	}
}
