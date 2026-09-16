package webapp

import (
	"strings"
	"testing"
	"time"
)

func TestNewDraftDefaultsToNextLane(t *testing.T) {
	app := testApp(t)
	fs := &FlowState{Name: "flow", Drafts: make(map[string]*DraftVersion), NextSeqForDate: make(map[string]int)}

	d, err := app.sessions.NewDraft(fs, nil, time.Now())
	if err != nil {
		t.Fatalf("NewDraft: %v", err)
	}
	if d.Horizon != HorizonNext {
		t.Fatalf("Horizon = %q, want next", d.Horizon)
	}
	if d.Status != StatusStaging {
		t.Fatalf("Status = %q, want staging", d.Status)
	}
}

func TestDraftHorizonRoundTripsThroughStore(t *testing.T) {
	app := testApp(t)
	d := &DraftVersion{
		ID: "flow-2026-09-15-v1", FlowName: "flow", Date: "2026-09-15", Seq: 1,
		Title: "Step-up check", Horizon: HorizonFuture, Status: StatusStaging,
		EvmlSource: "eventmodeling\n", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := app.store.Save(d); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := app.store.LoadFlow("flow")
	if err != nil {
		t.Fatalf("LoadFlow: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("got %d drafts, want 1", len(loaded))
	}
	if loaded[0].Horizon != HorizonFuture || loaded[0].Title != "Step-up check" {
		t.Fatalf("round trip lost staging fields: %+v", loaded[0])
	}
}

func TestOldDraftsDefaultToNextLane(t *testing.T) {
	app := testApp(t)
	d := &DraftVersion{
		ID: "flow-2026-09-15-v1", FlowName: "flow", Date: "2026-09-15", Seq: 1,
		EvmlSource: "eventmodeling\n", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := app.store.Save(d); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := app.store.LoadFlow("flow")
	if err != nil {
		t.Fatalf("LoadFlow: %v", err)
	}
	if loaded[0].Horizon != HorizonNext || loaded[0].Status != StatusStaging {
		t.Fatalf("old draft should load as next/staging, got %+v", loaded[0])
	}
}

func TestDiffEvmlGroupsChanges(t *testing.T) {
	rows := diffEvml("eventmodeling\ntf 01 ui Screen\n", "eventmodeling\ntf 01 ui Screen\ntf 02 cmd Go\n")
	var added int
	for _, r := range rows {
		if r.Kind == "added" {
			added++
		}
	}
	if added != 1 {
		t.Fatalf("want 1 added row, got %d (%v)", added, rows)
	}
}

func TestFriendlyFlowName(t *testing.T) {
	if got := friendlyFlowName("fintech-p2p-payment"); got != "Fintech P2p Payment" {
		t.Fatalf("friendlyFlowName = %q", got)
	}
}

func TestChatContentCollapsesEvml(t *testing.T) {
	html := string(chatContentHTML("Changed the flow.\n```evml\neventmodeling\n```"))
	if !strings.Contains(html, "Show diagram changes") {
		t.Fatalf("evml block should collapse, got: %s", html)
	}
	if strings.Contains(html, "<script") {
		t.Fatalf("chat content must be escaped, got: %s", html)
	}
}

func TestWorkspaceSplitsLanes(t *testing.T) {
	app := testApp(t)
	s := &Session{Token: "t", Flows: make(map[string]*FlowState), ActiveFlow: "flow"}
	fs := &FlowState{
		Name: "flow",
		Drafts: map[string]*DraftVersion{
			"a": {ID: "a", FlowName: "flow", Horizon: HorizonNow, Seq: 1},
			"b": {ID: "b", FlowName: "flow", Horizon: HorizonFuture, Seq: 2},
		},
		DraftOrder:     []string{"a", "b"},
		ActiveDraftID:  "a",
		NextSeqForDate: make(map[string]int),
	}
	s.Flows["flow"] = fs
	page, err := app.buildPage(s)
	if err != nil {
		t.Fatalf("buildPage: %v", err)
	}
	if len(page.NowDrafts) != 1 || len(page.FutureDrafts) != 1 || len(page.NextDrafts) != 0 {
		t.Fatalf("lane split wrong: %+v", page)
	}
}
