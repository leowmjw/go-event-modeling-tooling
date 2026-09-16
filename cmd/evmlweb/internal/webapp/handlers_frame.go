package webapp

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	evml "github.com/leowmjw/go-event-modeling-tooling"

	"github.com/starfederation/datastar-go/datastar"
)

// activeDraft resolves the flow state + draft the request's path points
// at, 404-ing when either is unknown.
func (a *App) activeDraft(w http.ResponseWriter, r *http.Request, s *Session) (*FlowState, *DraftVersion, bool) {
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return nil, nil, false
	}
	d, ok := fs.Drafts[draftID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return nil, nil, false
	}
	return fs, d, true
}

// mutateDraft runs one source edit against the active draft through the
// full pipeline: line surgery -> re-parse (blocking on parse errors) ->
// render -> persist -> patch. The edit function returns a short
// transcript note describing what changed (empty string for none) so the
// LLM stays aware of manual edits. Failures keep the previous good
// source and surface in the frame panel instead.
func (a *App) mutateDraft(w http.ResponseWriter, r *http.Request, s *Session, fs *FlowState, d *DraftVersion, edit func(e *SourceEditor) (string, error)) {
	e, err := NewSourceEditor(d.EvmlSource)
	if err != nil {
		fs.PanelError = "The draft's source doesn't parse, so it can't be hand-edited yet — fix the error in the chat first."
		a.patchWorkspace(w, r, s)
		return
	}
	note, err := edit(e)
	if err != nil {
		fs.PanelError = err.Error()
		a.patchWorkspace(w, r, s)
		return
	}

	next := e.Source()
	svg, parseErr, issues := evaluateEvml(next)
	if parseErr != "" {
		fs.PanelError = "That edit would break the model: " + parseErr
		a.patchWorkspace(w, r, s)
		return
	}

	d.EvmlSource = next
	d.SVG = svg
	d.ParseError = ""
	d.ValidationIssues = ""
	if len(issues) > 0 {
		d.ValidationIssues = ValidationErrorsText(issues)
	}
	d.UpdatedAt = time.Now()

	if note != "" {
		d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: note, At: time.Now()})
	}
	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving draft after edit failed", "draft_id", d.ID, "error", err)
	}
	a.sessionLog(s).Info("action: draft edited", "flow", fs.Name, "draft_id", d.ID, "edit", note)
	fs.PanelError = ""
	a.patchWorkspace(w, r, s)
}

func (a *App) handleFrameSelect(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Frame string `json:"frame"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if signals.Frame == "" {
		http.Error(w, "no frame selected", http.StatusBadRequest)
		return
	}
	fs.SelectedFrame = signals.Frame
	fs.PanelError = ""
	a.sessionLog(s).Info("action: frame selected", "flow", fs.Name, "draft_id", d.ID, "frame", signals.Frame)
	a.patchWorkspace(w, r, s)
}

func (a *App) handleFrameDeselect(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, _, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	fs.SelectedFrame = ""
	fs.PanelError = ""
	a.patchWorkspace(w, r, s)
}

func (a *App) handleFrameUpdate(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Name    string `json:"name"`
		Payload string `json:"payload"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := fs.SelectedFrame
	if id == "" {
		http.Error(w, "no frame selected", http.StatusBadRequest)
		return
	}

	a.mutateDraft(w, r, s, fs, d, func(e *SourceEditor) (string, error) {
		f := e.Frame(id)
		if f == nil {
			return "", fmt.Errorf("no frame %s", id)
		}
		note := ""
		if signals.Name != "" && signals.Name != f.Identifier {
			if err := e.RenameFrame(id, signals.Name); err != nil {
				return "", err
			}
			note = fmt.Sprintf("Renamed frame %s to %s.", id, signals.Name)
		}
		if strings.TrimSpace(signals.Payload) != f.DisplayData() {
			if err := e.SetFramePayload(id, signals.Payload); err != nil {
				return "", err
			}
			if note == "" {
				note = fmt.Sprintf("Updated frame %s (%s).", id, f.Identifier)
			}
		}
		return note, nil
	})
}

