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
	ParseError string // non-empty when EvmlSource fails to parse/validate
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
}
