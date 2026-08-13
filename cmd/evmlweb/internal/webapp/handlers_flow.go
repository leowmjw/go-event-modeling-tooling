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
		page.ActiveSVG = template.HTML(d.SVG)
		page.Transcript = toChatViews(d.Transcript)
		page.ParseError = d.ParseError
	} else {
		page.ActiveSVG = template.HTML(fs.BaselineSVG)
	}

	return page, nil
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	page, err := a.buildPage(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, "page", page); err != nil {
		a.log.Error("render index failed", "error", err)
	}
}

// renderWorkspaceFragment renders just the "workspace" block, for SSE
// patches after flow/draft/chat actions.
func (a *App) renderWorkspaceFragment(s *Session) (string, error) {
	page, err := a.buildPage(s)
	if err != nil {
		return "", err
	}
	b, err := a.renderTemplateToBytes("workspace", page)
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
	a.log.Info("model selected", "model_id", signals.Model)

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSelectFlow(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)

	var signals struct {
		Flow         string `json:"flow"`
		NewFlowName  string `json:"new_flow_name"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	a.patchWorkspace(w, r, s)
}

func (a *App) handleSwitchDraft(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	s.mu.Lock()
	if fs, ok := s.Flows[flow]; ok {
		if _, ok := fs.Drafts[draftID]; ok {
			fs.ActiveDraftID = draftID
			s.ActiveFlow = flow
		}
	}
	s.mu.Unlock()

	a.patchWorkspace(w, r, s)
}

// patchWorkspace renders the current workspace fragment and pushes it to
// the client over SSE.
func (a *App) patchWorkspace(w http.ResponseWriter, r *http.Request, s *Session) {
	frag, err := a.renderWorkspaceFragment(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElements(frag, datastar.WithSelectorID("workspace-inner"), datastar.WithModeOuter()); err != nil {
		a.log.Warn("patch workspace failed", "error", err)
	}
}
