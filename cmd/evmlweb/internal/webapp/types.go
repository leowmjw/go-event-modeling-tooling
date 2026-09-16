// Package webapp implements the local event-modeling web UI: a domain
// expert picks a business flow (an existing .evml fixture or a brand new
// one), talks to a local LLM via Kronk to tweak it, and activates a draft
// into testdata/fixtures once happy with it.
package webapp

import "time"

// ChatRole identifies who authored a ChatMessage.
type ChatRole string

const (
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
	RoleSystem    ChatRole = "system"
)

// ChatMessage is one turn in a draft's conversation with the LLM.
type ChatMessage struct {
	Role    ChatRole
	Content string
	At      time.Time
}

// DraftVersion is one dated, numbered iteration of a flow's event model.
// Its ID has the form "<flow>-<date>-v<seq>", e.g. "hotel-booking-2026-08-10-v2".
type DraftVersion struct {
	ID         string
	FlowName   string
	Date       string // YYYY-MM-DD, the date this draft was created
	Seq        int
	EvmlSource string
	SVG        string
	ParseError string // non-empty when EvmlSource fails to parse
	// ValidationIssues holds non-blocking wiring problems (from
	// evml.ValidateConnections) as joined text. The diagram still renders
	// with issues; activation is blocked until they're resolved.
	ValidationIssues string
	// Label is the expert-given scenario name ("Fraud screening — Q3
	// plan"), shown on the draft tab instead of v<seq> when set.
	Label string
	// Intent records where this draft sits on the staging spectrum:
	// "exploring" (default), "future", or "ready".
	Intent     string
	Transcript []ChatMessage
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// FlowState tracks one business flow: its on-disk baseline (if any) plus
// every in-progress draft version, so switching away and back restores
// exactly where the expert left off.
type FlowState struct {
	Name           string // fixture-file slug, e.g. "hotel-booking"
	BaselineEvml   string // "" for a brand new, not-yet-activated flow
	BaselineSVG    string
	IsNew          bool
	Drafts         map[string]*DraftVersion
	DraftOrder     []string // stable tab order, oldest first
	ActiveDraftID  string
	NextSeqForDate map[string]int // date -> next sequence number

	// SelectedFrame is the frame (by ID) whose detail panel is open.
	// Ephemeral view state — never persisted.
	SelectedFrame string
	// CompareMode is "" (off), "baseline", or "prev": whether the SVG is
	// rendered with diff highlighting against the flow baseline or the
	// previous draft. Ephemeral view state — never persisted.
	CompareMode string
	// PanelError carries the last frame-edit failure into the detail
	// panel so the expert sees what went wrong. Ephemeral.
	PanelError string
}
