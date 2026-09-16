package webapp

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testEvml = `eventmodeling

tf 01 ui Screen
tf 02 cmd DoThing { x: 1 }
tf 03 evt ThingDone ->> 02
`

// setupFlow wires a session with one flow and one draft (saved to disk)
// into the app, returning the session and draft.
func setupFlow(t *testing.T, app *App, evmlSource string) (*Session, *DraftVersion, *http.Cookie) {
	t.Helper()
	w := httptest.NewRecorder()
	s := app.sessions.ForRequest(w, httptest.NewRequest("GET", "/", nil))
	var cookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie issued")
	}

	d := &DraftVersion{
		ID:         "flow-2026-09-15-v1",
		FlowName:   "flow",
		Date:       "2026-09-15",
		Seq:        1,
		EvmlSource: evmlSource,
	}
	if err := app.store.Save(d); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	fs := &FlowState{
		Name:           "flow",
		BaselineEvml:   evmlSource,
		Drafts:         map[string]*DraftVersion{d.ID: d},
		DraftOrder:     []string{d.ID},
		ActiveDraftID:  d.ID,
		NextSeqForDate: map[string]int{"2026-09-15": 2},
	}
	s.mu.Lock()
	s.Flows["flow"] = fs
	s.ActiveFlow = "flow"
	s.mu.Unlock()
	return s, d, cookie
}

func TestActiveSVGPrefersPendingProposal(t *testing.T) {
	fs := &FlowState{BaselineSVG: "<base/>"}
	d := &DraftVersion{SVG: "<draft/>", PendingSVG: "<pending/>"}
	if got := activeSVG(fs, d); got != "<pending/>" {
		t.Fatalf("activeSVG = %q, want pending proposal preview", got)
	}
}

func TestAcceptRejectProposal(t *testing.T) {
	app := testApp(t)
	_, d, cookie := setupFlow(t, app, testEvml)
	d.PendingEvml = testEvml + "\n// extra\n"
	d.PendingSVG = "<pending/>"

	// Reject clears the proposal without touching the draft content.
	req := httptest.NewRequest("POST", "/flow/flow/draft/"+d.ID+"/reject", nil)
	req.SetPathValue("flow", "flow")
	req.SetPathValue("id", d.ID)
	req.AddCookie(cookie)
	app.handleRejectProposal(httptest.NewRecorder(), req)
	if d.PendingEvml != "" || d.PendingSVG != "" {
		t.Fatalf("reject should clear the proposal")
	}
	if d.EvmlSource != testEvml {
		t.Fatalf("reject should not change EvmlSource")
	}

	// Accept promotes the proposal.
	d.PendingEvml = testEvml + "\n// extra\n"
	d.PendingSVG = "<pending/>"
	req = httptest.NewRequest("POST", "/flow/flow/draft/"+d.ID+"/accept", nil)
	req.SetPathValue("flow", "flow")
	req.SetPathValue("id", d.ID)
	req.AddCookie(cookie)
	app.handleAcceptProposal(httptest.NewRecorder(), req)
	if d.EvmlSource != testEvml+"\n// extra\n" {
		t.Fatalf("accept should promote PendingEvml, got %q", d.EvmlSource)
	}
	if d.SVG != "<pending/>" || d.PendingEvml != "" {
		t.Fatalf("accept should adopt PendingSVG and clear the proposal")
	}
}

func TestSaveSourceValidates(t *testing.T) {
	app := testApp(t)
	_, d, cookie := setupFlow(t, app, testEvml)

	bad := httptest.NewRequest("POST", "/", strings.NewReader(`{"evmlSource":"not valid"}`))
	bad.SetPathValue("flow", "flow")
	bad.SetPathValue("id", d.ID)
	bad.AddCookie(cookie)
	bad.Header.Set("Content-Type", "application/json")
	bad.Header.Set("Datastar-Request", "true")
	app.handleSaveSource(httptest.NewRecorder(), bad)
	if d.ParseError == "" {
		t.Fatalf("invalid source should set ParseError")
	}
	if d.EvmlSource != testEvml {
		t.Fatalf("invalid source must not replace EvmlSource")
	}

	edited := testEvml + "tf 04 rmo Things ->> 03\n"
	good := httptest.NewRequest("POST", "/", strings.NewReader(`{"evmlSource":`+jsonQuote(edited)+`}`))
	good.SetPathValue("flow", "flow")
	good.SetPathValue("id", d.ID)
	good.AddCookie(cookie)
	good.Header.Set("Content-Type", "application/json")
	good.Header.Set("Datastar-Request", "true")
	app.handleSaveSource(httptest.NewRecorder(), good)
	if d.ParseError != "" {
		t.Fatalf("valid source set ParseError: %s", d.ParseError)
	}
	if d.EvmlSource != edited || d.SVG == "" {
		t.Fatalf("valid source should be applied and rendered")
	}
}

