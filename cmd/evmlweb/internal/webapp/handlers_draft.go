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
		VersionName string `json:"versionName"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	if signals.VersionName != "" {
		newDraft.Name = signals.VersionName
	}
	if err := a.store.Save(newDraft); err != nil {
		a.sessionLog(s).Warn("saving new version metadata failed", "draft_id", newDraft.ID, "error", err)
	}

	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: new version created", "flow", flow, "source_draft", sourceID, "new_draft", newDraft.ID, "name", newDraft.Name)

	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
	// Clear the name input so the form is ready for the next version.
	_ = sse.MarshalAndPatchSignals(map[string]any{"versionName": ""})
}

// activeDraft fetches the live *DraftVersion for flow/draftID within session s
// (under the session mutex). It returns nil if the flow or draft is unknown —
// callers translate that into a 404. The returned pointer is safe to mutate
// in place; persist with a.store.Save and refresh the UI via patchWorkspace.
func (a *App) activeDraft(s *Session, flow, draftID string) *DraftVersion {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs, ok := s.Flows[flow]
	if !ok {
		return nil
	}
	return fs.Drafts[draftID]
}

// handleAddQuestion appends a staging-checklist item to the active draft.
// qFuture=="future" marks it as a future goal rather than a present-day question.
func (a *App) handleAddQuestion(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	var signals struct {
		QText   string `json:"qText"`
		QFuture string `json:"qFuture"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(signals.QText)
	if text == "" {
		http.Error(w, "question text is required", http.StatusBadRequest)
		return
	}

	d := a.activeDraft(s, flow, draftID)
	if d == nil {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	d.Questions = append(d.Questions, OpenQuestion{
		ID:         newQuestionID(),
		Text:       text,
		FutureGoal: signals.QFuture == "future",
		CreatedAt:  time.Now(),
	})
	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving question failed", "draft_id", draftID, "error", err)
	}
	a.sessionLog(s).Info("action: question added", "flow", flow, "draft_id", draftID, "future_goal", signals.QFuture == "future")

	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
	// Reset the add-question inputs so the form is ready for the next item.
	_ = sse.MarshalAndPatchSignals(map[string]any{"qText": "", "qFuture": "open"})
}

// handleToggleQuestion flips a question's resolved flag.
func (a *App) handleToggleQuestion(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow, draftID, qid := r.PathValue("flow"), r.PathValue("id"), r.PathValue("qid")

	d := a.activeDraft(s, flow, draftID)
	if d == nil {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	toggled := false
	for i := range d.Questions {
		if d.Questions[i].ID == qid {
			d.Questions[i].Resolved = !d.Questions[i].Resolved
			toggled = true
			break
		}
	}
	if !toggled {
		http.Error(w, "unknown question", http.StatusNotFound)
		return
	}
	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving question toggle failed", "draft_id", draftID, "error", err)
	}
	a.sessionLog(s).Info("action: question toggled", "flow", flow, "draft_id", draftID, "question_id", qid)
	a.patchWorkspace(w, r, s)
}

// handleDeleteQuestion removes a question from the staging checklist.
func (a *App) handleDeleteQuestion(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow, draftID, qid := r.PathValue("flow"), r.PathValue("id"), r.PathValue("qid")

	d := a.activeDraft(s, flow, draftID)
	if d == nil {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	removed := false
	filtered := d.Questions[:0]
	for _, q := range d.Questions {
		if q.ID == qid {
			removed = true
			continue
		}
		filtered = append(filtered, q)
	}
	d.Questions = filtered
	if !removed {
		http.Error(w, "unknown question", http.StatusNotFound)
		return
	}
	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving question deletion failed", "draft_id", draftID, "error", err)
	}
	a.sessionLog(s).Info("action: question deleted", "flow", flow, "draft_id", draftID, "question_id", qid)
	a.patchWorkspace(w, r, s)
}

// handleSetSource updates a draft's .evml source from the inline source
// editor, re-parses/validates and re-renders the SVG, then refreshes the
// workspace so the diagram reflects the edit immediately.
func (a *App) handleSetSource(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	var signals struct {
		Source string `json:"source"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := a.activeDraft(s, flow, draftID)
	if d == nil {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	d.EvmlSource = signals.Source
	if svg, err := renderEvml(signals.Source); err != nil {
		d.ParseError = err.Error()
		d.SVG = ""
	} else {
		d.ParseError = ""
		d.SVG = svg
	}
	d.UpdatedAt = time.Now()
	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving source edit failed", "draft_id", draftID, "error", err)
	}
	a.sessionLog(s).Info("action: source edited", "flow", flow, "draft_id", draftID, "valid", d.ParseError == "")
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
