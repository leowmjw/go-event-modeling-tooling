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
	newDraft, err := a.sessions.NewDraft(fs, source, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: new version created", "flow", flow, "source_draft", sourceID, "new_draft", newDraft.ID)
	a.patchWorkspace(w, r, s)
}

// lookupDraft resolves the flow + draft path values, or writes the HTTP
// error and returns nils.
func (a *App) lookupDraft(w http.ResponseWriter, r *http.Request) (*Session, *FlowState, *DraftVersion, bool) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return nil, nil, nil, false
	}
	d, ok := fs.Drafts[draftID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return nil, nil, nil, false
	}
	return s, fs, d, true
}

// handleAcceptProposal promotes the draft's staged assistant proposal to
// be its content.
func (a *App) handleAcceptProposal(w http.ResponseWriter, r *http.Request) {
	s, _, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	d.mu.Lock()
	if d.PendingEvml == "" {
		d.mu.Unlock()
		http.Error(w, "no proposal to accept", http.StatusConflict)
		return
	}
	d.EvmlSource = d.PendingEvml
	d.SVG = d.PendingSVG
	d.PendingEvml, d.PendingSVG = "", ""
	d.ParseError = ""
	d.UpdatedAt = time.Now()
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: "Proposal accepted — it is now this version's model.", At: time.Now()})
	d.mu.Unlock()

	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving draft failed", "draft_id", d.ID, "error", err)
	}
	a.sessionLog(s).Info("action: proposal accepted", "draft_id", d.ID)
	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}

// handleRejectProposal discards the draft's staged assistant proposal.
func (a *App) handleRejectProposal(w http.ResponseWriter, r *http.Request) {
	s, _, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	d.mu.Lock()
	d.PendingEvml, d.PendingSVG = "", ""
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: "Proposal rejected — the diagram is back to this version's model.", At: time.Now()})
	d.mu.Unlock()

	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving draft failed", "draft_id", d.ID, "error", err)
	}
	a.sessionLog(s).Info("action: proposal rejected", "draft_id", d.ID)
	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}

// handleSaveSource applies the Source tab's hand-edited .evml to the
// draft after validating it. Invalid source is kept out of the diagram
// and reported as the draft's parse error.
func (a *App) handleSaveSource(w http.ResponseWriter, r *http.Request) {
	s, _, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	var signals struct {
		EvmlSource string `json:"evmlSource"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	svg, err := renderEvml(signals.EvmlSource)

	d.mu.Lock()
	if err != nil {
		d.ParseError = err.Error()
		d.mu.Unlock()
		a.sessionLog(s).Info("action: source edit failed validation", "draft_id", d.ID, "error", err)
		sse := datastar.NewSSE(w, r)
		a.patchWorkspaceSSE(sse, s)
		return
	}
	d.EvmlSource = signals.EvmlSource
	d.SVG = svg
	d.PendingEvml, d.PendingSVG = "", ""
	d.ParseError = ""
	d.UpdatedAt = time.Now()
	d.mu.Unlock()

	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving draft failed", "draft_id", d.ID, "error", err)
	}
	a.sessionLog(s).Info("action: source edit applied", "draft_id", d.ID, "evml_len", len(signals.EvmlSource))
	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}

// handleRenameDraft sets the draft's expert-given label (empty clears it,
// falling back to "vN").
func (a *App) handleRenameDraft(w http.ResponseWriter, r *http.Request) {
	s, _, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	var signals struct {
		DraftLabel string `json:"draftLabel"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d.Label = signals.DraftLabel
	d.UpdatedAt = time.Now()
	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving draft failed", "draft_id", d.ID, "error", err)
	}
	a.sessionLog(s).Info("action: draft renamed", "draft_id", d.ID, "label", d.Label)
	a.patchWorkspace(w, r, s)
}

// handleDeleteDraft removes a draft version from disk and the session,
// then switches to the newest remaining draft (or creates a fresh one
// from the baseline when none remain).
func (a *App) handleDeleteDraft(w http.ResponseWriter, r *http.Request) {
	s, fs, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	if err := a.store.Delete(fs.Name, d.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.sessionLog(s).Info("action: draft deleted", "flow", fs.Name, "draft_id", d.ID)

	s.mu.Lock()
	delete(fs.Drafts, d.ID)
	for i, id := range fs.DraftOrder {
		if id == d.ID {
			fs.DraftOrder = append(fs.DraftOrder[:i], fs.DraftOrder[i+1:]...)
			break
		}
	}
	if len(fs.DraftOrder) > 0 {
		fs.ActiveDraftID = fs.DraftOrder[len(fs.DraftOrder)-1]
	}
	s.mu.Unlock()

	if len(fs.DraftOrder) == 0 {
		if _, err := a.sessions.NewDraft(fs, nil, time.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	a.sessions.PersistSelection(s)
	a.patchWorkspace(w, r, s)
}

// handleExportEvml downloads the draft's .evml source as a file.
func (a *App) handleExportEvml(w http.ResponseWriter, r *http.Request) {
	_, _, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", d.ID+".evml"))
	_, _ = w.Write([]byte(d.EvmlSource))
}

// handleExportSVG downloads the draft's rendered diagram as an .svg file.
func (a *App) handleExportSVG(w http.ResponseWriter, r *http.Request) {
	_, _, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	if d.SVG == "" {
		http.Error(w, "draft has no rendered diagram yet", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", d.ID+".svg"))
	_, _ = w.Write([]byte(d.SVG))
}

// handleActivate drops draftID's date/version suffix and writes it into
// testdata/fixtures/<flow>.evml, promoting it to the flow's new baseline.
// The previous baseline is backed up under <state>/_backups/<flow>/ first.
func (a *App) handleActivate(w http.ResponseWriter, r *http.Request) {
	s, fs, d, ok := a.lookupDraft(w, r)
	if !ok {
		return
	}
	flow := fs.Name
	if d.EvmlSource == "" || d.ParseError != "" {
		http.Error(w, "draft has no valid .evml to activate yet", http.StatusConflict)
		return
	}

	if err := os.MkdirAll(a.fixturesDir(), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dest := filepath.Join(a.fixturesDir(), flow+".evml")

	// Back up the current baseline before overwriting it.
	if prev, err := os.ReadFile(dest); err == nil && len(prev) > 0 {
		backupDir := filepath.Join(a.cfg.StateDir, "_backups", flow)
		if err := os.MkdirAll(backupDir, 0o755); err == nil {
			backupPath := filepath.Join(backupDir, time.Now().Format("20060102-150405")+".evml")
			if err := os.WriteFile(backupPath, prev, 0o644); err != nil {
				a.sessionLog(s).Warn("baseline backup failed", "path", backupPath, "error", err)
			} else {
				a.sessionLog(s).Info("baseline backed up", "path", backupPath)
			}
		}
	}

	if err := os.WriteFile(dest, []byte(d.EvmlSource), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.sessionLog(s).Info("action: draft activated", "flow", flow, "draft_id", d.ID, "path", dest)

	fs.BaselineEvml = d.EvmlSource
	fs.BaselineSVG = d.SVG
	fs.IsNew = false

	note := fmt.Sprintf("Activated as %s — this is now the flow's baseline.", flow+".evml")
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: note, At: time.Now()})
	_ = a.store.Save(d)

	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}
