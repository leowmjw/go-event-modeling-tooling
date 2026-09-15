package webapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// editClient drives App.Routes() in-process with one session cookie, the
// way a browser would, so the whole no-DSL editing flow is covered without
// opening a port.
type editClient struct {
	t      *testing.T
	h      http.Handler
	cookie *http.Cookie
}

func newEditClient(t *testing.T) (*editClient, *App) {
	t.Helper()
	app := testAppWithRepo(t, filepath.Join("..", "..", "..", ".."))
	c := &editClient{t: t, h: app.Routes()}
	rec := c.do(http.MethodGet, "/", nil)
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == sessionCookieName {
			c.cookie = ck
		}
	}
	if c.cookie == nil {
		t.Fatal("no session cookie issued")
	}
	return c, app
}

func (c *editClient) do(method, path string, signals map[string]any) *httptest.ResponseRecorder {
	c.t.Helper()
	var body *strings.Reader
	if signals != nil {
		b, _ := json.Marshal(signals)
		body = strings.NewReader(string(b))
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	return rec
}

func (c *editClient) post(path string, signals map[string]any) string {
	c.t.Helper()
	rec := c.do(http.MethodPost, path, signals)
	if rec.Code != http.StatusOK {
		c.t.Fatalf("POST %s: status %d, body %s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			snippet := body
			if len(snippet) > 600 {
				snippet = snippet[:600]
			}
			if dir := os.Getenv("EVML_TEST_DUMP"); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, "failed-body.txt"), []byte(body), 0o644)
			}
			t.Fatalf("response missing %q\n--- first 600 bytes ---\n%s", w, snippet)
		}
	}
}

