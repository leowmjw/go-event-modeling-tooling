package webapp

import (
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func (a *App) handleChat(w http.ResponseWriter, r *http.Request) {
	s := a.sessions.ForRequest(w, r)
	log := a.sessionLog(s)
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

	log.Info("action: chat message received", "flow", flow, "draft_id", draftID, "model_id", modelID, "message_len", len(signals.Message))

	sse := datastar.NewSSE(w, r)
	ctx := sse.Context()

	// Append the user's turn and show it immediately.
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleUser, Content: signals.Message, At: time.Now()})
	if err := a.patchChatLog(sse, d); err != nil {
		return
	}

	llm, err := a.llmFor(ctx, modelID)
	if err != nil {
		log.Warn("action: chat model load failed", "model_id", modelID, "error", err)
		a.appendSystemNote(sse, d, "Couldn't load model "+modelID+": "+err.Error())
		return
	}

	assistantIdx := len(d.Transcript)
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleAssistant, Content: "", At: time.Now()})

	streamStart := time.Now()
	deltaCount := 0
	full, err := llm.StreamChat(ctx, a.systemPrompt, d.Transcript[:assistantIdx], func(delta string) error {
		deltaCount++
		d.Transcript[assistantIdx].Content += delta
		return a.patchChatLog(sse, d)
	})
	if err != nil {
		log.Warn("action: chat generation failed", "draft_id", draftID, "deltas", deltaCount, "duration_ms", time.Since(streamStart).Milliseconds(), "error", err)
		d.Transcript[assistantIdx].Content = full
		a.appendSystemNote(sse, d, "Model error: "+err.Error())
		return
	}
	d.Transcript[assistantIdx].Content = full
	log.Info("action: chat generation complete", "draft_id", draftID, "deltas", deltaCount, "response_len", len(full), "duration_ms", time.Since(streamStart).Milliseconds())

	evmlSrc, hasBlock := ExtractEvml(full)
	if !hasBlock {
		// Pure clarifying question / no proposed change yet — nothing to
		// parse or persist beyond the transcript update already sent.
		log.Info("action: chat response had no evml block", "draft_id", draftID)
		_ = a.store.Save(d)
		a.patchChatLog(sse, d)
		return
	}

	svg, renderErr := renderEvml(evmlSrc)
	if renderErr != nil {
		log.Info("action: chat evml failed validation", "draft_id", draftID, "error", renderErr)
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
		log.Warn("saving draft failed", "draft_id", d.ID, "error", err)
	}
	log.Info("action: chat evml applied", "draft_id", draftID, "evml_len", len(evmlSrc))

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
