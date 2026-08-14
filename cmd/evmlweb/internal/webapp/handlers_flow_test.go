package webapp

import (
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActiveSVGFallsBackToBaseline(t *testing.T) {
	fs := &FlowState{BaselineSVG: "<svg-baseline/>"}
	d := &DraftVersion{SVG: ""}
	if got := activeSVG(fs, d); got != "<svg-baseline/>" {
		t.Fatalf("activeSVG = %q, want baseline", got)
	}
}

func TestActiveSVGPrefersDraft(t *testing.T) {
	fs := &FlowState{BaselineSVG: "<svg-baseline/>"}
	d := &DraftVersion{SVG: "<svg-draft/>"}
	if got := activeSVG(fs, d); got != "<svg-draft/>" {
		t.Fatalf("activeSVG = %q, want draft svg", got)
	}
}

func TestWorkspacePatchFragmentOmitsInlineSVG(t *testing.T) {
	app := testApp(t)
	page := WorkspacePage{
		ActiveFlow:    "flow",
		ActiveDraftID: "flow-2026-08-13-v1",
		Drafts:        []DraftTab{{ID: "flow-2026-08-13-v1", Label: "v1"}},
		ActiveSVG:     template.HTML("<svg-draft/>"),
		PatchSVG:      true,
	}

	workspace, err := app.renderTemplateToBytes("workspace", page)
	if err != nil {
		t.Fatalf("render workspace: %v", err)
	}
	if strings.Contains(string(workspace), "<svg") {
		t.Fatalf("workspace patch fragment should not contain inline svg, got: %s", workspace)
	}
	if !strings.Contains(string(workspace), `id="svg-container"`) {
		t.Fatalf("workspace patch fragment missing svg-container placeholder")
	}

	svg, err := app.renderTemplateToBytes("svg-content", page)
	if err != nil {
		t.Fatalf("render svg-content: %v", err)
	}
	if !strings.Contains(string(svg), "<svg-draft/>") {
		t.Fatalf("svg fragment = %q, want draft svg", svg)
	}
}

func testApp(t *testing.T) *App {
	t.Helper()
	dir := "internal/webapp/templates"
	if _, err := os.Stat(dir); err != nil {
		dir = "templates"
	}
	tmpl, err := template.ParseGlob(filepath.Join(dir, "*.gohtml"))
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	store, err := NewDraftStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("NewDraftStore: %v", err)
	}
	return &App{
		log:      slog.Default(),
		tmpl:     tmpl,
		sessions: NewSessionStore(store, slog.Default()),
		store:    store,
	}
}
