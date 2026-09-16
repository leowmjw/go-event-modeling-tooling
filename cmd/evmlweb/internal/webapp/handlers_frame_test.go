package webapp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// stagingSession builds an app + session with one flow ("pay") whose
// baseline and single draft both hold the editBase model, registered in
// the session store so handlers can find it by cookie.
func stagingSession(t *testing.T) (*App, *Session, *FlowState, *DraftVersion) {
	t.Helper()
	app := testApp(t)
	s := &Session{Token: "test-token", ModelID: "stub-model", Flows: map[string]*FlowState{}}
	fs := &FlowState{
		Name:           "pay",
		BaselineEvml:   editBase,
		Drafts:         map[string]*DraftVersion{},
		NextSeqForDate: map[string]int{},
	}
	d := &DraftVersion{ID: "pay-2026-09-15-v1", FlowName: "pay", Date: "2026-09-15", Seq: 1, EvmlSource: editBase}
	svg, parseErr, issues := evaluateEvml(editBase)
	if parseErr != "" || len(issues) > 0 {
		t.Fatalf("staging fixture must be clean: %s / %v", parseErr, issues)
	}
	d.SVG = svg
	fs.Drafts[d.ID] = d
	fs.DraftOrder = []string{d.ID}
	fs.ActiveDraftID = d.ID
	s.Flows["pay"] = fs
	s.ActiveFlow = "pay"
	app.sessions.mu.Lock()
	app.sessions.sessions[s.Token] = s
	app.sessions.mu.Unlock()
	return app, s, fs, d
}

func postSignals(t *testing.T, app *App, s *Session, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: s.Token})
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)
	return rec
}

