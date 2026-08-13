package webapp

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func (a *App) handleNewVersion(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	sourceID := r.PathValue("id")

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return
	}

	source := fs.Drafts[sourceID] // nil is fine — NewDraft falls back to the baseline
	if _, err := a.sessions.NewDraft(fs, source, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.patchWorkspace(w, r, s)
}

// handleActivate drops draftID's date/version suffix and writes it into
// testdata/fixtures/<flow>.evml, promoting it to the flow's new baseline.
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

	if err := os.MkdirAll(a.fixturesDir(), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dest := filepath.Join(a.fixturesDir(), flow+".evml")
	if err := os.WriteFile(dest, []byte(d.EvmlSource), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.log.Info("draft activated", "flow", flow, "draft_id", draftID, "path", dest)

	fs.BaselineEvml = d.EvmlSource
	fs.BaselineSVG = d.SVG
	fs.IsNew = false

	note := fmt.Sprintf("Activated as %s — this is now the flow's baseline.", flow+".evml")
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: note, At: time.Now()})
	_ = a.store.Save(d)

	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}
