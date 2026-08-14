package webapp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleSelectFlowPersistsModelAndFlow(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..", "..")
	app := testAppWithRepo(t, repoRoot)

	// Establish session cookie.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	app.sessions.ForRequest(rec, req)
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	token := cookies[0].Value

	body, _ := json.Marshal(map[string]string{
		"model": "Qwen3-0.6B-Q8_0",
		"flow":  "simple-block",
	})
	selectReq := httptest.NewRequest(http.MethodPost, "/flow/select", bytes.NewReader(body))
	selectReq.Header.Set("Content-Type", "application/json")
	selectReq.AddCookie(cookies[0])
	selectRec := httptest.NewRecorder()
	app.handleSelectFlow(selectRec, selectReq)
	if selectRec.Code != http.StatusOK {
		t.Fatalf("handleSelectFlow status = %d, body = %s", selectRec.Code, selectRec.Body.String())
	}

	snap, found, err := app.store.LoadSessionByToken(token)
	if err != nil {
		t.Fatalf("LoadSessionByToken: %v", err)
	}
	if !found {
		t.Fatal("expected persisted session snapshot")
	}
	if snap.ModelID != "Qwen3-0.6B-Q8_0" {
		t.Fatalf("ModelID = %q, want Qwen3-0.6B-Q8_0", snap.ModelID)
	}
	if snap.ActiveFlow != "simple-block" {
		t.Fatalf("ActiveFlow = %q, want simple-block", snap.ActiveFlow)
	}
	if snap.ActiveDraftByFlow["simple-block"] == "" {
		t.Fatal("expected active draft for simple-block")
	}
}

func TestPageTemplateUsesDatastarColonSyntax(t *testing.T) {
	app := testApp(t)
	b, err := app.renderTemplateToBytes("page", WorkspacePage{
		ModelID:    "test-model",
		ActiveFlow: "simple-block",
		Models:     []ModelChoiceView{{ID: "test-model", SizeHuman: "1GiB"}},
		Fixtures:   []string{"simple-block"},
	})
	if err != nil {
		t.Fatalf("render page: %v", err)
	}
	html := string(b)
	for _, attr := range []string{
		"data-signals:model",
		"data-signals:flow",
		"data-on:change",
		"data-bind:model",
		"data-on:submit__prevent",
		"data-bind:flow",
		"data-bind:new-flow-name",
	} {
		if !strings.Contains(html, attr) {
			t.Fatalf("page template missing %s; snippet: %s", attr, html[:min(400, len(html))])
		}
	}
	for _, deprecated := range []string{
		"data-on-change",
		"data-bind-model",
		"data-on-submit",
		"data-signals-model",
	} {
		if strings.Contains(html, deprecated) {
			t.Fatalf("page template still uses deprecated %s", deprecated)
		}
	}
}

func testAppWithRepo(t *testing.T, repoRoot string) *App {
	t.Helper()
	app := testApp(t)
	app.cfg.RepoRoot = repoRoot
	app.systemPrompt = BuildSystemPrompt(repoRoot)
	return app
}