func (a *App) handleFrameAdd(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		After string `json:"after"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entityType := evml.EntityType(signals.Type)
	switch entityType {
	case evml.EntityUI, evml.EntityCommand, evml.EntityEvent, evml.EntityReadModel, evml.EntityProcessor:
	default:
		http.Error(w, "unknown step type", http.StatusBadRequest)
		return
	}

	a.mutateDraft(w, r, s, fs, d, func(e *SourceEditor) (string, error) {
		newID, err := e.AddFrame(signals.After, entityType, signals.Name, "")
		if err != nil {
			return "", err
		}
		fs.SelectedFrame = newID
		a.sessionLog(s).Info("action: frame added", "flow", fs.Name, "draft_id", d.ID, "frame", newID, "type", signals.Type, "after", signals.After)
		return fmt.Sprintf("Added frame %s (%s %s).", newID, entityType, signals.Name), nil
	})
}

func (a *App) handleFrameSources(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Sources string `json:"sources"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := fs.SelectedFrame
	if id == "" {
		http.Error(w, "no frame selected", http.StatusBadRequest)
		return
	}

	var ids []string
	for _, part := range strings.FieldsFunc(signals.Sources, func(r rune) bool { return r == ' ' || r == ',' || r == '\n' || r == '\t' }) {
		ids = append(ids, part)
	}
	a.mutateDraft(w, r, s, fs, d, func(e *SourceEditor) (string, error) {
		if err := e.SetFrameSources(id, ids); err != nil {
			return "", err
		}
		if len(ids) == 0 {
			return fmt.Sprintf("Removed all input wiring from frame %s.", id), nil
		}
		return fmt.Sprintf("Rewired frame %s to receive from %s.", id, strings.Join(ids, " ")), nil
	})
}

func (a *App) handleFrameDelete(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	id := fs.SelectedFrame
	if id == "" {
		http.Error(w, "no frame selected", http.StatusBadRequest)
		return
	}

	a.mutateDraft(w, r, s, fs, d, func(e *SourceEditor) (string, error) {
		if err := e.DeleteFrame(id); err != nil {
			return "", err
		}
		fs.SelectedFrame = ""
		return fmt.Sprintf("Deleted frame %s.", id), nil
	})
}

func (a *App) handleFrameMove(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Dir string `json:"dir"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := fs.SelectedFrame
	if id == "" {
		http.Error(w, "no frame selected", http.StatusBadRequest)
		return
	}
	dir := 1
	if signals.Dir == "earlier" {
		dir = -1
	}
	a.mutateDraft(w, r, s, fs, d, func(e *SourceEditor) (string, error) {
		if err := e.MoveFrame(id, dir); err != nil {
			return "", err
		}
		return fmt.Sprintf("Moved frame %s %s in the timeline.", id, map[int]string{-1: "earlier", 1: "later"}[dir]), nil
	})
}

func (a *App) handleCompare(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, _, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Mode string `json:"mode"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch signals.Mode {
	case "", "baseline", "prev":
		fs.CompareMode = signals.Mode
	default:
		http.Error(w, "unknown compare mode", http.StatusBadRequest)
		return
	}
	a.sessionLog(s).Info("action: compare mode set", "flow", fs.Name, "mode", fs.CompareMode)
	a.patchWorkspace(w, r, s)
}

