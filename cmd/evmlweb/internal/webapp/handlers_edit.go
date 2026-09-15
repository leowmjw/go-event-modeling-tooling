package webapp

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/starfederation/datastar-go/datastar"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

// The handlers in this file are the "no DSL required" path for domain
// experts: each one turns a small form into a text edit of the active
// draft's .evml, re-parses, and either commits the new source (recording
// what happened in the draft's transcript so the session has a log) or
// rejects it with a plain-language error and leaves the draft untouched.

// draftFor resolves the flow + draft named in the request path.
func (a *App) draftFor(w http.ResponseWriter, r *http.Request, s *Session) (*FlowState, *DraftVersion, bool) {
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

// currentModel parses the draft's current source, falling back to the
// flow baseline for a brand-new draft.
func currentSource(fs *FlowState, d *DraftVersion) string {
	if strings.TrimSpace(d.EvmlSource) != "" {
		return d.EvmlSource
	}
	if strings.TrimSpace(fs.BaselineEvml) != "" {
		return fs.BaselineEvml
	}
	return "eventmodeling\n"
}

// commitSource validates newSrc and, if sound, makes it the draft's source
// with note appended to the transcript. It returns false (and records the
// error on the session) when the edit is rejected.
func (a *App) commitSource(s *Session, d *DraftVersion, newSrc, note string) bool {
	log := a.sessionLog(s)
	svg, err := renderEvml(newSrc)
	if err != nil {
		s.mu.Lock()
		s.LastError = err.Error()
		s.mu.Unlock()
		log.Info("action: edit rejected", "draft_id", d.ID, "error", err)
		return false
	}
	d.EvmlSource = newSrc
	d.SVG = svg
	d.ParseError = ""
	d.UpdatedAt = time.Now()
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: note, At: time.Now()})
	if err := a.store.Save(d); err != nil {
		log.Warn("saving draft failed", "draft_id", d.ID, "error", err)
	}
	s.mu.Lock()
	s.LastError = ""
	s.mu.Unlock()
	log.Info("action: edit applied", "draft_id", d.ID, "note", note, "evml_len", len(newSrc))
	return true
}

