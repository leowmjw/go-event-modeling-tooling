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
	return d, nil
}
