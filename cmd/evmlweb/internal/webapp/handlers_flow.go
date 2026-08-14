package webapp

import (
	"bytes"
	"html/template"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

// buildPage assembles the full WorkspacePage view model from s's current
// state: model choices, fixture list, and (if a flow is active) its tabs,
// active draft's SVG, and chat transcript.
func (a *App) buildPage(s *Session) (WorkspacePage, error) {
	choices, err := ListModelChoices(a.models)
	if err != nil {
		a.log.Warn("listing models failed", "error", err)
	}
	if s.ModelID == "" {
		s.ModelID = DefaultModelID(choices)
	}

	a.resumeActiveFlow(s)

	fixtures, err := a.listFixtures()
	if err != nil {
		a.log.Warn("listing fixtures failed", "error", err)
	}

	page := WorkspacePage{
		ModelID:    s.ModelID,
		Models:     toModelViews(choices),
		Fixtures:   fixtures,
		ActiveFlow: s.ActiveFlow,
	}

	if s.ActiveFlow == "" {
		return page, nil
	}

	fs, ok := s.Flows[s.ActiveFlow]
	if !ok {
		page.ActiveFlow = ""
		return page, nil
	}

	for _, id := range fs.DraftOrder {
		d := fs.Drafts[id]
		page.Drafts = append(page.Drafts, DraftTab{ID: d.ID, Label: draftLabel(d)})
	}
	page.ActiveDraftID = fs.ActiveDraftID

	if d, ok := fs.Drafts[fs.ActiveDraftID]; ok {
		page.ActiveSVG = template.HTML(activeSVG(fs, d))
		page.Transcript = toChatViews(d.Transcript)
		page.ParseError = d.ParseError
	} else {
		page.ActiveSVG = template.HTML(fs.BaselineSVG)
	}

	return page, nil
}

// activeSVG returns the best available rendered diagram for a draft,
// falling back to the flow baseline when the draft has no cached SVG.
func activeSVG(fs *FlowState, d *DraftVersion) string {
	if d.SVG != "" {
		return d.SVG
	}
	return fs.BaselineSVG
}

// resumeActiveFlow restores s.ActiveFlow from s's own persisted selection
// (s.PendingFlow, set only when this Session was just rehydrated from disk
// under the same cookie token — see SessionStore.ForRequest). A no-op once
// s already has an active flow, or if it was never resumed from disk.
func (a *App) resumeActiveFlow(s *Session) {
	s.mu.Lock()
	alreadyActive := s.ActiveFlow != ""
	name := s.PendingFlow
	s.mu.Unlock()
	if alreadyActive || name == "" {
		return
	}

	baselineEvml, baselineSVG, err := a.readFixture(name)
	isNew := false
	if err != nil {
		// Not an existing fixture — most likely a not-yet-activated "new
		// flow" whose drafts still live only in the draft store.
		baselineEvml = "eventmodeling\n"
		isNew = true
	}

	fs, err := a.sessions.FlowFor(s, name, baselineEvml, baselineSVG, isNew)
	if err != nil {
		a.log.Warn("resuming active flow failed", "flow", name, "error", err)
		return
	}
	if len(fs.DraftOrder) == 0 {
		if _, err := a.sessions.NewDraft(fs, nil, time.Now()); err != nil {
			a.log.Warn("resuming active flow failed", "flow", name, "error", err)
			return
		}
	}

	s.mu.Lock()
	s.ActiveFlow = name
	s.mu.Unlock()
	a.log.Info("resumed active flow", "flow", name, "draft_id", fs.ActiveDraftID)
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	needsPersist := s.ModelID == ""
	page, err := a.buildPage(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if needsPersist && s.ModelID != "" {
		a.sessions.PersistSelection(s)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, "page", page); err != nil {
		a.log.Error("render index failed", "error", err)
	}
}

// renderWorkspaceFragment renders just the "workspace" block, for SSE
// patches after flow/draft/chat actions. SVG is omitted here and patched
// separately via renderSVGFragment.
func (a *App) renderWorkspaceFragment(s *Session) (string, error) {
	page, err := a.buildPage(s)
	if err != nil {
		return "", err
	}
	page.PatchSVG = true
	b, err := a.renderTemplateToBytes("workspace", page)
	return string(b), err
}

// renderSVGFragment renders only the active draft's SVG markup.
func (a *App) renderSVGFragment(s *Session) (string, error) {
	page, err := a.buildPage(s)
	if err != nil {
		return "", err
	}
	if page.ActiveFlow == "" {
		return "", nil
	}
	b, err := a.renderTemplateToBytes("svg-content", page)
	return string(b), err
}

// renderTemplateToBytes executes the named template into a byte slice.
func (a *App) renderTemplateToBytes(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := a.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) handleSelectModel(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)

	var signals struct {
		Model string `json:"model"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.ModelID = signals.Model
	s.mu.Unlock()
	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: model selected", "model_id", signals.Model)

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSelectFlow(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	log := a.sessionLog(s)

	var signals struct {
		Model       string `json:"model"`
		Flow        string `json:"flow"`
		NewFlowName string `json:"newFlowName"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Info("action: flow select requested", "model_id", signals.Model, "flow", signals.Flow)

	if signals.Model != "" {
		s.mu.Lock()
		s.ModelID = signals.Model
		s.mu.Unlock()
	}

	if signals.Flow == "" {
		http.Error(w, "select a flow before opening", http.StatusBadRequest)
		return
	}

	var name, baselineEvml, baselineSVG string
	isNew := false

	if signals.Flow == "__new__" {
		name = Slugify(signals.NewFlowName)
		if name == "" {
			http.Error(w, "a flow name is required", http.StatusBadRequest)
			return
		}
		isNew = true
		baselineEvml = "eventmodeling\n"
	} else {
		name = signals.Flow
		var err error
		baselineEvml, baselineSVG, err = a.readFixture(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	fs, err := a.sessions.FlowFor(s, name, baselineEvml, baselineSVG, isNew)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.ActiveFlow = name
	s.mu.Unlock()

	if len(fs.DraftOrder) == 0 {
		if _, err := a.sessions.NewDraft(fs, nil, time.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	a.sessions.PersistSelection(s)
	log.Info("action: flow opened", "flow", name, "model_id", s.ModelID, "is_new", isNew, "active_draft", fs.ActiveDraftID)
	a.patchWorkspace(w, r, s)
}

func (a *App) handleSwitchDraft(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	s.mu.Lock()
	switched := false
	if fs, ok := s.Flows[flow]; ok {
		if _, ok := fs.Drafts[draftID]; ok {
			fs.ActiveDraftID = draftID
			s.ActiveFlow = flow
			switched = true
		}
	}
	s.mu.Unlock()

	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: draft tab switched", "flow", flow, "draft_id", draftID, "found", switched)
	a.patchWorkspace(w, r, s)
}

// patchWorkspace renders the current workspace fragment and pushes it to
// the client over SSE.
func (a *App) patchWorkspace(w http.ResponseWriter, r *http.Request, s *Session) {
	if _, err := a.renderWorkspaceFragment(s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}

// patchWorkspaceSSE re-renders and patches the workspace shell and SVG as
// separate elements so Datastar never morphs inline <svg> inside a large
// HTML fragment.
func (a *App) patchWorkspaceSSE(sse *datastar.ServerSentEventGenerator, s *Session) {
	log := a.sessionLog(s)

	frag, err := a.renderWorkspaceFragment(s)
	if err != nil {
		log.Warn("render workspace fragment failed", "error", err)
		return
	}
	if err := sse.PatchElements(frag, datastar.WithSelectorID("workspace-inner"), datastar.WithModeReplace()); err != nil {
		log.Warn("patch workspace failed", "error", err)
		return
	}

	svgFrag, err := a.renderSVGFragment(s)
	if err != nil {
		log.Warn("render svg fragment failed", "error", err)
		return
	}
	if svgFrag == "" {
		return
	}
	if err := sse.PatchElements(svgFrag, datastar.WithSelectorID("svg-container"), datastar.WithModeInner()); err != nil {
		log.Warn("patch svg failed", "error", err)
		return
	}

	s.mu.Lock()
	flow, draftID := s.ActiveFlow, ""
	if fs, ok := s.Flows[flow]; ok {
		draftID = fs.ActiveDraftID
	}
	s.mu.Unlock()
	log.Info("action: workspace patched", "flow", flow, "draft_id", draftID, "workspace_len", len(frag), "svg_len", len(svgFrag))
}