func TestWorkshopEditingFlow(t *testing.T) {
	c, app := newEditClient(t)

	body := c.post("/flow/select", map[string]any{"model": "", "flow": "staging-lens-hotspots", "newFlowName": ""})
	mustContain(t, body, "panel-tabs", "Velocity check", `id="svg-container"`, "<svg")

	// Find the draft id the server created.
	s := app.sessions.sessions[c.cookie.Value]
	fs := s.Flows["staging-lens-hotspots"]
	draft := fs.ActiveDraftID
	if draft == "" {
		t.Fatal("no active draft")
	}
	base := "/flow/staging-lens-hotspots/draft/" + draft

	t.Run("lens hides staging frames", func(t *testing.T) {
		body := c.post("/lens", map[string]any{"lens": "current"})
		defer c.post("/lens", map[string]any{"lens": "all"})
		if strings.Contains(body, `class="box box-pcr box-stage-staging"`) {
			t.Fatal("as-is lens still shows staging frames")
		}
		mustContain(t, body, "hidden-by-lens", `class="box box-ui box-stage-current"`)
	})

	t.Run("rejected step leaves the draft untouched", func(t *testing.T) {
		before := fs.Drafts[draft].EvmlSource
		body := c.post(base+"/step", map[string]any{"stepType": "widget", "stepName": "x"})
		mustContain(t, body, "That change was not applied", "unknown step type")
		if fs.Drafts[draft].EvmlSource != before {
			t.Fatal("source changed after rejected edit")
		}
	})

	t.Run("add a staging step after 09", func(t *testing.T) {
		body := c.post(base+"/step", map[string]any{
			"stepType": "cmd", "stepName": "review flagged top up", "stepActor": "risk analyst",
			"stepAfter": "09", "stepStage": "staging", "stepPayload": `walletId: "w-1"`,
		})
		mustContain(t, body, "Added step 13", `"sel":"13"`, "ReviewFlaggedTopUp")
		src := fs.Drafts[draft].EvmlSource
		mustContain(t, src, "tf 13 cmd ReviewFlaggedTopUp @RiskAnalyst { walletId: \"w-1\" } #staging")
		// Inserted right after frame 09, before frame 10.
		if strings.Index(src, "tf 13 ") > strings.Index(src, "tf 10 ") || strings.Index(src, "tf 13 ") < strings.Index(src, "tf 09 ") {
			t.Fatalf("step 13 not inserted after 09:\n%s", src)
		}
	})

	t.Run("resolve a question into a note", func(t *testing.T) {
		body := c.post(base+"/hotspot/0/resolve", map[string]any{"resolveAction": "note", "resolveText": "Threshold scales with KYC tier."})
		mustContain(t, body, "Resolved the question on step 06")
		src := fs.Drafts[draft].EvmlSource
		mustContain(t, src, "note 06 {", "Decision: Threshold scales with KYC tier.")
		if strings.Count(src, "hotspot ") != 1 {
			t.Fatalf("expected one hotspot left, got:\n%s", src)
		}
	})

	t.Run("park a new question", func(t *testing.T) {
		body := c.post(base+"/hotspot", map[string]any{"hotFrame": "13", "hotText": "Does review block the wallet meanwhile?"})
		mustContain(t, body, "Opened a question on step 13")
		mustContain(t, fs.Drafts[draft].EvmlSource, "hotspot 13 {")
	})

	t.Run("add a scenario", func(t *testing.T) {
		body := c.post(base+"/scenario", map[string]any{
			"scnFrame": "13", "scnLabel": "analyst clears flag",
			"scnGiven": `event UnusualTopUpFlagged { walletId: "w-1" }`,
			"scnWhen":  `command ReviewFlaggedTopUp { decision: "clear" }`,
			"scnThen":  `event FlaggedTopUpCleared { walletId: "w-1" }`,
		})
		mustContain(t, body, "Added scenario")
		mustContain(t, fs.Drafts[draft].EvmlSource, `gwt 13 "analyst clears flag"`, "    cmd ReviewFlaggedTopUp { decision: \"clear\" }")
	})

	t.Run("promote step to current", func(t *testing.T) {
		body := c.post(base+"/step/13/stage", map[string]any{"stage": "current"})
		mustContain(t, body, "is now current")
		if strings.Contains(fs.Drafts[draft].EvmlSource, "#staging") && strings.Contains(fs.Drafts[draft].EvmlSource, "tf 13 cmd ReviewFlaggedTopUp @RiskAnalyst { walletId: \"w-1\" } #staging") {
			t.Fatal("step 13 still tagged staging")
		}
	})

	t.Run("add a slice", func(t *testing.T) {
		body := c.post(base+"/slice", map[string]any{"sliceKind": "slice", "sliceName": "Analyst review", "sliceStart": "13", "sliceEnd": "13", "sliceStatus": "Planned", "sliceStage": "staging"})
		mustContain(t, body, `Added slice`, `Analyst review`, `13-13 status Planned stage staging`)
	})

	t.Run("compare panel lists the diff and lint", func(t *testing.T) {
		rec := c.do(http.MethodGet, base, nil)
		body := rec.Body.String()
		mustContain(t, body, "diff-added", "13 cmd ReviewFlaggedTopUp", "question(s) resolved", "slice Analyst review", "[hotspot] frame 13", `class="session-log"`, "Added step 13")
	})

	t.Run("export returns the draft source", func(t *testing.T) {
		rec := c.do(http.MethodGet, base+"/export.evml", nil)
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "eventmodeling") {
			t.Fatalf("export: %d %s", rec.Code, rec.Body.String()[:40])
		}
		rec = c.do(http.MethodGet, base+"/export.svg", nil)
		mustContain(t, rec.Body.String(), "<svg", `data-frame="13"`)
	})

	t.Run("broken source is rejected with a line number", func(t *testing.T) {
		before := fs.Drafts[draft].EvmlSource
		body := c.post(base+"/source", map[string]any{"source": "eventmodeling\ntf 01 ui X\ntf 02 evt Y ->> 01\n"})
		mustContain(t, body, "That change was not applied", "event can only receive input from a command")
		if fs.Drafts[draft].EvmlSource != before {
			t.Fatal("broken source was applied")
		}
	})

	t.Run("good source is applied with a diff note", func(t *testing.T) {
		src := fs.Drafts[draft].EvmlSource + "\ntf 14 rmo ReviewQueue { pending: 1 } #staging\n"
		body := c.post(base+"/source", map[string]any{"source": src})
		mustContain(t, body, "Edited the model text: + 14 rmo ReviewQueue")
	})

	t.Run("remove step drops anchored blocks", func(t *testing.T) {
		body := c.post(base+"/step/13/remove", nil)
		mustContain(t, body, "Removed step 13")
		src := fs.Drafts[draft].EvmlSource
		for _, gone := range []string{"tf 13 ", "gwt 13 ", "hotspot 13 ", "Analyst review", "13-13"} {
			if strings.Contains(src, gone) {
				t.Fatalf("%q still present after removal:\n%s", gone, src)
			}
		}
	})

	t.Run("persisted session keeps the lens", func(t *testing.T) {
		c.post("/lens", map[string]any{"lens": "staging"})
		snap, found, err := app.store.LoadSessionByToken(c.cookie.Value)
		if err != nil || !found || snap.Lens != "staging" {
			t.Fatalf("snapshot lens = %q (found=%v, err=%v)", snap.Lens, found, err)
		}
	})
}

func TestNewFlowStartsEmptyAndAcceptsFirstStep(t *testing.T) {
	c, app := newEditClient(t)
	body := c.post("/flow/select", map[string]any{"model": "", "flow": "__new__", "newFlowName": "Merchant Onboarding"})
	mustContain(t, body, "no saved baseline yet")
	s := app.sessions.sessions[c.cookie.Value]
	fs := s.Flows["merchant-onboarding"]
	base := "/flow/merchant-onboarding/draft/" + fs.ActiveDraftID
	body = c.post(base+"/step", map[string]any{"stepType": "screen", "stepName": "Merchant application form", "stepActor": "Merchant", "stepStage": "current"})
	mustContain(t, body, "Added step 01 (tf 01 ui MerchantApplicationForm @Merchant)")
	body = c.post(base+"/step", map[string]any{"stepType": "command", "stepName": "Submit application", "stepAfter": "01"})
	mustContain(t, body, "Added step 02", "#staging")
}
