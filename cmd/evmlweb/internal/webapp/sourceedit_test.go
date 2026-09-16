package webapp

import (
	"strings"
	"testing"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

const editBase = `eventmodeling
// payment flow under construction

entity InstantPayment

tf 01 ui Payments.SendMoneyScreen
tf 02 cmd Payments.InitiatePayment ->> 01 { paymentId: "p-1", amount: 25.00 }
tf 03 evt Payments.PaymentInitiated ->> 02 { paymentId: "p-1", amount: 25.00 }
tf 04 rmo Payments.PaymentHistory [[PaymentHistory]] ->> 03
tf 05 ui Payments.PaymentStatusScreen ->> 04

data PaymentHistory ` + "`json`" + ` {
  paymentId: "p-1",
  status: "initiated"
}

note 02 { ask payments about ids }

gwt 02 "happy path"
  given
    evt Payments.PaymentInitiated { paymentId: "p-1" }
  then
    evt Payments.PaymentInitiated { paymentId: "p-1" }
`

func mustEditor(t *testing.T, src string) *SourceEditor {
	t.Helper()
	e, err := NewSourceEditor(src)
	if err != nil {
		t.Fatalf("NewSourceEditor() error = %v", err)
	}
	return e
}

// reparse round-trips the editor's output back through the parser so each
// test asserts the *semantic* result, not string trivia.
func reparse(t *testing.T, e *SourceEditor) *evml.Model {
	t.Helper()
	m, err := evml.Parse(e.Source())
	if err != nil {
		t.Fatalf("edited source no longer parses: %v\nsource:\n%s", err, e.Source())
	}
	return m
}

func frameByID(t *testing.T, m *evml.Model, id string) *evml.Frame {
	t.Helper()
	for _, f := range m.Frames {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("no frame %s in edited model", id)
	return nil
}

func TestSourceEditorAddFrameAfterAnchor(t *testing.T) {
	e := mustEditor(t, editBase)
	newID, err := e.AddFrame("03", evml.EntityProcessor, "Payments.PaymentAuthorizer", "decision: true")
	if err != nil {
		t.Fatalf("AddFrame() error = %v", err)
	}
	if newID != "06" {
		t.Fatalf("new frame ID = %q, want 06", newID)
	}
	m := reparse(t, e)
	f := frameByID(t, m, "06")
	if f.EntityType != evml.EntityProcessor || f.Identifier != "Payments.PaymentAuthorizer" {
		t.Fatalf("unexpected new frame: %+v", f)
	}
	// evt ->> pcr is legal, so the new frame wires to the anchor.
	if len(f.SourceIDs) != 1 || f.SourceIDs[0] != "03" {
		t.Fatalf("new frame sources = %v, want [03]", f.SourceIDs)
	}
	if f.Data != `{ decision: true }` {
		t.Fatalf("new frame payload = %q", f.Data)
	}
	// Inserted after frame 03's line: it becomes the 4th frame in the
	// timeline.
	if m.Frames[3].ID != "06" {
		t.Fatalf("frame order = %v, want 06 in position 3", frameIDs(m))
	}
	// Untouched lines survive verbatim.
	for _, want := range []string{"// payment flow under construction", "note 02 { ask payments about ids }", "  status: \"initiated\""} {
		if !strings.Contains(e.Source(), want) {
			t.Fatalf("edited source lost line %q:\n%s", want, e.Source())
		}
	}
}

func TestSourceEditorAddFrameIllegalConnectionOmitsSource(t *testing.T) {
	e := mustEditor(t, editBase)
	// cmd from evt anchor is illegal (commands take ui/pcr) — the frame
	// is added without wiring instead of failing.
	newID, err := e.AddFrame("03", evml.EntityCommand, "Payments.CancelPayment", "")
	if err != nil {
		t.Fatalf("AddFrame() error = %v", err)
	}
	f := frameByID(t, reparse(t, e), newID)
	if len(f.SourceIDs) != 0 {
		t.Fatalf("sources = %v, want none", f.SourceIDs)
	}
}

func TestSourceEditorAddFrameRejectsMultiLinePayload(t *testing.T) {
	e := mustEditor(t, editBase)
	if _, err := e.AddFrame("03", evml.EntityEvent, "Payments.X", "a: 1\nb: 2"); err == nil {
		t.Fatal("expected error for multi-line frame payload")
	}
}

func TestSourceEditorRenameFrame(t *testing.T) {
	e := mustEditor(t, editBase)
	if err := e.RenameFrame("02", "Payments.RequestPayment"); err != nil {
		t.Fatalf("RenameFrame() error = %v", err)
	}
	f := frameByID(t, reparse(t, e), "02")
	if f.Identifier != "Payments.RequestPayment" {
		t.Fatalf("identifier = %q", f.Identifier)
	}
	// Sources and payload survive the rewrite.
	if len(f.SourceIDs) != 1 || f.SourceIDs[0] != "01" {
		t.Fatalf("sources = %v", f.SourceIDs)
	}
	if !strings.Contains(f.Data, `"p-1"`) {
		t.Fatalf("payload lost: %q", f.Data)
	}
}

func TestSourceEditorSetFramePayloadInline(t *testing.T) {
	e := mustEditor(t, editBase)
	if err := e.SetFramePayload("02", `paymentId: "p-9"`); err != nil {
		t.Fatalf("SetFramePayload() error = %v", err)
	}
	f := frameByID(t, reparse(t, e), "02")
	if f.Data != `{ paymentId: "p-9" }` {
		t.Fatalf("payload = %q", f.Data)
	}
}

func TestSourceEditorSetFramePayloadViaDataBlock(t *testing.T) {
	e := mustEditor(t, editBase)
	// Frame 04 references [[PaymentHistory]] — the data block's value is
	// edited, and multi-line payloads are allowed there.
	if err := e.SetFramePayload("04", "paymentId: \"p-1\",\nstatus: \"settled\""); err != nil {
		t.Fatalf("SetFramePayload() error = %v", err)
	}
	f := frameByID(t, reparse(t, e), "04")
	if f.DataRefName != "PaymentHistory" {
		t.Fatalf("data ref lost: %q", f.DataRefName)
	}
	if !strings.Contains(f.DisplayData(), `status: "settled"`) {
		t.Fatalf("data block not updated: %q", f.DisplayData())
	}
}

func TestSourceEditorSetFramePayloadRejectsUnbalanced(t *testing.T) {
	e := mustEditor(t, editBase)
	if err := e.SetFramePayload("02", `a: { 1`); err == nil {
		t.Fatal("expected error for unbalanced payload")
	}
}

func TestSourceEditorSetFrameSources(t *testing.T) {
	e := mustEditor(t, editBase)
	if err := e.SetFrameSources("05", []string{"04", "02"}); err != nil {
		t.Fatalf("SetFrameSources() error = %v", err)
	}
	// ui sourcing from a cmd is illegal wiring, but the editor performs
	// structural edits; the handler layer's re-validation catches it. Here
	// we only assert the line rewrite.
	f := frameByID(t, reparse(t, e), "05")
	if len(f.SourceIDs) != 2 || f.SourceIDs[0] != "04" || f.SourceIDs[1] != "02" {
		t.Fatalf("sources = %v, want [04 02]", f.SourceIDs)
	}
}

func TestSourceEditorDeleteFrameStripsReferences(t *testing.T) {
	e := mustEditor(t, editBase)
	if err := e.DeleteFrame("03"); err != nil {
		t.Fatalf("DeleteFrame() error = %v", err)
	}
	m := reparse(t, e)
	if frameByID(t, m, "02") == nil || frameByID(t, m, "04") == nil {
		t.Fatal("neighbour frames must survive")
	}
	f := frameByID(t, m, "04")
	if len(f.SourceIDs) != 0 {
		t.Fatalf("frame 04 sources = %v, want stripped", f.SourceIDs)
	}
}

func TestSourceEditorDeleteFrameRefusesWhenGwtAttached(t *testing.T) {
	e := mustEditor(t, editBase)
	if err := e.DeleteFrame("02"); err == nil {
		t.Fatal("expected refusal — frame 02 has a gwt")
	}
	if !strings.Contains(e.Source(), "tf 02 cmd") {
		t.Fatal("source must be unchanged after a refused delete")
	}
}

func TestSourceEditorMoveFrame(t *testing.T) {
	e := mustEditor(t, editBase)
	if err := e.MoveFrame("03", -1); err != nil {
		t.Fatalf("MoveFrame() error = %v", err)
	}
	m := reparse(t, e)
	if m.Frames[1].ID != "03" || m.Frames[2].ID != "02" {
		t.Fatalf("frame order after move = %v", frameIDs(m))
	}
	if err := e.MoveFrame("01", -1); err == nil {
		t.Fatal("expected refusal at start of timeline")
	}
}

func TestSourceEditorNextFrameIDPadding(t *testing.T) {
	e := mustEditor(t, editBase)
	if id, err := e.AddFrame("", evml.EntityEvent, "Payments.Later", ""); err != nil || id != "06" {
		t.Fatalf("AddFrame at end = (%q, %v), want (06, nil)", id, err)
	}
}

func frameIDs(m *evml.Model) []string {
	ids := make([]string, len(m.Frames))
	for i, f := range m.Frames {
		ids[i] = f.ID
	}
	return ids
}
