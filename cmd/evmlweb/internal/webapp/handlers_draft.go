package webapp

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// handleActivate publishes a draft as the flow's new baseline. Only drafts
// in the Now lane that the group has accepted can be published; the
// previous baseline is backed up first so publishing is never silent.
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
	if d.Horizon != HorizonNow || d.Status != StatusAccepted {
		http.Error(w, "only an accepted Now-lane draft can be published — move it to Now and mark it accepted first", http.StatusConflict)
		return
	}

	if err := os.MkdirAll(a.fixturesDir(), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dest := filepath.Join(a.fixturesDir(), flow+".evml")
	if prev, err := os.ReadFile(dest); err == nil {
		backupDir := filepath.Join(a.fixturesDir(), ".backups")
		if err := os.MkdirAll(backupDir, 0o755); err == nil {
			backup := filepath.Join(backupDir, fmt.Sprintf("%s-%s.evml", flow, time.Now().Format("20060102-150405")))
			_ = os.WriteFile(backup, prev, 0o644)
		}
	}
	if err := os.WriteFile(dest, []byte(d.EvmlSource), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.sessionLog(s).Info("action: draft activated", "flow", flow, "draft_id", draftID, "path", dest)

	fs.BaselineEvml = d.EvmlSource
	fs.BaselineSVG = d.SVG
	fs.IsNew = false

	note := fmt.Sprintf("Published as %s — this is now the flow's agreed baseline (previous version backed up).", flow+".evml")
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: note, At: time.Now()})
	_ = a.store.Save(d)

	sse := datastar.NewSSE(w, r)
	a.patchWorkspaceSSE(sse, s)
}

// handleHorizon moves a draft into the Now, Next, or Future staging lane.
func (a *App) handleHorizon(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	var signals struct {
		Horizon string `json:"horizon"`
		Title   string `json:"title"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h := Horizon(signals.Horizon)
	if h != HorizonNow && h != HorizonNext && h != HorizonFuture {
		http.Error(w, "unknown lane — use now, next, or future", http.StatusBadRequest)
		return
	}

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
	d.Horizon = h
	if strings.TrimSpace(signals.Title) != "" {
		d.Title = strings.TrimSpace(signals.Title)
	}
	d.UpdatedAt = time.Now()
	_ = a.store.Save(d)

	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: draft moved lanes", "flow", flow, "draft_id", draftID, "horizon", string(h))
	a.patchWorkspace(w, r, s)
}

// handlePromote copies a staged draft's diagram into a new version in the
// target lane (Next → Now, Future → Next), so promoting never overwrites
// the source variation.
func (a *App) handlePromote(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	var signals struct {
		Horizon string `json:"horizon"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target := Horizon(signals.Horizon)
	if target != HorizonNow && target != HorizonNext {
		http.Error(w, "can only promote into the Now or Next lane", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return
	}
	source, ok := fs.Drafts[draftID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	if source.EvmlSource == "" || source.ParseError != "" {
		http.Error(w, "draft has no valid diagram to promote yet", http.StatusConflict)
		return
	}

	promoted, err := a.sessions.NewDraft(fs, source, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	promoted.Horizon = target
	promoted.Status = StatusStaging
	promoted.Transcript = append(promoted.Transcript, ChatMessage{
		Role:    RoleSystem,
		Content: fmt.Sprintf("Promoted from %s into the %s lane — tweak further here.", laneName(source.Horizon), laneName(target)),
		At:      time.Now(),
	})
	_ = a.store.Save(promoted)

	a.sessions.PersistSelection(s)
	a.sessionLog(s).Info("action: draft promoted", "flow", flow, "source_draft", draftID, "new_draft", promoted.ID, "horizon", string(target))
	a.patchWorkspace(w, r, s)
}

// laneName renders a horizon as plain language for the UI.
func laneName(h Horizon) string {
	switch h {
	case HorizonNow:
		return "Now lane"
	case HorizonFuture:
		return "Future lane"
	default:
		return "Next lane"
	}
}

// handleDiff renders a line-based, frame-aware comparison between two
// drafts (or a draft and the flow baseline) as an SSE patch.
func (a *App) handleDiff(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	fromID := r.URL.Query().Get("from")
	toID := r.URL.Query().Get("to")

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return
	}
	fromSrc, fromLabel := fs.BaselineEvml, "Agreed baseline (Now)"
	if fromID != "" && fromID != "baseline" {
		from, ok := fs.Drafts[fromID]
		if !ok {
			http.Error(w, "unknown draft", http.StatusNotFound)
			return
		}
		fromSrc, fromLabel = from.EvmlSource, draftDisplayName(from)
	}
	to, ok := fs.Drafts[toID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}

	rows := diffEvml(fromSrc, to.EvmlSource)
	view := WorkspacePage{DiffFrom: fromLabel, DiffTo: draftDisplayName(to), DiffRows: rows}
	frag, err := a.renderTemplateToBytes("diff", view)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElements(string(frag), datastar.WithSelectorID("diff-panel"), datastar.WithModeReplace()); err != nil {
		a.sessionLog(s).Warn("patch diff failed", "error", err)
	}
}

// diffRow is one line of a staged comparison.
type diffRow struct {
	Kind string // "same", "added", "removed"
	Text string
}

// diffEvml compares two .evml sources line by line, keeping blank lines
// and comments out of the noise so domain experts see frame changes.
func diffEvml(from, to string) []diffRow {
	fromLines := meaningfulLines(from)
	toLines := meaningfulLines(to)
	fromSet := make(map[string]int, len(fromLines))
	for _, l := range fromLines {
		fromSet[l]++
	}
	toSet := make(map[string]int, len(toLines))
	for _, l := range toLines {
		toSet[l]++
	}
	var rows []diffRow
	for _, l := range fromLines {
		if toSet[l] > 0 {
			toSet[l]--
			rows = append(rows, diffRow{Kind: "same", Text: l})
		} else {
			rows = append(rows, diffRow{Kind: "removed", Text: l})
		}
	}
	for _, l := range toLines {
		if fromSet[l] > 0 {
			fromSet[l]--
			continue
		}
		rows = append(rows, diffRow{Kind: "added", Text: l})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		order := map[string]int{"removed": 0, "added": 1, "same": 2}
		return order[rows[i].Kind] < order[rows[j].Kind]
	})
	return rows
}

func meaningfulLines(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "%%") {
			continue
		}
		out = append(out, t)
	}
	return out
}
