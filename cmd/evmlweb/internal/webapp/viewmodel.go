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
	IsSystem    bool // true for session-log notes (edits, promotions)
	ContentHTML template.HTML
}

// DraftTab is one entry in the draft-version tab strip.
type DraftTab struct {
	ID    string
	Label string // e.g. "v1", "v2"
	Date  string // YYYY-MM-DD
}

// StepView is one frame in the Steps panel / inspector.
type StepView struct {
	ID        string
	Type      string // ui|cmd|evt|rmo|pcr
	TypeLabel string // Screen|Command|Event|Read model|Automation
	Name      string
	Actor     string
	Stage     string
	Kind      string // timeframe|resetframe
	Payload   string
	Slice     string
	Hotspots  int
	Scenarios int
	Sources   string // "03, 04"
	Hidden    bool   // true when the current lens filters this frame out
}

// HotspotView is one open question in the Questions panel.
type HotspotView struct {
	Index     int // position in Model.Hotspots (stable within one source version)
	FrameID   string
	FrameName string
	Text      string
	Stage     string
}

// DiffLine is one line of the compare panel with a CSS class derived from
// its leading marker (+ added, - removed, ~ changed, » stage, ✓ resolved).
type DiffLine struct {
	Class string
	Text  string
}

func toDiffLines(lines []string) []DiffLine {
	out := make([]DiffLine, 0, len(lines))
	for _, l := range lines {
		class := "other"
		switch {
		case strings.HasPrefix(l, "+"):
			class = "added"
		case strings.HasPrefix(l, "-"):
			class = "removed"
		case strings.HasPrefix(l, "~"):
			class = "changed"
		case strings.HasPrefix(l, "»"):
			class = "stage"
		case strings.HasPrefix(l, "✓"):
			class = "resolved"
		}
		out = append(out, DiffLine{Class: class, Text: l})
	}
	return out
}

// SliceView is one slice/chapter row in the Slices panel.
type SliceView struct {
	Name   string
	Start  string
	End    string
	Status string
	Stage  string
	Frames int
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

	// Session-level workshop state.
	Lens      string // current | staging | all
	EditError string // last rejected edit (transient)
	HasModels bool   // false when no local LLM is available — chat is hidden

	// Derived from the active draft's source.
	Source        string
	Steps         []StepView
	Hotspots      []HotspotView
	Slices        []SliceView
	Chapters      []SliceView
	Actors        []string
	Diff          []DiffLine // vs. the flow baseline
	Lint          []string
	FrameCount    int
	ScenarioCount int
	StagingCount  int
	FutureCount   int
	IsNewFlow     bool
	DraftDate     string
	NextFrameID   string
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
		views = append(views, ChatMessageView{Role: m.Role, IsSystem: m.Role == RoleSystem, ContentHTML: template.HTML(escaped)})
	}
	return views
}

func draftLabel(d *DraftVersion) string {
	return fmt.Sprintf("v%d", d.Seq)
}

func typeLabel(t evml.EntityType) string {
	switch t {
	case evml.EntityUI:
		return "Screen"
	case evml.EntityCommand:
		return "Command"
	case evml.EntityEvent:
		return "Event"
	case evml.EntityReadModel:
		return "Read model"
	case evml.EntityProcessor:
		return "Automation"
	}
	return string(t)
}

// lensStages maps the session lens onto the stages to render.
func lensStages(lens string) []evml.Stage {
	switch lens {
	case "current":
		return []evml.Stage{evml.StageCurrent}
	case "staging":
		return []evml.Stage{evml.StageCurrent, evml.StageStaging}
	default:
		return []evml.Stage{evml.StageCurrent, evml.StageStaging, evml.StageFuture}
	}
}

// describeModel fills the derived, per-draft sections of page from a parsed
// model and the flow baseline.
func describeModel(page *WorkspacePage, m *evml.Model, baseline *evml.Model, lens string) {
	if m == nil {
		return
	}
	visible := map[string]bool{}
	for _, f := range evml.FilterStages(m, lensStages(lens)...).Frames {
		visible[f.ID] = true
	}
	gwtCount := map[string]int{}
	for _, g := range m.GWTs {
		gwtCount[g.SourceID]++
	}
	for _, f := range m.Frames {
		stage := m.FrameStage(f)
		sv := StepView{
			ID:        f.ID,
			Type:      string(f.EntityType),
			TypeLabel: typeLabel(f.EntityType),
			Name:      f.Identifier,
			Actor:     f.Actor,
			Stage:     string(stage),
			Kind:      string(f.Kind),
			Payload:   f.DisplayData(),
			Hotspots:  len(m.HotspotsFor(f.ID)),
			Scenarios: gwtCount[f.ID],
			Sources:   strings.Join(f.SourceIDs, ", "),
			Hidden:    !visible[f.ID],
		}
		if sl := m.SliceFor(f); sl != nil {
			sv.Slice = sl.Name
		}
		switch stage {
		case evml.StageStaging:
			page.StagingCount++
		case evml.StageFuture:
			page.FutureCount++
		}
		page.Steps = append(page.Steps, sv)
	}
	for i, h := range m.Hotspots {
		page.Hotspots = append(page.Hotspots, HotspotView{
			Index:     i,
			FrameID:   h.SourceID,
			FrameName: h.Source.Identifier,
			Text:      strings.TrimSpace(evml.StripOuterBraces(h.Value)),
			Stage:     string(m.FrameStage(h.Source)),
		})
	}
	for _, sl := range m.Slices {
		n := 0
		for _, f := range m.Frames {
			if sl.Contains(f) {
				n++
			}
		}
		page.Slices = append(page.Slices, SliceView{Name: sl.Name, Start: sl.StartID, End: sl.EndID, Status: sl.Status, Stage: string(sl.Stage), Frames: n})
	}
	for _, ch := range m.Chapters {
		n := 0
		for _, f := range m.Frames {
			if ch.Contains(f) {
				n++
			}
		}
		page.Chapters = append(page.Chapters, SliceView{Name: ch.Name, Start: ch.StartID, End: ch.EndID, Frames: n})
	}
	page.Actors = m.Actors
	page.FrameCount = len(m.Frames)
	page.ScenarioCount = len(m.GWTs)
	page.NextFrameID = NextFrameID(m)
	for _, f := range evml.Lint(m) {
		page.Lint = append(page.Lint, f.String())
	}
	if baseline != nil {
		page.Diff = toDiffLines(evml.Diff(baseline, m).Summary())
	}
}
