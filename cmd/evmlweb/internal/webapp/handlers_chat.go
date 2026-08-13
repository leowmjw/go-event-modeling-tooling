package webapp

import (
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func (a *App) handleChat(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	flow := r.PathValue("flow")
	draftID := r.PathValue("id")

	var signals struct {
		Message string `json:"message"`
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
	d, ok := fs.Drafts[draftID]
	if !ok {
		http.Error(w, "unknown draft", http.StatusNotFound)
		return
	}

	modelID := s.ModelID
	if modelID == "" {
		http.Error(w, "no model selected", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	ctx := sse.Context()

	// Append the user's turn and show it immediately.
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleUser, Content: signals.Message, At: time.Now()})
	if err := a.patchChatLog(sse, d); err != nil {
		return
	}

	llm, err := a.llmFor(ctx, modelID)
	if err != nil {
		a.appendSystemNote(sse, d, "Couldn't load model "+modelID+": "+err.Error())
		return
	}

	assistantIdx := len(d.Transcript)
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleAssistant, Content: "", At: time.Now()})

	full, err := llm.StreamChat(ctx, a.systemPrompt, d.Transcript[:assistantIdx], func(delta string) error {
		d.Transcript[assistantIdx].Content += delta
		return a.patchChatLog(sse, d)
	})
	if err != nil {
		d.Transcript[assistantIdx].Content = full
		a.appendSystemNote(sse, d, "Model error: "+err.Error())
		return
	}
	d.Transcript[assistantIdx].Content = full

	evmlSrc, hasBlock := ExtractEvml(full)
	if !hasBlock {
		// Pure clarifying question / no proposed change yet — nothing to
		// parse or persist beyond the transcript update already sent.
		_ = a.store.Save(d)
		a.patchChatLog(sse, d)
		return
	}

	svg, renderErr := renderEvml(evmlSrc)
	if renderErr != nil {
		d.ParseError = renderErr.Error()
		_ = a.store.Save(d)
		a.patchWorkspaceSSE(sse, s)
		return
	}

	d.EvmlSource = evmlSrc
	d.SVG = svg
	d.ParseError = ""
	d.UpdatedAt = time.Now()
	if err := a.store.Save(d); err != nil {
		a.log.Warn("saving draft failed", "draft_id", d.ID, "error", err)
	}

	a.patchWorkspaceSSE(sse, s)
}

func (a *App) patchChatLog(sse *datastar.ServerSentEventGenerator, d *DraftVersion) error {
	view := WorkspacePage{Transcript: toChatViews(d.Transcript), ParseError: d.ParseError}
	var buf []byte
	buf, err := a.renderTemplateToBytes("chatlog", view)
	if err != nil {
		a.log.Warn("render chatlog failed", "error", err)
		return err
	}
	return sse.PatchElements(string(buf), datastar.WithSelectorID("chat-log"), datastar.WithModeOuter())
}

func (a *App) appendSystemNote(sse *datastar.ServerSentEventGenerator, d *DraftVersion, note string) {
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleSystem, Content: note, At: time.Now()})
	_ = a.patchChatLog(sse, d)
}

// patchWorkspaceSSE re-renders and patches the whole workspace fragment
// (used when the SVG itself changed, not just the chat log).
func (a *App) patchWorkspaceSSE(sse *datastar.ServerSentEventGenerator, s *Session) {
	frag, err := a.renderWorkspaceFragment(s)
	if err != nil {
		a.log.Warn("render workspace fragment failed", "error", err)
		return
	}
	if err := sse.PatchElements(frag, datastar.WithSelectorID("workspace-inner"), datastar.WithModeOuter()); err != nil {
		a.log.Warn("patch workspace failed", "error", err)
	}
}