func TestFrameSelectShowsPanel(t *testing.T) {
	app, s, fs, _ := stagingSession(t)
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/select", map[string]string{"frame": "02"})
	if rec.Code != http.StatusOK {
		t.Fatalf("frame select status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fs.SelectedFrame != "02" {
		t.Fatalf("SelectedFrame = %q, want 02", fs.SelectedFrame)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="frame-panel"`) {
		t.Fatalf("patch missing frame panel: %s", body[:min(600, len(body))])
	}
	if !strings.Contains(body, "Payments.InitiatePayment") {
		t.Fatalf("panel missing frame identifier: %s", body[:min(600, len(body))])
	}
}

func TestFrameRenameViaPanel(t *testing.T) {
	app, s, _, d := stagingSession(t)
	postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/select", map[string]string{"frame": "02"})
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/update", map[string]string{"name": "Payments.RequestPayment"})
	if rec.Code != http.StatusOK {
		t.Fatalf("frame update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(d.EvmlSource, "tf 02 cmd Payments.RequestPayment ->> 01") {
		t.Fatalf("draft source not renamed:\n%s", d.EvmlSource)
	}
	last := d.Transcript[len(d.Transcript)-1]
	if last.Role != RoleSystem || !strings.Contains(last.Content, "Renamed frame 02 to Payments.RequestPayment") {
		t.Fatalf("expected rename transcript note, got %+v", last)
	}
}

func TestFrameAddViaPanel(t *testing.T) {
	app, s, fs, d := stagingSession(t)
	postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/select", map[string]string{"frame": "03"})
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/add", map[string]string{"type": "pcr", "name": "Payments.Authorizer", "after": "03"})
	if rec.Code != http.StatusOK {
		t.Fatalf("frame add status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(d.EvmlSource, "tf 06 pcr Payments.Authorizer ->> 03") {
		t.Fatalf("added frame missing from source:\n%s", d.EvmlSource)
	}
	if fs.SelectedFrame != "06" {
		t.Fatalf("new frame should be selected, got %q", fs.SelectedFrame)
	}
}

func TestFrameDeleteViaPanel(t *testing.T) {
	app, s, fs, d := stagingSession(t)
	postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/select", map[string]string{"frame": "03"})
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/delete", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("frame delete status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(d.EvmlSource, "tf 03 evt") {
		t.Fatalf("frame 03 should be gone:\n%s", d.EvmlSource)
	}
	if fs.SelectedFrame != "" {
		t.Fatalf("selection should clear after delete, got %q", fs.SelectedFrame)
	}
	// Frame 04's ->> 03 reference must be stripped too.
	if strings.Contains(d.EvmlSource, "->> 03") {
		t.Fatalf("stale reference to 03 remains:\n%s", d.EvmlSource)
	}
}

func TestFrameDeleteRefusesWhenGwtAttached(t *testing.T) {
	app, s, fs, d := stagingSession(t)
	postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/select", map[string]string{"frame": "02"})
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/delete", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("refused delete should still patch the workspace, status = %d", rec.Code)
	}
	if !strings.Contains(d.EvmlSource, "tf 02 cmd") {
		t.Fatal("refused delete must not change the source")
	}
	if !strings.Contains(fs.PanelError, "scenario") {
		t.Fatalf("panel should explain the refusal, got %q", fs.PanelError)
	}
}

func TestCompareBaselineHighlightsDiff(t *testing.T) {
	app, s, fs, _ := stagingSession(t)
	// Add a frame so the draft differs from the baseline, then close the
	// panel (selection would override the diff colour on that frame).
	postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/select", map[string]string{"frame": "03"})
	postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/add", map[string]string{"type": "pcr", "name": "Payments.Authorizer", "after": "03"})
	postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/frame/deselect", nil)

	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/compare", map[string]string{"mode": "baseline"})
	if rec.Code != http.StatusOK {
		t.Fatalf("compare status = %d", rec.Code)
	}
	if fs.CompareMode != "baseline" {
		t.Fatalf("CompareMode = %q", fs.CompareMode)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "legend-added") {
		t.Fatalf("patch missing added legend: %s", body[:min(600, len(body))])
	}
	// The SVG patch itself must carry the diff highlight.
	if !strings.Contains(body, `fill="#e6f4e6"`) {
		t.Fatalf("svg missing added highlight: %s", body[:min(600, len(body))])
	}
}

func TestDraftMetaSave(t *testing.T) {
	app, s, _, d := stagingSession(t)
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/meta", map[string]string{"label": "Fraud screening", "intent": "future"})
	if rec.Code != http.StatusOK {
		t.Fatalf("meta status = %d", rec.Code)
	}
	if d.Label != "Fraud screening" || d.Intent != "future" {
		t.Fatalf("label/intent = %q/%q", d.Label, d.Intent)
	}
	// Persisted sidecar carries them.
	drafts, err := app.store.LoadFlow("pay")
	if err != nil {
		t.Fatalf("LoadFlow: %v", err)
	}
	var loaded *DraftVersion
	for _, cand := range drafts {
		if cand.ID == d.ID {
			loaded = cand
		}
	}
	if loaded == nil {
		t.Fatalf("draft %s not persisted", d.ID)
	}
	if loaded.Label != "Fraud screening" || loaded.Intent != "future" {
		t.Fatalf("persisted label/intent = %q/%q", loaded.Label, loaded.Intent)
	}
}

func TestActivateBlockedOnWiringIssues(t *testing.T) {
	app, s, _, d := stagingSession(t)
	d.ValidationIssues = "a command can only receive input from a ui or processor"
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/activate", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("activate status = %d, want 409", rec.Code)
	}
}

func TestSeedFlowSeedsCommittedDrafts(t *testing.T) {
	app := testApp(t)
	seedStore, err := NewDraftStore("../../seed", app.log)
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	app.seedStore = seedStore

	fs := &FlowState{Name: "fintech-payment-lifecycle", Drafts: map[string]*DraftVersion{}, NextSeqForDate: map[string]int{}}
	if err := app.ensureDraft(fs); err != nil {
		t.Fatalf("ensureDraft: %v", err)
	}
	if len(fs.DraftOrder) != 2 {
		t.Fatalf("seeded drafts = %v, want 2", fs.DraftOrder)
	}
	if fs.ActiveDraftID != "fintech-payment-lifecycle-2026-09-15-v3" {
		t.Fatalf("active draft = %q, want the newest seed", fs.ActiveDraftID)
	}
	for _, id := range fs.DraftOrder {
		d := fs.Drafts[id]
		if d.SVG == "" || d.ParseError != "" || d.ValidationIssues != "" {
			t.Fatalf("seed %s not clean: svg=%d parse=%q issues=%q", id, len(d.SVG), d.ParseError, d.ValidationIssues)
		}
		if d.Label == "" {
			t.Fatalf("seed %s missing label", id)
		}
	}
	if fs.Drafts[fs.DraftOrder[0]].Label != "Fraud screening (Q3 plan)" {
		t.Fatalf("v2 label = %q", fs.Drafts[fs.DraftOrder[0]].Label)
	}

	// ensureDraft must never duplicate seeds on a second call.
	if err := app.ensureDraft(fs); err != nil {
		t.Fatalf("second ensureDraft: %v", err)
	}
	if len(fs.DraftOrder) != 2 {
		t.Fatalf("second ensureDraft duplicated drafts: %v", fs.DraftOrder)
	}
}

func TestChatRepairLoopRecoversParseError(t *testing.T) {
	app, s, _, d := stagingSession(t)

	var calls int
	var mu sync.Mutex
	app.chatFn = func(ctx context.Context, modelID, systemPrompt string, transcript []ChatMessage, onDelta func(string) error) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return "Here you go:\n```evml\neventmodeling\ntf 01 cmd Bad ->> 99\n```", nil
		}
		return "Fixed:\n```evml\neventmodeling\ntf 01 ui Screen\ntf 02 cmd Go ->> 01\ntf 03 evt Went ->> 02\n```", nil
	}

	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/chat", map[string]string{"message": "make it work"})
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if calls != 2 {
		t.Fatalf("chatFn calls = %d, want 2 (one repair round)", calls)
	}
	if !strings.Contains(d.EvmlSource, "evt Went") {
		t.Fatalf("repaired source not applied:\n%s", d.EvmlSource)
	}
	if d.ParseError != "" {
		t.Fatalf("ParseError should be cleared, got %q", d.ParseError)
	}
	var sawCorrection bool
	for _, m := range d.Transcript {
		if m.Role == RoleSystem && strings.Contains(m.Content, "problem") {
			sawCorrection = true
		}
	}
	if !sawCorrection {
		t.Fatal("transcript should record the repair correction")
	}
}

func TestChatRepairLoopRecoversWiringIssue(t *testing.T) {
	app, s, _, d := stagingSession(t)

	var calls int
	var mu sync.Mutex
	app.chatFn = func(ctx context.Context, modelID, systemPrompt string, transcript []ChatMessage, onDelta func(string) error) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			// Parses fine, but a command may not receive input from an event.
			return "Draft:\n```evml\neventmodeling\ntf 01 evt Start\ntf 02 cmd Wrong ->> 01\n```", nil
		}
		return "Fixed:\n```evml\neventmodeling\ntf 01 ui Screen\ntf 02 cmd Right ->> 01\n```", nil
	}

	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/chat", map[string]string{"message": "model a simple flow"})
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if calls != 2 {
		t.Fatalf("chatFn calls = %d, want 2 (one repair round)", calls)
	}
	if !strings.Contains(d.EvmlSource, "cmd Right") {
		t.Fatalf("repaired source not applied:\n%s", d.EvmlSource)
	}
	if d.ValidationIssues != "" {
		t.Fatalf("ValidationIssues should be cleared, got %q", d.ValidationIssues)
	}
}

func TestChatRepairLoopGivesUpAfterMaxAttempts(t *testing.T) {
	app, s, _, d := stagingSession(t)

	var calls int
	var mu sync.Mutex
	app.chatFn = func(ctx context.Context, modelID, systemPrompt string, transcript []ChatMessage, onDelta func(string) error) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return "Broken:\n```evml\neventmodeling\ntf 01 cmd Bad ->> 99\n```", nil
	}

	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/chat", map[string]string{"message": "try again"})
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d", rec.Code)
	}
	if calls != maxRepairAttempts+1 {
		t.Fatalf("chatFn calls = %d, want %d", calls, maxRepairAttempts+1)
	}
	if d.ParseError == "" {
		t.Fatal("final failure should surface a parse error")
	}
	if !strings.Contains(d.EvmlSource, "Payments.SendMoneyScreen") {
		t.Fatal("last good source must be kept")
	}
}

func TestFramePanelTemplateUsesColonSyntax(t *testing.T) {
	app := testApp(t)
	page := WorkspacePage{
		ActiveFlow:    "pay",
		ActiveDraftID: "pay-2026-09-15-v1",
		Drafts:        []DraftTab{{ID: "pay-2026-09-15-v1", Label: "v1"}},
		FramePanel: &FramePanelView{
			FlowName:   "pay",
			DraftID:    "pay-2026-09-15-v1",
			ID:         "02",
			Kind:       "timeframe",
			EntityType: "cmd",
			Identifier: "Payments.InitiatePayment",
			Payload:    `paymentId: "p-1"`,
			Choices:    []FrameChoice{{ID: "01", Identifier: "Payments.SendMoneyScreen", EntityType: "ui"}},
		},
	}
	b, err := app.renderTemplateToBytes("workspace", page)
	if err != nil {
		t.Fatalf("render workspace: %v", err)
	}
	html := string(b)
	for _, attr := range []string{
		"data-on:submit__prevent",
		"data-bind:frame-name",
		"data-bind:frame-payload",
		"data-on:mousedown__stop",
		"data-on:click",
	} {
		if !strings.Contains(html, attr) {
			t.Fatalf("workspace template missing %s", attr)
		}
	}
	for _, deprecated := range []string{"data-bind-frame", "data-on-mousedown", "data-on-submit"} {
		if strings.Contains(html, deprecated) {
			t.Fatalf("workspace template still uses deprecated %s", deprecated)
		}
	}
}

func TestNewWhatIfCarriesLabelAndIntent(t *testing.T) {
	app, s, fs, d := stagingSession(t)
	rec := postSignals(t, app, s, "/flow/pay/draft/pay-2026-09-15-v1/new-version", map[string]string{"label": "Rail outage plan", "intent": "ready"})
	if rec.Code != http.StatusOK {
		t.Fatalf("new-version status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fs.ActiveDraftID == d.ID {
		t.Fatal("new draft should become active")
	}
	nd := fs.Drafts[fs.ActiveDraftID]
	if nd.Label != "Rail outage plan" || nd.Intent != "ready" {
		t.Fatalf("new draft label/intent = %q/%q", nd.Label, nd.Intent)
	}
	if !strings.Contains(nd.EvmlSource, "Payments.SendMoneyScreen") {
		t.Fatal("new draft should fork the active draft's source")
	}
	if nd.CreatedAt.IsZero() || time.Since(nd.CreatedAt) > time.Minute {
		t.Fatalf("CreatedAt = %v, want ~now", nd.CreatedAt)
	}
}
