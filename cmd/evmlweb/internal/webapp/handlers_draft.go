package webapp

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func (a *App) handleNewVersion(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	sourceID := r.PathValue("id")

	var signals struct {
		Label  string `json:"label"`
		Intent string `json:"intent"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		// No body / unreadable signals is fine — the plain button posts
		// nothing and gets an unnamed what-if.
		signals.Label, signals.Intent = "", ""
	}

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return
	}

	switch signals.Intent {
	case "", "exploring", "future", "ready":
	default:
		http.Error(w, "unknown intent", http.StatusBadRequest)
		return
	}

	source := fs.Drafts[sourceID] // nil is fine — NewDraft falls back to the baseline
	newDraft, err := a.sessions.NewDraft(fs, source, time.Now(), strings.TrimSpace(signals.Label), signals.Intent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: new what-if created", "flow", flow, "source_draft", sourceID, "new_draft", newDraft.ID, "label", newDraft.Label, "intent", newDraft.Intent)
	a.patchWorkspace(w, r, s)
}

// handleActivate drops draftID's date/version suffix and writes it into
// testdata/fixtures/<flow>.evml, promoting it to the flow's new baseline.
// It refuses while the draft has a parse error or unresolved wiring
// issues — activation is the "commit" step of staging.
func (a *App) handleActivate(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return
	}
	d, ok := fs.Drafts[draftID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	if d.EvmlSource == "" || d.ParseError != "" {
		http.Error(w, "draft has no valid .evml to activate yet", http.StatusConflict)
		return
	}
	if d.ValidationIssues != "" {
		http.Error(w, "resolve the wiring issues before activating this draft", http.StatusConflict)
		return
	}

	if err := os.MkdirAll(a.fixturesDir(), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dest := filepath.Join(a.fixturesDir(), flow+".evml")
	if err := os.WriteFile(dest, []byte(d.EvmlSource), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.sessionLog(s).Info("action: draft activated", "flow", flow, "draft_id", draftID, "path", dest)

	fs.BaselineEvml = d.EvmlSource
	fs.BaselineSVG = d.SVG
	fs.IsNew = false

	note := fmt.Sprintf("Activated as %s — this is now the flow's baseline.", flow+".evml")
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: note, At: time.Now()})
	_ = a.store.Save(d)

	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}