func (a *App) handleDraftMeta(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.activeDraft(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Label  string `json:"label"`
		Intent string `json:"intent"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch signals.Intent {
	case "", "exploring", "future", "ready":
	default:
		http.Error(w, "unknown intent", http.StatusBadRequest)
		return
	}
	d.Label = strings.TrimSpace(signals.Label)
	d.Intent = signals.Intent
	d.UpdatedAt = time.Now()
	if err := a.store.Save(d); err != nil {
		a.sessionLog(s).Warn("saving draft meta failed", "draft_id", d.ID, "error", err)
	}
	a.sessionLog(s).Info("action: draft meta saved", "flow", fs.Name, "draft_id", d.ID, "label", d.Label, "intent", d.Intent)
	a.patchWorkspace(w, r, s)
}

// buildFramePanel assembles the detail-panel view for the flow's selected
// frame, or nil when nothing is selected.
func buildFramePanel(fs *FlowState, d *DraftVersion) *FramePanelView {
	if fs.SelectedFrame == "" {
		return nil
	}
	e, err := NewSourceEditor(d.EvmlSource)
	if err != nil {
		return &FramePanelView{
			FlowName:  fs.Name,
			DraftID:   d.ID,
			ID:        fs.SelectedFrame,
			EditError: "The draft's source doesn't parse, so it can't be hand-edited yet — fix the error in the chat first.",
		}
	}
	f := e.Frame(fs.SelectedFrame)
	if f == nil {
		return nil
	}

	view := FramePanelView{
		FlowName:    fs.Name,
		DraftID:     d.ID,
		ID:          f.ID,
		Kind:        string(f.Kind),
		EntityType:  string(f.EntityType),
		Identifier:  f.Identifier,
		Namespace:   f.Namespace(),
		SourceIDs:   f.SourceIDs,
		DataRefName: f.DataRefName,
		Payload:     f.DisplayData(),
		EditError:   fs.PanelError,
	}
	for _, gwt := range e.model.GWTs {
		if gwt.SourceID != f.ID {
			continue
		}
		label := "Scenario"
		if gwt.Label != "" {
			label = evml.StripQuotes(gwt.Label)
		}
		view.GWTLabels = append(view.GWTLabels, label)
	}
	for _, other := range e.model.Frames {
		if other.ID == f.ID {
			continue
		}
		connected := false
		for _, src := range f.SourceIDs {
			if src == other.ID {
				connected = true
				break
			}
		}
		view.Choices = append(view.Choices, FrameChoice{
			ID:          other.ID,
			Identifier:  other.Identifier,
			EntityType:  string(other.EntityType),
			IsConnected: connected,
		})
	}
	return &view
}

// ensureDraft guarantees a flow has at least one draft tab: committed seed
// drafts are copied in the first time a seeded flow is opened (never
// overwriting existing drafts), otherwise a fresh v1 is forked from the
// baseline.
func (a *App) ensureDraft(fs *FlowState) error {
	if len(fs.DraftOrder) > 0 {
		return nil
	}
	if a.seedStore != nil {
		seeded, err := a.seedFlow(fs)
		if err != nil {
			return err
		}
		if seeded {
			return nil
		}
	}
	_, err := a.sessions.NewDraft(fs, nil, time.Now(), "", "")
	return err
}

// seedFlow copies this flow's committed example drafts (if any) from the
// seed store into the working draft store and registers them on fs.
func (a *App) seedFlow(fs *FlowState) (bool, error) {
	seeds, err := a.seedStore.LoadFlow(fs.Name)
	if err != nil {
		return false, err
	}
	if len(seeds) == 0 {
		return false, nil
	}
	for _, sd := range seeds {
		sd.FlowName = fs.Name
		if err := a.store.Save(sd); err != nil {
			return false, err
		}
		fs.Drafts[sd.ID] = sd
		fs.DraftOrder = append(fs.DraftOrder, sd.ID)
		if sd.Seq >= fs.NextSeqForDate[sd.Date] {
			fs.NextSeqForDate[sd.Date] = sd.Seq + 1
		}
	}
	fs.ActiveDraftID = fs.DraftOrder[len(fs.DraftOrder)-1]
	a.log.Info("seeded example drafts", "flow", fs.Name, "count", len(seeds))
	return true, nil
}
