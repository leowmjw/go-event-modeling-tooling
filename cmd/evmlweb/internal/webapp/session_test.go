package webapp

import (
	"log/slog"
	"testing"
	"time"
)

func TestSessionStoreFlowSwitchIsolatesState(t *testing.T) {
	store, err := NewDraftStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("NewDraftStore: %v", err)
	}
	ss := NewSessionStore(store, slog.Default())
	s := &Session{Flows: make(map[string]*FlowState)}

	flowA, err := ss.FlowFor(s, "flow-a", "eventmodeling\n", "<svg-a/>", false)
	if err != nil {
		t.Fatalf("FlowFor flow-a: %v", err)
	}
	draftA, err := ss.NewDraft(flowA, nil, time.Now())
	if err != nil {
		t.Fatalf("NewDraft flow-a: %v", err)
	}
	draftA.EvmlSource = "eventmodeling\ntf 01 ui A\n"
	if err := store.Save(draftA); err != nil {
		t.Fatalf("Save draftA: %v", err)
	}

	flowB, err := ss.FlowFor(s, "flow-b", "eventmodeling\n", "<svg-b/>", false)
	if err != nil {
		t.Fatalf("FlowFor flow-b: %v", err)
	}
	draftB, err := ss.NewDraft(flowB, nil, time.Now())
	if err != nil {
		t.Fatalf("NewDraft flow-b: %v", err)
	}
	draftB.EvmlSource = "eventmodeling\ntf 01 ui B\n"

	// Switching to flow-b and back to flow-a must not have touched flow-a's
	// active draft or its content.
	again, err := ss.FlowFor(s, "flow-a", "should-not-be-used", "should-not-be-used", false)
	if err != nil {
		t.Fatalf("FlowFor flow-a again: %v", err)
	}
	if again != flowA {
		t.Fatalf("FlowFor returned a different *FlowState on second lookup for the same session")
	}
	if again.ActiveDraftID != draftA.ID {
		t.Fatalf("flow-a active draft = %q, want %q", again.ActiveDraftID, draftA.ID)
	}
	got := again.Drafts[again.ActiveDraftID]
	if got.EvmlSource != "eventmodeling\ntf 01 ui A\n" {
		t.Fatalf("flow-a draft content changed after switching flows: %q", got.EvmlSource)
	}

	// flow-b's own state must be intact too.
	if flowB.ActiveDraftID != draftB.ID {
		t.Fatalf("flow-b active draft = %q, want %q", flowB.ActiveDraftID, draftB.ID)
	}
}

func TestSessionStoreNewDraftForksSourceNotBaseline(t *testing.T) {
	store, err := NewDraftStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("NewDraftStore: %v", err)
	}
	ss := NewSessionStore(store, slog.Default())
	s := &Session{Flows: make(map[string]*FlowState)}

	fs, err := ss.FlowFor(s, "flow", "eventmodeling\n// baseline\n", "<svg-baseline/>", false)
	if err != nil {
		t.Fatalf("FlowFor: %v", err)
	}

	v1, err := ss.NewDraft(fs, nil, time.Now())
	if err != nil {
		t.Fatalf("NewDraft v1: %v", err)
	}
	v1.EvmlSource = "eventmodeling\n// v1 edits\n"

	v2, err := ss.NewDraft(fs, v1, time.Now())
	if err != nil {
		t.Fatalf("NewDraft v2: %v", err)
	}
	if v2.EvmlSource != v1.EvmlSource {
		t.Fatalf("forked draft source = %q, want copy of v1 %q", v2.EvmlSource, v1.EvmlSource)
	}
	if v2.Seq != 2 {
		t.Fatalf("forked draft seq = %d, want 2", v2.Seq)
	}
}