func (a *App) handleLens(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	var signals struct {
		Lens string `json:"lens"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch signals.Lens {
	case "current", "staging", "all":
	default:
		http.Error(w, "lens must be current, staging or all", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.Lens = signals.Lens
	s.mu.Unlock()
	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: lens changed", "lens", signals.Lens)
	a.patchWorkspace(w, r, s)
}

func (a *App) handleApplySource(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	var signals struct {
		Source string `json:"source"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	src := strings.ReplaceAll(signals.Source, "\r\n", "\n")
	if strings.TrimSpace(src) == "" {
		src = "eventmodeling\n"
	}
	if !strings.HasSuffix(src, "\n") {
		src += "\n"
	}
	before, _ := evml.Parse(currentSource(fs, d))
	after, err := evml.Parse(src)
	note := "Edited the model text directly."
	if err == nil && before != nil {
		if lines := evml.Diff(before, after).Summary(); len(lines) > 0 {
			note = "Edited the model text: " + strings.Join(lines, " · ")
		}
	}
	a.commitSource(s, d, src, note)
	a.patchWorkspace(w, r, s)
}

func (a *App) handleAddStep(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	var sig struct {
		StepType    string `json:"stepType"`
		StepName    string `json:"stepName"`
		StepPayload string `json:"stepPayload"`
		StepActor   string `json:"stepActor"`
		StepStage   string `json:"stepStage"`
		StepAfter   string `json:"stepAfter"`
		StepSources string `json:"stepSources"`
		StepReset   bool   `json:"stepReset"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	src := currentSource(fs, d)
	m, err := parseEvml(src)
	if err != nil {
		a.rejectEdit(w, r, s, "The current model doesn't parse, fix it in the Source tab first: "+err.Error())
		return
	}
	var sources []string
	for _, part := range strings.Split(sig.StepSources, ",") {
		if p := strings.TrimSpace(part); p != "" {
			sources = append(sources, p)
		}
	}
	id := NextFrameID(m)
	line, err := FormatFrameLine(id, StepInput{
		Type: sig.StepType, Name: sig.StepName, Payload: sig.StepPayload,
		Actor: sig.StepActor, Stage: sig.StepStage, Sources: sources, Reset: sig.StepReset,
	})
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	newSrc, err := InsertAfterFrame(src, m, strings.TrimSpace(sig.StepAfter), line)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	stage := sig.StepStage
	if stage == "" {
		stage = string(evml.StageStaging)
	}
	a.commitSource(s, d, newSrc, fmt.Sprintf("Added step %s (%s) as %s.", id, strings.TrimSpace(line), stage))
	a.patchWorkspaceSelecting(w, r, s, id)
}

func (a *App) handleSetStage(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	frameID := r.PathValue("frame")
	var sig struct {
		Stage string `json:"stage"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stage, ok := evml.ParseStage(sig.Stage)
	if !ok {
		http.Error(w, "unknown stage", http.StatusBadRequest)
		return
	}
	src := currentSource(fs, d)
	m, err := parseEvml(src)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	f := m.FrameByID(frameID)
	if f == nil {
		http.Error(w, "unknown step", http.StatusNotFound)
		return
	}
	newSrc, err := SetFrameStage(src, m, f, stage)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	a.commitSource(s, d, newSrc, fmt.Sprintf("Step %s %s is now %s.", f.ID, f.Identifier, stage))
	a.patchWorkspaceSelecting(w, r, s, frameID)
}

func (a *App) handleRemoveStep(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	frameID := r.PathValue("frame")
	src := currentSource(fs, d)
	m, err := parseEvml(src)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	f := m.FrameByID(frameID)
	if f == nil {
		http.Error(w, "unknown step", http.StatusNotFound)
		return
	}
	newSrc, err := RemoveFrame(src, m, f)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	if a.commitSource(s, d, newSrc, fmt.Sprintf("Removed step %s %s and its notes, questions and scenarios.", f.ID, f.Identifier)) {
		a.patchWorkspaceSelecting(w, r, s, "")
		return
	}
	a.patchWorkspace(w, r, s)
}

func (a *App) handleAddScenario(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	var sig struct {
		Frame string `json:"scnFrame"`
		Label string `json:"scnLabel"`
		Given string `json:"scnGiven"`
		When  string `json:"scnWhen"`
		Then  string `json:"scnThen"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	block, err := FormatScenario(ScenarioInput{FrameID: strings.TrimSpace(sig.Frame), Label: sig.Label, Given: sig.Given, When: sig.When, Then: sig.Then})
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	newSrc := AppendBlock(currentSource(fs, d), block)
	a.commitSource(s, d, newSrc, fmt.Sprintf("Added scenario %q to step %s.", strings.TrimSpace(sig.Label), strings.TrimSpace(sig.Frame)))
	a.patchWorkspaceSelecting(w, r, s, strings.TrimSpace(sig.Frame))
}

func (a *App) handleAddHotspot(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	var sig struct {
		Frame string `json:"hotFrame"`
		Text  string `json:"hotText"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	frame := strings.TrimSpace(sig.Frame)
	if frame == "" {
		a.rejectEdit(w, r, s, "pick the step the question is about")
		return
	}
	block, err := FormatHotspot(frame, sig.Text)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	newSrc := AppendBlock(currentSource(fs, d), block)
	a.commitSource(s, d, newSrc, fmt.Sprintf("Opened a question on step %s: %s", frame, strings.TrimSpace(sig.Text)))
	a.patchWorkspaceSelecting(w, r, s, frame)
}

func (a *App) handleResolveHotspot(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, "bad hotspot index", http.StatusBadRequest)
		return
	}
	var sig struct {
		Action   string `json:"resolveAction"` // note | remove
		Decision string `json:"resolveText"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	src := currentSource(fs, d)
	m, err := parseEvml(src)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	if idx < 0 || idx >= len(m.Hotspots) {
		http.Error(w, "unknown question", http.StatusNotFound)
		return
	}
	h := m.Hotspots[idx]
	var newSrc, note string
	switch sig.Action {
	case "remove":
		newSrc, err = RemoveLines(src, h.Line, h.LineCount)
		note = fmt.Sprintf("Dropped the question on step %s (no longer applies).", h.SourceID)
	default:
		decision := strings.TrimSpace(sig.Decision)
		if decision == "" {
			a.rejectEdit(w, r, s, "write down the decision that resolves this question")
			return
		}
		if strings.ContainsAny(decision, "{}") {
			a.rejectEdit(w, r, s, "the decision text cannot contain braces")
			return
		}
		question := strings.TrimSpace(evml.StripOuterBraces(h.Value))
		newSrc, err = ReplaceLines(src, h.Line, h.LineCount, FormatNote(h.SourceID, "Q: "+question+"\nDecision: "+decision))
		note = fmt.Sprintf("Resolved the question on step %s: %s", h.SourceID, decision)
	}
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	a.commitSource(s, d, newSrc, note)
	a.patchWorkspaceSelecting(w, r, s, h.SourceID)
}

func (a *App) handleAddSlice(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	var sig struct {
		Name   string `json:"sliceName"`
		Start  string `json:"sliceStart"`
		End    string `json:"sliceEnd"`
		Status string `json:"sliceStatus"`
		Stage  string `json:"sliceStage"`
		Kind   string `json:"sliceKind"` // slice | chapter
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	src := currentSource(fs, d)
	m, err := parseEvml(src)
	if err != nil {
		a.rejectEdit(w, r, s, err.Error())
		return
	}
	var line string
	if sig.Kind == "chapter" {
		name := strings.TrimSpace(sig.Name)
		if name == "" || sig.Start == "" || sig.End == "" {
			a.rejectEdit(w, r, s, "a chapter needs a name, a first step and a last step")
			return
		}
		line = fmt.Sprintf("chapter %q %s-%s", strings.ReplaceAll(name, `"`, "'"), strings.TrimSpace(sig.Start), strings.TrimSpace(sig.End))
	} else {
		line, err = FormatSlice(SliceInput{Name: sig.Name, Start: strings.TrimSpace(sig.Start), End: strings.TrimSpace(sig.End), Status: sig.Status, Stage: sig.Stage})
		if err != nil {
			a.rejectEdit(w, r, s, err.Error())
			return
		}
	}
	// Declarations go before the first frame so they read like a table of
	// contents at the top of the file.
	newSrc := src
	if len(m.Frames) > 0 {
		first := m.Frames[0]
		for _, f := range m.Frames {
			if f.Line < first.Line {
				first = f
			}
		}
		newSrc, err = ReplaceLines(src, first.Line, 1, line+"\n\n"+strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")[first.Line-1])
		if err != nil {
			a.rejectEdit(w, r, s, err.Error())
			return
		}
	} else {
		newSrc = AppendBlock(src, line)
	}
	a.commitSource(s, d, newSrc, "Added "+line)
	a.patchWorkspace(w, r, s)
}

func (a *App) handleExportEvml(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", d.ID+".evml"))
	_, _ = w.Write([]byte(currentSource(fs, d)))
}

func (a *App) handleExportSVG(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	fs, d, ok := a.draftFor(w, r, s)
	if !ok {
		return
	}
	s.mu.Lock()
	lens := s.Lens
	s.mu.Unlock()
	svg, err := renderLens(currentSource(fs, d), lens)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", d.ID+".svg"))
	_, _ = w.Write([]byte(svg))
}

// rejectEdit records msg as the session's last error and re-renders so the
// expert sees it next to the form they used.
func (a *App) rejectEdit(w http.ResponseWriter, r *http.Request, s *Session, msg string) {
	s.mu.Lock()
	s.LastError = msg
	s.mu.Unlock()
	a.sessionLog(s).Info("action: edit rejected", "error", msg)
	a.patchWorkspace(w, r, s)
}
