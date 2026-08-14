package webapp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DraftStore persists draft versions to disk so a flow's in-progress
// sessions survive a server restart. Layout:
//
//	<root>/<flow>/<draftID>.evml   the .evml source
//	<root>/<flow>/<draftID>.json   metadata (transcript, timestamps, parse error)
type DraftStore struct {
	root string
	log  *slog.Logger
}

// draftMeta is the on-disk JSON sidecar for a DraftVersion (everything but
// the .evml source itself, which is stored alongside as plain text so it
// stays diffable/readable on its own).
type draftMeta struct {
	FlowName   string        `json:"flow_name"`
	Date       string        `json:"date"`
	Seq        int           `json:"seq"`
	ParseError string        `json:"parse_error,omitempty"`
	Transcript []ChatMessage `json:"transcript"`
	CreatedAt  string        `json:"created_at"`
	UpdatedAt  string        `json:"updated_at"`
}

// NewDraftStore creates a store rooted at root, creating the directory if
// it doesn't already exist.
func NewDraftStore(root string, log *slog.Logger) (*DraftStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("draftstore: creating root %q: %w", root, err)
	}
	if log == nil {
		log = slog.Default()
	}
	return &DraftStore{root: root, log: log}, nil
}

func (s *DraftStore) flowDir(flow string) string {
	return filepath.Join(s.root, flow)
}

func (s *DraftStore) evmlPath(flow, draftID string) string {
	return filepath.Join(s.flowDir(flow), draftID+".evml")
}

func (s *DraftStore) metaPath(flow, draftID string) string {
	return filepath.Join(s.flowDir(flow), draftID+".json")
}

// Save writes d's .evml source and metadata to disk, overwriting any
// previous snapshot for the same draft ID.
func (s *DraftStore) Save(d *DraftVersion) error {
	dir := s.flowDir(d.FlowName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("draftstore: creating flow dir %q: %w", dir, err)
	}

	if err := os.WriteFile(s.evmlPath(d.FlowName, d.ID), []byte(d.EvmlSource), 0o644); err != nil {
		return fmt.Errorf("draftstore: writing evml for %q: %w", d.ID, err)
	}

	meta := draftMeta{
		FlowName:   d.FlowName,
		Date:       d.Date,
		Seq:        d.Seq,
		ParseError: d.ParseError,
		Transcript: d.Transcript,
		CreatedAt:  d.CreatedAt.Format(timeLayout),
		UpdatedAt:  d.UpdatedAt.Format(timeLayout),
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("draftstore: marshaling meta for %q: %w", d.ID, err)
	}
	if err := os.WriteFile(s.metaPath(d.FlowName, d.ID), b, 0o644); err != nil {
		return fmt.Errorf("draftstore: writing meta for %q: %w", d.ID, err)
	}
	return nil
}

const timeLayout = "2006-01-02T15:04:05Z07:00"

// LoadFlow reads every persisted draft for flow back from disk, keyed by
// draft ID, ordered oldest-first by (date, seq).
func (s *DraftStore) LoadFlow(flow string) ([]*DraftVersion, error) {
	dir := s.flowDir(flow)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("draftstore: reading flow dir %q: %w", dir, err)
	}

	var drafts []*DraftVersion
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".evml") {
			continue
		}
		draftID := strings.TrimSuffix(name, ".evml")
		d, err := s.load(flow, draftID)
		if err != nil {
			s.log.Warn("draftstore: skipping unreadable draft", "flow", flow, "draft_id", draftID, "error", err)
			continue
		}
		drafts = append(drafts, d)
	}

	sort.Slice(drafts, func(i, j int) bool {
		if drafts[i].Date != drafts[j].Date {
			return drafts[i].Date < drafts[j].Date
		}
		return drafts[i].Seq < drafts[j].Seq
	})
	return drafts, nil
}

// sessionSnapshot is one browser session's last-known selection: which
// model, which flow, and (per flow) which draft tab was active. It's keyed
// by that session's own cookie token, so the *same* browser resumes
// exactly where it left off after the dev server restarts (air rebuild) —
// without a genuinely new/different browser session inheriting someone
// else's in-progress state.
type sessionSnapshot struct {
	ModelID           string            `json:"model_id"`
	ActiveFlow        string            `json:"active_flow"`
	ActiveDraftByFlow map[string]string `json:"active_draft_by_flow"`
}

func (s *DraftStore) sessionsDir() string {
	return filepath.Join(s.root, "_sessions")
}

func (s *DraftStore) sessionPath(token string) string {
	return filepath.Join(s.sessionsDir(), token+".json")
}

// SaveSession persists token's selection snapshot, overwriting any
// previous one for that same token.
func (s *DraftStore) SaveSession(token string, snap sessionSnapshot) error {
	if err := os.MkdirAll(s.sessionsDir(), 0o755); err != nil {
		return fmt.Errorf("draftstore: creating sessions dir: %w", err)
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("draftstore: marshaling session snapshot: %w", err)
	}
	if err := os.WriteFile(s.sessionPath(token), b, 0o644); err != nil {
		return fmt.Errorf("draftstore: writing session snapshot: %w", err)
	}
	return nil
}

// LoadSessionByToken reads back token's persisted selection snapshot.
// found is false when nothing has ever been persisted for this exact
// token — the caller should treat that as a genuinely fresh session, not
// an error.
func (s *DraftStore) LoadSessionByToken(token string) (snap sessionSnapshot, found bool, err error) {
	b, err := os.ReadFile(s.sessionPath(token))
	if os.IsNotExist(err) {
		return sessionSnapshot{}, false, nil
	}
	if err != nil {
		return sessionSnapshot{}, false, fmt.Errorf("draftstore: reading session snapshot: %w", err)
	}
	if err := json.Unmarshal(b, &snap); err != nil {
		return sessionSnapshot{}, false, fmt.Errorf("draftstore: parsing session snapshot: %w", err)
	}
	if snap.ActiveDraftByFlow == nil {
		snap.ActiveDraftByFlow = map[string]string{}
	}
	return snap, true, nil
}

func (s *DraftStore) load(flow, draftID string) (*DraftVersion, error) {
	evml, err := os.ReadFile(s.evmlPath(flow, draftID))
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.metaPath(flow, draftID))
	if err != nil {
		return nil, err
	}
	var meta draftMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}

	d := &DraftVersion{
		ID:         draftID,
		FlowName:   meta.FlowName,
		Date:       meta.Date,
		Seq:        meta.Seq,
		EvmlSource: string(evml),
		ParseError: meta.ParseError,
		Transcript: meta.Transcript,
	}
	d.CreatedAt, _ = parseTime(meta.CreatedAt)
	d.UpdatedAt, _ = parseTime(meta.UpdatedAt)

	// The rendered SVG is never persisted (it's fully derived from
	// EvmlSource) — recompute it here so a freshly loaded draft (new
	// session, or the process having restarted) has something to show
	// without waiting for the next chat turn.
	if d.EvmlSource != "" {
		if svg, err := renderEvml(d.EvmlSource); err != nil {
			if d.ParseError == "" {
				d.ParseError = err.Error()
			}
		} else {
			d.SVG = svg
		}
	}
	return d, nil
}
