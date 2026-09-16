package webapp

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %s: %v", root, err)
	}
	return root
}

// setupSessionAndDraft opens a session, selects an existing fixture flow,
// and returns the session cookie plus the first (v1) draft's ID.
func setupSessionAndDraft(t *testing.T, app *App, flow string) (*http.Cookie, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	app.sessions.ForRequest(rec, req)
	cookie := rec.Result().Cookies()[0]

	body, _ := json.Marshal(map[string]string{"model": "Qwen3-0.6B-Q8_0", "flow": flow})
	sr := httptest.NewRequest(http.MethodPost, "/flow/select", bytes.NewReader(body))
	sr.AddCookie(cookie)
	sr.Header.Set("Content-Type", "application/json")
	scr := httptest.NewRecorder()
	app.handleSelectFlow(scr, sr)
	if scr.Code != http.StatusOK {
		t.Fatalf("select %s: %d %s", flow, scr.Code, scr.Body.String())
	}

	gc := httptest.NewRequest(http.MethodGet, "/", nil)
	gc.AddCookie(cookie)
	grec := httptest.NewRecorder()
	s := app.sessions.ForRequest(grec, gc)
	fs := s.Flows[flow]
	if fs == nil || fs.ActiveDraftID == "" {
		t.Fatalf("no active draft for %q after select", flow)
	}
	return cookie, fs.ActiveDraftID
}

func postJSON(t *testing.T, app *App, cookie *http.Cookie, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)
	return rec
}

func sessionFor(t *testing.T, app *App, cookie *http.Cookie) *Session {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	return app.sessions.ForRequest(rec, req)
}

func draftQuestions(t *testing.T, app *App, cookie *http.Cookie, flow, draftID string) []OpenQuestion {
	t.Helper()
	s := sessionFor(t, app, cookie)
	return append([]OpenQuestion{}, s.Flows[flow].Drafts[draftID].Questions...)
}

