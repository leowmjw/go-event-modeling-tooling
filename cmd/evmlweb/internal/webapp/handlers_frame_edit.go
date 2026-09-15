package webapp

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

// frameEditRequest is the payload for renaming or restaging a frame
// inline from the workspace right-rail.
type frameEditRequest struct {
	FrameID  string `json:"frameId"`
	Name     string `json:"name"`
	EntityType string `json:"entityType"`
}

func (a *App) handleUpdateFrame(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")
	frameID := r.PathValue("fid")

	var req frameEditRequest
	if err := datastar.ReadSignals(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d, ok := a.lockedDraft(s, flow, draftID)
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}

	// We don't mutate the .evml source directly — we re-serialise by
	// appending a tiny note in the chat instead, leaving the source for
	// the LLM's next rewrite. For pure inline edits, replace the frame
	// line in the source via a targeted string substitution.
	updated := updateFrameLine(d.EvmlSource, frameID, req)
	if updated == d.EvmlSource {
		http.Error(w, "no change requested", http.StatusBadRequest)
		return
	}

	if err := applyDraftSource(d, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.sessionLog(s).Info("action: frame updated",
		"flow", flow, "draft_id", draftID, "frame_id", frameID,
		"name", req.Name, "entity_type", req.EntityType)
	a.patchWorkspace(w, r, s)
}

// updateFrameLine replaces the entity type and identifier on a single
// tf/rf line identified by frameID. Returns the updated source, or the
// original source when no change was applicable.
func updateFrameLine(src, frameID string, req frameEditRequest) string {
	if req.Name == "" && req.EntityType == "" {
		return src
	}
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Match `tf 03 ...` or `rf 03 ...` where `03` is the frame ID.
		if !strings.HasPrefix(trimmed, "tf ") && !strings.HasPrefix(trimmed, "rf ") {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) < 4 {
			continue
		}
		if parts[1] != frameID {
			continue
		}
		// parts[2] = current entity type; parts[3] = current identifier
		oldType := parts[2]
		oldName := parts[3]
		newType := req.EntityType
		if newType == "" {
			newType = oldType
		}
		newName := req.Name
		if newName == "" {
			newName = oldName
		}
		// Preserve the tail of the line (sources / dataRef / payload).
		// parts[3] is the identifier; everything after is the tail.
		tail := ""
		idx := strings.Index(line, oldName)
		if idx >= 0 {
			tail = line[idx+len(oldName):]
		}
		newLine := strings.TrimRight(parts[0]+" "+frameID+" "+newType+" "+newName, " ") + tail
		lines[i] = newLine
		return strings.Join(lines, "\n")
	}
	return src
}

// applyDraftSource parses, validates, renders, and persists the new
// source for d. On any failure, d is left untouched and the error is
// returned (HTTP 400).
func applyDraftSource(d *DraftVersion, src string) error {
	svg, err := renderEvml(src)
	if err != nil {
		return err
	}
	d.EvmlSource = src
	d.SVG = svg
	d.ParseError = ""
	d.UpdatedAt = time.Now()
	return nil
}

// handleDeleteFrame removes a frame from the active draft's source by
// dropping its declaration line and any inbound/outbound ->> references
// to it. Best-effort — for very large files we still re-parse from
// scratch each time, so correctness comes from validation, not from
// cleverness here.
func (a *App) handleDeleteFrame(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")
	frameID := r.PathValue("fid")

	d, ok := a.lockedDraft(s, flow, draftID)
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}

	updated := deleteFrameLine(d.EvmlSource, frameID)
	if updated == d.EvmlSource {
		http.Error(w, "frame not found", http.StatusNotFound)
		return
	}
	if err := applyDraftSource(d, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sessionLog(s).Info("action: frame deleted", "flow", flow, "draft_id", draftID, "frame_id", frameID)
	a.patchWorkspace(w, r, s)
}

