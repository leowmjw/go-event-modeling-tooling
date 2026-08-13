package webapp

import (
	"log/slog"
	"testing"
	"time"
)

func TestDraftStoreSaveLoadRoundTrip(t *testing.T) {
	store, err := NewDraftStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("NewDraftStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	d := &DraftVersion{
		ID:         NewDraftID("hotel-booking", "2026-08-10", 1),
		FlowName:   "hotel-booking",
		Date:       "2026-08-10",
		Seq:        1,
		EvmlSource: "eventmodeling\ntf 01 ui BookRoomScreen\n",
		ParseError: "",
		Transcript: []ChatMessage{
			{Role: RoleUser, Content: "start with booking a room", At: now},
			{Role: RoleAssistant, Content: "done", At: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.Save(d); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.LoadFlow("hotel-booking")
	if err != nil {
		t.Fatalf("LoadFlow: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadFlow returned %d drafts, want 1", len(loaded))
	}

	got := loaded[0]
	if got.ID != d.ID || got.EvmlSource != d.EvmlSource || got.Seq != d.Seq || got.Date != d.Date {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, d)
	}
	if len(got.Transcript) != 2 || got.Transcript[0].Content != "start with booking a room" {
		t.Fatalf("transcript round trip mismatch: %+v", got.Transcript)
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now) {
		t.Fatalf("timestamp round trip mismatch: got created=%v updated=%v, want %v", got.CreatedAt, got.UpdatedAt, now)
	}
}

func TestDraftStoreLoadFlowMissingIsEmptyNotError(t *testing.T) {
	store, err := NewDraftStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("NewDraftStore: %v", err)
	}
	drafts, err := store.LoadFlow("never-touched")
	if err != nil {
		t.Fatalf("LoadFlow: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("LoadFlow returned %d drafts, want 0", len(drafts))
	}
}

func TestDraftStoreOrdersByDateThenSeq(t *testing.T) {
	store, err := NewDraftStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("NewDraftStore: %v", err)
	}

	mk := func(date string, seq int) *DraftVersion {
		return &DraftVersion{
			ID: NewDraftID("flow", date, seq), FlowName: "flow", Date: date, Seq: seq,
			EvmlSource: "eventmodeling\n", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
	}
	// Save out of order.
	for _, d := range []*DraftVersion{mk("2026-08-11", 1), mk("2026-08-10", 2), mk("2026-08-10", 1)} {
		if err := store.Save(d); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	loaded, err := store.LoadFlow("flow")
	if err != nil {
		t.Fatalf("LoadFlow: %v", err)
	}
	want := []string{
		NewDraftID("flow", "2026-08-10", 1),
		NewDraftID("flow", "2026-08-10", 2),
		NewDraftID("flow", "2026-08-11", 1),
	}
	if len(loaded) != len(want) {
		t.Fatalf("got %d drafts, want %d", len(loaded), len(want))
	}
	for i, d := range loaded {
		if d.ID != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, d.ID, want[i])
		}
	}
}