func loadDraftFromStore(t *testing.T, app *App, flow, draftID string) *DraftVersion {
	t.Helper()
	drafts, err := app.store.LoadFlow(flow)
	if err != nil {
		t.Fatalf("LoadFlow(%q): %v", flow, err)
	}
	for _, d := range drafts {
		if d.ID == draftID {
			return d
		}
	}
	t.Fatalf("draft %q not persisted to disk", draftID)
	return nil
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestNewVersionReceivesName(t *testing.T) {
	app := testAppWithRepo(t, repoRoot(t))
	cookie, v1 := setupSessionAndDraft(t, app, "simple-block")

	rec := postJSON(t, app, cookie, "/flow/simple-block/draft/"+v1+"/new-version", mustJSON(t, map[string]string{"versionName": "happy path"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("new-version: %d %s", rec.Code, rec.Body.String())
	}

	s := sessionFor(t, app, cookie)
	fs := s.Flows["simple-block"]
	d := fs.Drafts[fs.ActiveDraftID]
	if d.Name != "happy path" {
		t.Fatalf("Name = %q, want \"happy path\"", d.Name)
	}
	if fs.ActiveDraftID == v1 {
		t.Fatal("expected a NEW draft id after new-version")
	}
	if len(d.Questions) != 0 {
		t.Fatalf("expected 0 questions on a fresh version, got %d", len(d.Questions))
	}
}

func TestQuestionStagingLifecycle(t *testing.T) {
	app := testAppWithRepo(t, repoRoot(t))
	cookie, draftID := setupSessionAndDraft(t, app, "simple-block")

	rec := postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/question",
		mustJSON(t, map[string]string{"qText": "do we support refunds?", "qFuture": "open"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("add Q1: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/question",
		mustJSON(t, map[string]string{"qText": "mobile app in v3", "qFuture": "future"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("add Q2: %d %s", rec.Code, rec.Body.String())
	}

	qs := draftQuestions(t, app, cookie, "simple-block", draftID)
	if len(qs) != 2 {
		t.Fatalf("want 2 staged questions, got %d (%+v)", len(qs), qs)
	}
	q1, q2 := qs[0].ID, qs[1].ID

	// Resolve the first one.
	if rec := postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/question/"+q1+"/toggle", nil); rec.Code != http.StatusOK {
		t.Fatalf("toggle: %d %s", rec.Code, rec.Body.String())
	}
	// Delete the future goal.
	if rec := postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/question/"+q2+"/delete", nil); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}

	qs = draftQuestions(t, app, cookie, "simple-block", draftID)
	if len(qs) != 1 {
		t.Fatalf("want 1 question after delete, got %d (%+v)", len(qs), qs)
	}
	if !qs[0].Resolved {
		t.Fatal("expected remaining question to be resolved")
	}
	if qs[0].Text != "do we support refunds?" {
		t.Fatalf("Text = %q", qs[0].Text)
	}

	// The staging checklist survives a process restart.
	loaded := loadDraftFromStore(t, app, "simple-block", draftID)
	if len(loaded.Questions) != 1 || !loaded.Questions[0].Resolved || loaded.Questions[0].FutureGoal {
		t.Fatalf("disk persistence mismatch: %+v", loaded.Questions)
	}

	// A brand-new version carries the open questions forward.
	rec = postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/new-version", mustJSON(t, map[string]string{"versionName": "with-refund"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("new-version: %d %s", rec.Code, rec.Body.String())
	}
	s := sessionFor(t, app, cookie)
	fs := s.Flows["simple-block"]
	child := fs.Drafts[fs.ActiveDraftID]
	if len(child.Questions) != 1 || child.Questions[0].Text != "do we support refunds?" {
		t.Fatalf("forked version should carry staged questions forward: %+v", child.Questions)
	}
}

func TestAddQuestionRejectsEmptyText(t *testing.T) {
	app := testAppWithRepo(t, repoRoot(t))
	cookie, draftID := setupSessionAndDraft(t, app, "simple-block")

	rec := postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/question",
		mustJSON(t, map[string]string{"qText": "   ", "qFuture": "open"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty question: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if len(draftQuestions(t, app, cookie, "simple-block", draftID)) != 0 {
		t.Fatal("no question should be persisted for empty text")
	}
}

func TestSetSourceValidatesAndRenders(t *testing.T) {
	app := testAppWithRepo(t, repoRoot(t))
	cookie, draftID := setupSessionAndDraft(t, app, "simple-block")

	valid := mustJSON(t, map[string]string{"source": "eventmodeling\ntf 01 evt Start\n"})
	if rec := postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/source", valid); rec.Code != http.StatusOK {
		t.Fatalf("set valid source: %d %s", rec.Code, rec.Body.String())
	}
	s := sessionFor(t, app, cookie)
	loaded := s.Flows["simple-block"].Drafts[draftID]
	if loaded.ParseError != "" {
		t.Fatalf("expected no parse error for valid source, got %q", loaded.ParseError)
	}
	if loaded.SVG == "" {
		t.Fatal("expected a rendered SVG after a valid source edit")
	}

	invalid := mustJSON(t, map[string]string{"source": "eventmodeling\ntf 01 evt Start { \"a\": { }"})
	if rec := postJSON(t, app, cookie, "/flow/simple-block/draft/"+draftID+"/source", invalid); rec.Code != http.StatusOK {
		t.Fatalf("set invalid source: %d %s", rec.Code, rec.Body.String())
	}
	loaded = sessionFor(t, app, cookie).Flows["simple-block"].Drafts[draftID]
	if loaded.ParseError == "" {
		t.Fatal("expected a parse/validation error for an invalid source")
	}
	if loaded.SVG != "" {
		t.Fatal("expected no SVG while the source is invalid")
	}
}

func TestWorkspaceRendersStagingTabs(t *testing.T) {
	app := testApp(t)
	page := WorkspacePage{
		ActiveFlow:    "flow",
		ActiveDraftID: "flow-2026-08-13-v1",
		ActiveSVG:     template.HTML(""),
		PatchSVG:      true,
		Questions: []QuestionView{
			{ID: "q1", Text: "do we support refunds?", FutureGoal: false},
		},
		Source:      "eventmodeling\ntf 01 evt Start\n",
		ParseError:  "something broke",
		Drafts:      []DraftTab{{ID: "flow-2026-08-13-v1", Label: "happy path"}},
	}
	b, err := app.renderTemplateToBytes("workspace", page)
	if err != nil {
		t.Fatalf("render workspace: %v", err)
	}
	html := string(b)
	for _, want := range []string{
		"$chatTab == 'discussion'",
		"$chatTab == 'source'",
		"$chatTab == 'open-questions'",
		"data-bind:source",
		"data-bind:qText",
		"data-bind:qFuture",
		"data-bind:versionName",
		"do we support refunds?",
		"something broke",
		"/flow/flow/draft/flow-2026-08-13-v1/question/q1/toggle",
		"/flow/flow/draft/flow-2026-08-13-v1/source",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("workspace template missing %q:\n%s", want, html)
		}
	}
	if !strings.Contains(html, "data-attr:class") {
		t.Fatalf("workspace template should use data-attr:class for tab styling:\n%s", html)
	}
	for _, bad := range []string{"data-bind-source", "data-show-"} {
		if strings.Contains(html, bad) {
			t.Fatalf("workspace template uses deprecated hyphen attr %q", bad)
		}
	}
	if strings.Contains(html, "<svg") {
		t.Fatalf("workspace patch fragment should not contain inline svg:\n%s", html)
	}
}

func TestExamplesGalleryRenders(t *testing.T) {
	app := testAppWithRepo(t, repoRoot(t))
	req := httptest.NewRequest(http.MethodGet, "/examples", nil)
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /examples: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Examples gallery",
		"Core Payments",
		"With Fraud Screening",
		"With Settlement and Reconciliation",
		"With Async Payout-Failure Recovery",
		"/?flow=fintech-payments-v4-async-fails",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("examples page missing %q", want)
		}
	}
	if strings.Count(body, "<svg") != len(fintechExamples) {
		t.Fatalf("expected %d rendered example SVGs, got %d", len(fintechExamples), strings.Count(body, "<svg"))
	}
}

func TestIndexDeepLinksFlow(t *testing.T) {
	app := testAppWithRepo(t, repoRoot(t))
	req := httptest.NewRequest(http.MethodGet, "/?flow=simple-block", nil)
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("index: %d %s", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()[0]
	gr := httptest.NewRequest(http.MethodGet, "/", nil)
	gr.AddCookie(cookie)
	grec := httptest.NewRecorder()
	s := app.sessions.ForRequest(grec, gr)
	if s.ActiveFlow != "simple-block" {
		t.Fatalf("ActiveFlow = %q, want simple-block", s.ActiveFlow)
	}
	if s.Flows["simple-block"].ActiveDraftID == "" {
		t.Fatal("expected a v1 draft for the deep-linked flow")
	}
}
