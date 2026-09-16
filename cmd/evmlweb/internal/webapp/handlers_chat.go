package webapp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

// maxRepairAttempts is how many times the chat loop feeds a broken .evml
// block (with its errors) back to the model for a corrected full document
// before giving up and surfacing the problem to the expert.
const maxRepairAttempts = 2

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

	// Generate, then — when the model proposes a broken or untidy .evml
	// document — feed the problems back and let it repair itself before
	// anything is shown as an error.
	for attempt := 0; ; attempt++ {
		full, err := a.generate(ctx, sse, s, d, modelID)
		if err != nil {
			log.Warn("action: chat generation failed", "draft_id", draftID, "attempt", attempt, "error", err)
			a.appendSystemNote(sse, d, "Model error: "+err.Error())
			return
		}

		evmlSrc, hasBlock := ExtractEvml(full)
		if !hasBlock {
			// Pure clarifying question / no proposed change yet — nothing
			// to parse or persist beyond the transcript already sent.
			log.Info("action: chat response had no evml block", "draft_id", draftID)
			_ = a.store.Save(d)
			a.patchChatLog(sse, d)
			return
		}

		svg, parseErr, issues := evaluateEvml(evmlSrc)
		if parseErr == "" && len(issues) == 0 {
			d.EvmlSource = evmlSrc
			d.SVG = svg
			d.ParseError = ""
			d.ValidationIssues = ""
			d.UpdatedAt = time.Now()
			if err := a.store.Save(d); err != nil {
				log.Warn("saving draft failed", "draft_id", d.ID, "error", err)
			}
			log.Info("action: chat evml applied", "draft_id", draftID, "evml_len", len(evmlSrc), "attempts", attempt+1)
			a.patchWorkspaceSSE(sse, s)
			return
		}

		if attempt >= maxRepairAttempts {
			// Out of retries: keep the last good source, surface the
			// problem. Parse errors block the diagram; wiring issues are
			// recorded but the previous diagram stays.
			log.Info("action: chat evml failed validation after repairs", "draft_id", draftID, "parse_error", parseErr, "issues", len(issues))
			if parseErr != "" {
				d.ParseError = parseErr
			} else {
				d.ValidationIssues = ValidationErrorsText(issues)
			}
			_ = a.store.Save(d)
			a.patchWorkspaceSSE(sse, s)
			return
		}

		// One more chance: tell the model exactly what was wrong.
		var problem string
		if parseErr != "" {
			problem = "it doesn't parse (" + parseErr + ")"
		} else {
			problem = "the wiring rules are violated:\n" + ValidationErrorsText(issues)
		}
		d.Transcript = append(d.Transcript, ChatMessage{
			Role: RoleSystem,
			Content: fmt.Sprintf("That .evml block had a problem — %s. "+
				"Please reply with the complete corrected .evml document.", problem),
			At: time.Now(),
		})
		_ = a.patchChatLog(sse, d)
	}
}

// generate appends an empty assistant turn to the transcript, streams the
// model's response into it (patching the chat log per delta), and returns
// the full response text.
func (a *App) generate(ctx context.Context, sse *datastar.ServerSentEventGenerator, s *Session, d *DraftVersion, modelID string) (string, error) {
	assistantIdx := len(d.Transcript)
	d.Transcript = append(d.Transcript, ChatMessage{Role: RoleAssistant, Content: "", At: time.Now()})

	streamStart := time.Now()
	deltaCount := 0
	full, err := a.chatFn(ctx, modelID, a.systemPrompt, d.Transcript[:assistantIdx], func(delta string) error {
		deltaCount++
		d.Transcript[assistantIdx].Content += delta
		return a.patchChatLog(sse, d)
	})
	if full != "" {
		d.Transcript[assistantIdx].Content = full
	}
	a.sessionLog(s).Info("action: chat generation complete", "draft_id", d.ID, "deltas", deltaCount, "response_len", len(full), "duration_ms", time.Since(streamStart).Milliseconds())
	return full, err
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
