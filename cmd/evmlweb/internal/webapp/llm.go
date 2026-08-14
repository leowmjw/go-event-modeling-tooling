package webapp

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/ardanlabs/kronk/sdk/kronk/vram"
	"github.com/ardanlabs/kronk/sdk/tools/models"
)

// maxModelBytes is the VRAM/on-disk-size ceiling used to keep the model
// picker limited to models that stay fast on ordinary local hardware.
const maxModelBytes = 10 * 1024 * 1024 * 1024 // 10GB

// defaultModelID is preferred whenever it's present and under the size
// ceiling: a 1-bit ~1.2GB model that stays fast even on modest hardware.
const defaultModelID = "Bonsai-8B"

// ModelChoice is one locally downloaded, size-eligible model offered in
// the model dropdown.
type ModelChoice struct {
	ID           string
	SizeBytes    int64
	VRAMEstBytes int64 // best-effort estimate; falls back to SizeBytes on failure
}

// ListModelChoices returns every locally downloaded model under
// maxModelBytes, sorted with defaultModelID first (if present), then by
// ascending size.
func ListModelChoices(m *models.Models) ([]ModelChoice, error) {
	if m == nil {
		return nil, nil
	}
	files, err := m.Files()
	if err != nil {
		return nil, fmt.Errorf("listing local models: %w", err)
	}

	var choices []ModelChoice
	for _, f := range files {
		if !f.Validated || f.Size <= 0 || f.Size > maxModelBytes {
			continue
		}
		vramBytes := f.Size
		if res, err := m.CalculateVRAM(f.ID, vram.Config{
			ContextWindow:   4096,
			BytesPerElement: 2,
			Slots:           1,
		}); err == nil {
			vramBytes = res.TotalVRAM
		}
		if vramBytes > maxModelBytes {
			continue
		}
		choices = append(choices, ModelChoice{ID: f.ID, SizeBytes: f.Size, VRAMEstBytes: vramBytes})
	}

	sort.Slice(choices, func(i, j int) bool {
		if choices[i].ID == defaultModelID {
			return true
		}
		if choices[j].ID == defaultModelID {
			return false
		}
		return choices[i].SizeBytes < choices[j].SizeBytes
	})
	return choices, nil
}

// DefaultModelID returns defaultModelID if it's among choices, otherwise
// the first (smallest) choice, or "" if choices is empty.
func DefaultModelID(choices []ModelChoice) string {
	for _, c := range choices {
		if c.ID == defaultModelID {
			return c.ID
		}
	}
	if len(choices) > 0 {
		return choices[0].ID
	}
	return ""
}

// slogAppLogger adapts an *slog.Logger to Kronk's applog.Logger shape
// (func(ctx, msg, args...)).
func slogAppLogger(log *slog.Logger) func(ctx context.Context, msg string, args ...any) {
	return func(ctx context.Context, msg string, args ...any) {
		log.InfoContext(ctx, msg, args...)
	}
}

// LLM wraps a loaded Kronk model for chat.
type LLM struct {
	ModelID string
	krn     *kronk.Kronk
}

// LoadLLM resolves modelID's local GGUF file(s) and loads it via Kronk,
// ready for chat. Callers must call Close when done with this model
// (e.g. when the user switches the dropdown selection).
func LoadLLM(ctx context.Context, m *models.Models, modelID string) (*LLM, error) {
	path, err := m.FullPath(modelID)
	if err != nil {
		return nil, fmt.Errorf("resolving local path for %q: %w", modelID, err)
	}

	krn, err := kronk.NewWithContext(ctx,
		model.WithModelFiles(path.ModelFiles),
		model.WithAutoTune(true),
	)
	if err != nil {
		return nil, fmt.Errorf("loading model %q: %w", modelID, err)
	}

	return &LLM{ModelID: modelID, krn: krn}, nil
}

// Close unloads the underlying model.
func (l *LLM) Close(ctx context.Context) error {
	if l == nil || l.krn == nil {
		return nil
	}
	return l.krn.Unload(ctx)
}

// StreamChat sends the full conversation (system prompt + transcript) to
// the model and streams back content deltas via onDelta. It returns the
// full assembled response text.
func (l *LLM) StreamChat(ctx context.Context, systemPrompt string, transcript []ChatMessage, onDelta func(delta string) error) (string, error) {
	msgs := make([]model.D, 0, len(transcript)+1)
	msgs = append(msgs, model.TextMessage(model.RoleSystem, systemPrompt))
	for _, m := range transcript {
		msgs = append(msgs, model.TextMessage(string(m.Role), m.Content))
	}

	req := model.D{
		"messages":    model.DocumentArray(msgs...),
		"temperature": 0.4,
		"top_p":       0.9,
		"max_tokens":  4096,
	}

	ch, err := l.krn.ChatStreaming(ctx, req)
	if err != nil {
		return "", fmt.Errorf("chat streaming: %w", err)
	}

	var full string
	for resp := range ch {
		if len(resp.Choices) == 0 {
			continue
		}
		choice := resp.Choices[0]
		if choice.FinishReason() == "error" {
			msg := ""
			if choice.Delta != nil {
				msg = choice.Delta.Content
			}
			return full, fmt.Errorf("model error: %s", msg)
		}
		if choice.Delta == nil || choice.Delta.Content == "" {
			continue
		}
		full += choice.Delta.Content
		if onDelta != nil {
			if err := onDelta(choice.Delta.Content); err != nil {
				return full, err
			}
		}
	}
	return full, nil
}