func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func TestRenameAndDeleteDraft(t *testing.T) {
	app := testApp(t)
	s, d, cookie := setupFlow(t, app, testEvml)

	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"draftLabel":"chargeback path"}`))
	req.SetPathValue("flow", "flow")
	req.SetPathValue("id", d.ID)
	req.AddCookie(cookie)
	req.Header.Set("Datastar-Request", "true")
	app.handleRenameDraft(httptest.NewRecorder(), req)
	if d.Label != "chargeback path" {
		t.Fatalf("rename failed, label = %q", d.Label)
	}
	if got := draftLabel(d); got != "chargeback path" {
		t.Fatalf("draftLabel = %q, want custom label", got)
	}

	req = httptest.NewRequest("POST", "/", nil)
	req.SetPathValue("flow", "flow")
	req.SetPathValue("id", d.ID)
	req.AddCookie(cookie)
	app.handleDeleteDraft(httptest.NewRecorder(), req)

	s.mu.Lock()
	fs := s.Flows["flow"]
	s.mu.Unlock()
	if _, exists := fs.Drafts[d.ID]; exists {
		t.Fatalf("deleted draft still in session")
	}
	if len(fs.DraftOrder) != 1 {
		t.Fatalf("deleting the last draft should create a fresh one, got %d", len(fs.DraftOrder))
	}
	if _, err := os.Stat(app.store.evmlPath("flow", d.ID)); !os.IsNotExist(err) {
		t.Fatalf("deleted draft file still on disk")
	}
}

func TestExportHandlers(t *testing.T) {
	app := testApp(t)
	_, d, cookie := setupFlow(t, app, testEvml)
	d.SVG = "<svg/>"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("flow", "flow")
	req.SetPathValue("id", d.ID)
	req.AddCookie(cookie)
	app.handleExportEvml(rec, req)
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, d.ID+".evml") {
		t.Fatalf("evml export Content-Disposition = %q", cd)
	}
	if rec.Body.String() != testEvml {
		t.Fatalf("evml export body mismatch")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("flow", "flow")
	req.SetPathValue("id", d.ID)
	req.AddCookie(cookie)
	app.handleExportSVG(rec, req)
	if rec.Body.String() != "<svg/>" {
		t.Fatalf("svg export body = %q", rec.Body.String())
	}
}

func TestActivateBacksUpBaseline(t *testing.T) {
	app := testApp(t)
	repoRoot := t.TempDir()
	stateDir := t.TempDir()
	app.cfg.RepoRoot = repoRoot
	app.cfg.StateDir = stateDir

	fixtures := filepath.Join(repoRoot, "testdata", "fixtures")
	if err := os.MkdirAll(fixtures, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixtures, "flow.evml"), []byte("eventmodeling\n// old baseline\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, d, cookie := setupFlow(t, app, testEvml)
	svg, err := renderEvml(testEvml)
	if err != nil {
		t.Fatalf("render test evml: %v", err)
	}
	d.SVG = svg

	req := httptest.NewRequest("POST", "/", nil)
	req.SetPathValue("flow", "flow")
	req.SetPathValue("id", d.ID)
	req.AddCookie(cookie)
	app.handleActivate(httptest.NewRecorder(), req)

	got, err := os.ReadFile(filepath.Join(fixtures, "flow.evml"))
	if err != nil || string(got) != testEvml {
		t.Fatalf("activated fixture = %q, %v", got, err)
	}
	backups, err := os.ReadDir(filepath.Join(stateDir, "_backups", "flow"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("expected one baseline backup, got %v, %v", backups, err)
	}
	b, _ := os.ReadFile(filepath.Join(stateDir, "_backups", "flow", backups[0].Name()))
	if !strings.Contains(string(b), "old baseline") {
		t.Fatalf("backup should contain previous baseline, got %q", b)
	}
}

func TestDraftStoreRoundTripsLabelAndProposal(t *testing.T) {
	store, err := NewDraftStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	d := &DraftVersion{
		ID:          "flow-2026-09-15-v1",
		FlowName:    "flow",
		Date:        "2026-09-15",
		Seq:         1,
		Label:       "fraud branch",
		EvmlSource:  testEvml,
		PendingEvml: testEvml,
	}
	if err := store.Save(d); err != nil {
		t.Fatal(err)
	}
	drafts, err := store.LoadFlow("flow")
	if err != nil || len(drafts) != 1 {
		t.Fatalf("LoadFlow: %v (%d drafts)", err, len(drafts))
	}
	got := drafts[0]
	if got.Label != "fraud branch" {
		t.Fatalf("label = %q", got.Label)
	}
	if got.PendingEvml != testEvml || got.PendingSVG == "" {
		t.Fatalf("proposal not round-tripped: pending=%q svg=%q", got.PendingEvml, got.PendingSVG)
	}

	if err := store.Delete("flow", d.ID); err != nil {
		t.Fatal(err)
	}
	drafts, err = store.LoadFlow("flow")
	if err != nil || len(drafts) != 0 {
		t.Fatalf("after delete: %v (%d drafts)", err, len(drafts))
	}
}