// deleteFrameLine removes the frame's tf/rf line and any references to
// it from other frames' source lists.
func deleteFrameLine(src, frameID string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Drop the frame declaration itself.
		if (strings.HasPrefix(trimmed, "tf ") || strings.HasPrefix(trimmed, "rf ")) &&
			len(strings.Fields(trimmed)) >= 2 &&
			strings.Fields(trimmed)[1] == frameID {
			continue
		}
		// Drop ->> frameID references on other lines.
		if strings.Contains(line, "->> "+frameID) {
			cleaned := stripSourceRef(line, frameID)
			if cleaned != "" {
				out = append(out, cleaned)
			}
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func stripSourceRef(line, frameID string) string {
	// Repeatedly drop "->> frameID" tokens; collapse double spaces.
	tokens := strings.Fields(line)
	out := make([]string, 0, len(tokens))
	drop := false
	for _, t := range tokens {
		if t == "->>" {
			drop = true
			continue
		}
		if drop {
			drop = false
			if t == frameID {
				continue
			}
			out = append(out, "->>", t)
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, " ")
}

// hotspotAddRequest is the payload for the "+ Add hotspot" form.
type hotspotAddRequest struct {
	FrameID string `json:"frameId"`
	Body    string `json:"body"`
}

func (a *App) handleAddHotspot(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	var req hotspotAddRequest
	if err := datastar.ReadSignals(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		http.Error(w, "hotspot body is required", http.StatusBadRequest)
		return
	}
	if !isFrameID(req.FrameID) {
		http.Error(w, "frameId must be 1-3 digits", http.StatusBadRequest)
		return
	}

	d, ok := a.lockedDraft(s, flow, draftID)
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	// Wrap the body in braces if the user didn't, so we always emit a
	// valid hotspot block.
	if !strings.HasPrefix(body, "{") {
		body = "{\n" + body + "\n}"
	}
	insert := "\nhotspot " + req.FrameID + " " + body + "\n"
	updated := d.EvmlSource + insert

	if err := applyDraftSource(d, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sessionLog(s).Info("action: hotspot added",
		"flow", flow, "draft_id", draftID, "frame_id", req.FrameID)
	a.patchWorkspace(w, r, s)
}

func (a *App) handleResolveHotspot(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")
	frameID := r.PathValue("fid")

	d, ok := a.lockedDraft(s, flow, draftID)
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}
	updated := resolveHotspot(d.EvmlSource, frameID)
	if updated == d.EvmlSource {
		http.Error(w, "no open hotspot on that frame", http.StatusNotFound)
		return
	}
	if err := applyDraftSource(d, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.sessionLog(s).Info("action: hotspot resolved",
		"flow", flow, "draft_id", draftID, "frame_id", frameID)
	a.patchWorkspace(w, r, s)
}

// resolveHotspot adds `status resolved` to the first hotspot attached to
// frameID. Best-effort — leaves the source unchanged when no matching
// hotspot exists.
func resolveHotspot(src, frameID string) string {
	lines := strings.Split(src, "\n")
	prefix := "hotspot " + frameID + " "
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, prefix) {
			continue
		}
		// Already resolved.
		if strings.Contains(t, "status resolved") {
			return src
		}
		// Insert "status resolved " right after the frame ID token.
		// Re-join: `hotspot <fid> status resolved { … body … }`.
		rest := strings.TrimPrefix(t, prefix)
		lines[i] = prefix + "status resolved " + rest
		return strings.Join(lines, "\n")
	}
	return src
}

func isFrameID(s string) bool {
	if s == "" || len(s) > 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

// lockedDraft grabs the draft from session state under its mutex, then
// returns it (still under the session's broader mutex — see Session.mu).
func (a *App) lockedDraft(s *Session, flow, draftID string) (*DraftVersion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs, ok := s.Flows[flow]
	if !ok {
		return nil, false
	}
	d, ok := fs.Drafts[draftID]
	return d, ok
}