package webapp

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

// renameDraftRequest is the payload sent from the workspace "Rename this
// scenario" form.
type renameDraftRequest struct {
	Label string `json:"label"`
}

func (a *App) handleRenameDraft(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	var req renameDraftRequest
	if err := datastar.ReadSignals(r, &req); err != nil {
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
	d, ok := fs.Drafts[draftID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}

	d.Label = strings.TrimSpace(req.Label)
	d.UpdatedAt = time.Now()
	if err := a.store.Save(d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.sessionLog(s).Info("action: draft renamed", "flow", flow, "draft_id", draftID, "label", d.Label)
	a.patchWorkspace(w, r, s)
}

// handleDuplicateDraft clones the named draft into a new dated version
// (same flow) so the expert can explore a divergent alternative without
// losing the original. The new draft's transcript carries a system note
// recording where it was forked from.
func (a *App) handleDuplicateDraft(w http.ResponseWriter, r *http.Request) {
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
	source, ok := fs.Drafts[draftID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}

	newDraft, err := a.sessions.NewDraft(fs, source, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newDraft.Transcript = append(newDraft.Transcript, ChatMessage{
		Role:    RoleSystem,
		Content: fmt.Sprintf("Forked from %s to explore a divergent alternative.", source.ID),
		At:      time.Now(),
	})
	if err := a.store.Save(newDraft); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.sessionLog(s).Info("action: draft duplicated", "flow", flow, "source_draft", draftID, "new_draft", newDraft.ID)
	a.patchWorkspace(w, r, s)
}

// handleDraftDiff produces a textual unified diff between the active
// draft and the named "other" draft, returning a WorkspacePage.DiffHTML
// field on the SSE patch. For now the response is rendered as a single
// fragment via renderTemplateToBytes — see the diff_fragment template
// fragment below.
func (a *App) handleDraftDiff(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")
	otherID := r.PathValue("otherID")

	s.mu.Lock()
	fs, ok := s.Flows[flow]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return
	}
	aDraft, ok1 := fs.Drafts[draftID]
	bDraft, ok2 := fs.Drafts[otherID]
	if !ok1 || !ok2 {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	diff := unifiedDiff(aDraft.EvmlSource, bDraft.EvmlSource, draftID, otherID)

	sse := datastar.NewSSE(w, r)
	frag := renderDiffFragment(diff)
	if err := sse.PatchElements(frag, datastar.WithSelectorID("diff-panel"), datastar.WithModeInner()); err != nil {
		a.log.Warn("patch diff failed", "error", err)
		return
	}
	a.sessionLog(s).Info("action: draft diff viewed", "flow", flow, "a", draftID, "b", otherID)
}

// unifiedDiff produces a textual line-by-line diff. The output is
// intentionally simple (no external diff library) — it shows removed
// lines prefixed with "- " and added lines with "+ ". Lines that match
// are shown with "  ". This is enough for the workshop UI's "compare two
// drafts" feature.
func unifiedDiff(a, b, aLabel, bLabel string) string {
	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n", aLabel)
	fmt.Fprintf(&out, "+++ %s\n", bLabel)
	i, j := 0, 0
	for i < len(aLines) || j < len(bLines) {
		switch {
		case i < len(aLines) && j < len(bLines) && aLines[i] == bLines[j]:
			fmt.Fprintf(&out, "  %s\n", aLines[i])
			i++
			j++
		case i < len(aLines):
			fmt.Fprintf(&out, "- %s\n", aLines[i])
			i++
		default:
			fmt.Fprintf(&out, "+ %s\n", bLines[j])
			j++
		}
	}
	return out.String()
}

func renderDiffFragment(diff string) string {
	// Lightweight HTML escaping — Datastar patches this as inner HTML so
	// it has to be safe.
	escaped := strings.ReplaceAll(diff, "&", "&amp;")
	escaped = strings.ReplaceAll(escaped, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")
	// Per-line colouring.
	var b strings.Builder
	b.WriteString(`<pre class="diff-pre">`)
	for _, line := range strings.Split(escaped, "\n") {
		switch {
		case strings.HasPrefix(line, "+ "):
			b.WriteString(`<span class="diff-add">`)
			b.WriteString(line)
			b.WriteString("</span>\n")
		case strings.HasPrefix(line, "- "):
			b.WriteString(`<span class="diff-del">`)
			b.WriteString(line)
			b.WriteString("</span>\n")
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			b.WriteString(`<span class="diff-hdr">`)
			b.WriteString(line)
			b.WriteString("</span>\n")
		default:
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString(`</pre>`)
	return b.String()
}
