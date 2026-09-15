package webapp

import (
	"strings"
	"testing"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

const editFixture = `eventmodeling

slice "Top up" 01-03 stage staging

tf 01 ui TopUpScreen @Customer
tf 02 cmd TopUpWallet { walletId: "w-1", amount: 50.00 }
tf 03 evt WalletToppedUp { walletId: "w-1" }

hotspot 02 {
  Is there a minimum?
}

gwt 02 "happy"
  given
    evt WalletOpened
  when
    cmd TopUpWallet
  then
    evt WalletToppedUp
`

func mustParse(t *testing.T, src string) *evml.Model {
	t.Helper()
	m, err := evml.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, src)
	}
	return m
}

func TestFormatFrameLine(t *testing.T) {
	line, err := FormatFrameLine("04", StepInput{Type: "read model", Name: "wallet balance", Payload: `balance: 150`, Actor: "customer", Stage: ""})
	if err != nil {
		t.Fatal(err)
	}
	if line != `tf 04 rmo WalletBalance @Customer { balance: 150 } #staging` {
		t.Fatalf("line = %q", line)
	}
	line, _ = FormatFrameLine("05", StepInput{Type: "evt", Name: "Scheme.Accepted", Reset: true, Stage: "current"})
	if line != `rf 05 evt Scheme.Accepted` {
		t.Fatalf("line = %q", line)
	}
	if _, err := FormatFrameLine("06", StepInput{Type: "widget", Name: "X"}); err == nil {
		t.Fatal("expected unknown type error")
	}
	if _, err := FormatFrameLine("06", StepInput{Type: "cmd", Name: "X", Payload: "{ a: 1"}); err == nil {
		t.Fatal("expected unbalanced payload error")
	}
}

func TestInsertAfterFrameAndReparse(t *testing.T) {
	m := mustParse(t, editFixture)
	line, err := FormatFrameLine(NextFrameID(m), StepInput{Type: "rmo", Name: "WalletBalance", Stage: "staging"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := InsertAfterFrame(editFixture, m, "03", line)
	if err != nil {
		t.Fatal(err)
	}
	m2 := mustParse(t, out)
	if len(m2.Frames) != 4 || m2.Frames[3].ID != "04" || m2.Frames[3].Line != 8 {
		t.Fatalf("frames after insert = %+v", m2.Frames[3])
	}
	if m2.FrameStage(m2.Frames[3]) != evml.StageStaging {
		t.Fatal("inserted frame should be staging")
	}
}

func TestResolveHotspotToNote(t *testing.T) {
	m := mustParse(t, editFixture)
	h := m.Hotspots[0]
	out, err := ReplaceLines(editFixture, h.Line, h.LineCount, FormatNote(h.SourceID, "Decided: minimum is MYR 10."))
	if err != nil {
		t.Fatal(err)
	}
	m2 := mustParse(t, out)
	if len(m2.Hotspots) != 0 || len(m2.NoteEntities) != 1 {
		t.Fatalf("hotspots=%d notes=%d", len(m2.Hotspots), len(m2.NoteEntities))
	}
	if !strings.Contains(m2.NoteEntities[0].Value, "MYR 10") {
		t.Fatalf("note = %q", m2.NoteEntities[0].Value)
	}
}

func TestSetFrameStagePromotesOutOfStagingSlice(t *testing.T) {
	m := mustParse(t, editFixture)
	f := m.FrameByID("02")
	out, err := SetFrameStage(editFixture, m, f, evml.StageCurrent)
	if err != nil {
		t.Fatal(err)
	}
	m2 := mustParse(t, out)
	if got := m2.FrameStage(m2.FrameByID("02")); got != evml.StageCurrent {
		t.Fatalf("stage = %s, want current (explicit #current)", got)
	}
	// Setting back to the inherited stage drops the tag again.
	out2, _ := SetFrameStage(out, m2, m2.FrameByID("02"), evml.StageStaging)
	if strings.Contains(out2, "#") {
		t.Fatalf("expected tag removed, got:\n%s", out2)
	}
}

func TestRemoveFrameDropsAnchoredBlocks(t *testing.T) {
	m := mustParse(t, editFixture)
	out, err := RemoveFrame(editFixture, m, m.FrameByID("02"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "hotspot 02") || strings.Contains(out, "gwt 02") || strings.Contains(out, "tf 02") {
		t.Fatalf("frame 02 remnants left:\n%s", out)
	}
	m2 := mustParse(t, out)
	if len(m2.Frames) != 2 {
		t.Fatalf("frames = %d", len(m2.Frames))
	}
}

func TestRemoveFrameShrinksRangesAndSources(t *testing.T) {
	src := `eventmodeling
chapter "All" 01-04
slice "Only three" 03-03
slice "Tail" 03-04
tf 01 ui X
tf 02 cmd Y
tf 03 evt Z
tf 04 pcr Bot ->> 03 ->> 02
`
	m := mustParse(t, src)
	out, err := RemoveFrame(src, m, m.FrameByID("03"))
	if err != nil {
		t.Fatal(err)
	}
	m2 := mustParse(t, out)
	if len(m2.Slices) != 1 || m2.Slices[0].StartID != "04" || m2.Slices[0].EndID != "04" {
		t.Fatalf("slices after removal = %+v\n%s", m2.Slices, out)
	}
	if m2.Chapters[0].StartID != "01" || m2.Chapters[0].EndID != "04" {
		t.Fatalf("chapter after removal = %+v", m2.Chapters[0])
	}
	bot := m2.FrameByID("04")
	if len(bot.SourceIDs) != 1 || bot.SourceIDs[0] != "02" {
		t.Fatalf("bot sources = %v\n%s", bot.SourceIDs, out)
	}
}

func TestFormatScenarioAndSlice(t *testing.T) {
	block, err := FormatScenario(ScenarioInput{
		FrameID: "02", Label: `reject "big" top-up`,
		Given: "event WalletOpened\n", When: "command TopUpWallet amount: 10000", Then: "event TopUpRejected { reason: \"over_limit\" }",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := mustParse(t, AppendBlock(editFixture, block))
	if len(m.GWTs) != 2 || m.GWTs[1].Label != `"reject 'big' top-up"` {
		t.Fatalf("gwts = %+v", m.GWTs)
	}
	if m.GWTs[1].When[0].Data != "{ amount: 10000 }" {
		t.Fatalf("when payload = %q", m.GWTs[1].When[0].Data)
	}
	if _, err := FormatScenario(ScenarioInput{FrameID: "02", Given: "evt X"}); err == nil {
		t.Fatal("expected missing Then error")
	}

	line, err := FormatSlice(SliceInput{Name: "Top up", Start: "01", End: "03", Status: "in-progress", Stage: "proposed"})
	if err != nil {
		t.Fatal(err)
	}
	if line != `slice "Top up" 01-03 status InProgress stage staging` {
		t.Fatalf("slice = %q", line)
	}
}

func TestNormalizeIdentifier(t *testing.T) {
	cases := map[string]string{
		"approve loan offer": "ApproveLoanOffer",
		"Scheme.Accepted":    "Scheme.Accepted",
		"KYC-passed!":        "KYCPassed",
		"payment_settled":    "payment_settled",
	}
	for in, want := range cases {
		got, err := NormalizeIdentifier(in)
		if err != nil || got != want {
			t.Errorf("NormalizeIdentifier(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := NormalizeIdentifier("!!!"); err == nil {
		t.Fatal("expected error for unusable name")
	}
}
