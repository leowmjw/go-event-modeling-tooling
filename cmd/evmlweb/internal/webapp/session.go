package webapp

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const sessionCookieName = "evmlweb_session"

// Session is one browser's server-side state: which flow it's currently
// looking at, and every flow it has touched (so switching flows and
// switching back restores exactly where the expert left off).
//
// PendingFlow/PendingDraftByFlow are only set momentarily, right after this
// Session was rehydrated from its own persisted snapshot (see SessionStore
// doc) — resumeActiveFlow (handlers_flow.go) and FlowFor consume them to
// restore ActiveFlow and each flow's active draft tab, then leave them
// alone; live state afterward lives in ActiveFlow / FlowState.ActiveDraftID
// as usual.
type Session struct {
	mu                 sync.Mutex
	Token              string
	ActiveFlow         string
	Flows              map[string]*FlowState
	ModelID            string // selected LLM, empty until the picker is used
	PendingFlow        string
	PendingDraftByFlow map[string]string
}

// SessionStore keeps one Session per browser, identified by an opaque
// cookie token. It is the top-level, process-wide piece of mutable state
// for the whole app.
//
// In-memory sessions don't survive a process restart (e.g. air rebuilding
// on a source change during development). Each Session's selection (model,
// active flow, active draft per flow) is persisted to disk keyed by its
// own cookie token, so *that specific browser* resumes exactly where it
// left off after a restart — reusing the same cookie it already has — while
// a genuinely different/fresh browser session (a token never seen before)
// always starts blank rather than inheriting someone else's state.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	store    *DraftStore
	log      *slog.Logger
}

func NewSessionStore(store *DraftStore, log *slog.Logger) *SessionStore {
	if log == nil {
		log = slog.Default()
	}
	return &SessionStore{
		sessions: make(map[string]*Session),
		store:    store,
		log:      log,
	}
}

// ForRequest returns the Session for r. If r's cookie doesn't match a
// live in-memory session (no cookie yet, or the process restarted since it
// was issued), it tries to rehydrate that exact token's persisted
// snapshot before falling back to a genuinely blank session with a new
// token.
func (ss *SessionStore) ForRequest(w http.ResponseWriter, r *http.Request) *Session {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		token := c.Value

		ss.mu.Lock()
		s, ok := ss.sessions[token]
		ss.mu.Unlock()
		if ok {
			return s
		}

		if snap, found, err := ss.store.LoadSessionByToken(token); err != nil {
			ss.log.Warn("loading persisted session snapshot failed", "token", token, "error", err)
		} else if found {
			s := &Session{
				Token:              token,
				Flows:              make(map[string]*FlowState),
				ModelID:            snap.ModelID,
				PendingFlow:        snap.ActiveFlow,
				PendingDraftByFlow: snap.ActiveDraftByFlow,
			}
			ss.mu.Lock()
			ss.sessions[token] = s
			ss.mu.Unlock()
			ss.log.Info("session resumed from disk", "token", token, "model_id", s.ModelID, "flow", s.PendingFlow)
			return s
		}
	}

	token := newToken()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	s := &Session{Token: token, Flows: make(map[string]*FlowState)}
	ss.mu.Lock()
	ss.sessions[token] = s
	ss.mu.Unlock()

	// Persist a (currently blank) snapshot immediately, not just after the
	// first selection change. Without this, a session that's created and
	// then loses its in-memory state to a process restart (e.g. an air
	// rebuild) before ever touching a picker has no file to resume from at
	// all — it silently falls back to a brand new token, discarding the
	// cookie the browser already has.
	ss.PersistSelection(s)

	ss.log.Info("session created", "token", token)
	return s
}

// PersistSelection saves s's current model + active-flow + per-flow
// active-draft selection to disk, keyed by s's own token. Call it after
// any action that changes which model/flow/draft is active (not needed
// for edits within an already-active draft, since draft content is
// persisted separately by DraftStore.Save).
func (ss *SessionStore) PersistSelection(s *Session) {
	s.mu.Lock()
	snap := sessionSnapshot{
		ModelID:           s.ModelID,
		ActiveFlow:        s.ActiveFlow,
		ActiveDraftByFlow: make(map[string]string, len(s.Flows)),
	}
	for name, fs := range s.Flows {
		if fs.ActiveDraftID != "" {
			snap.ActiveDraftByFlow[name] = fs.ActiveDraftID
		}
	}
	token := s.Token
	s.mu.Unlock()

	if err := ss.store.SaveSession(token, snap); err != nil {
		ss.log.Warn("persisting session snapshot failed", "token", token, "error", err)
	}
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// FlowFor returns s's FlowState for name, loading it (baseline + any
// persisted drafts) from disk on first access within this session.
func (ss *SessionStore) FlowFor(s *Session, name string, baselineEvml, baselineSVG string, isNew bool) (*FlowState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if fs, ok := s.Flows[name]; ok {
		return fs, nil
	}

	fs := &FlowState{
		Name:           name,
		BaselineEvml:   baselineEvml,
		BaselineSVG:    baselineSVG,
		IsNew:          isNew,
		Drafts:         make(map[string]*DraftVersion),
		NextSeqForDate: make(map[string]int),
	}

	drafts, err := ss.store.LoadFlow(name)
	if err != nil {
		return nil, err
	}
	for _, d := range drafts {
		fs.Drafts[d.ID] = d
		fs.DraftOrder = append(fs.DraftOrder, d.ID)
		if d.Seq >= fs.NextSeqForDate[d.Date] {
			fs.NextSeqForDate[d.Date] = d.Seq + 1
		}
	}
	if len(fs.DraftOrder) > 0 {
		fs.ActiveDraftID = fs.DraftOrder[len(fs.DraftOrder)-1]
	}
	if preferred, ok := s.PendingDraftByFlow[name]; ok {
		if _, exists := fs.Drafts[preferred]; exists {
			fs.ActiveDraftID = preferred
		}
	}

	s.Flows[name] = fs
	return fs, nil
}

// NewDraft forks source (nil for a blank draft) into a new dated tab for
// flow, persists it, and makes it the flow's active draft.
func (ss *SessionStore) NewDraft(fs *FlowState, source *DraftVersion, now time.Time) (*DraftVersion, error) {
	date := now.Format("2006-01-02")
	seq := fs.NextSeqForDate[date]
	if seq == 0 {
		seq = 1
	}
	fs.NextSeqForDate[date] = seq + 1

	d := &DraftVersion{
		ID:        NewDraftID(fs.Name, date, seq),
		FlowName:  fs.Name,
		Date:      date,
		Seq:       seq,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if source != nil {
		d.EvmlSource = source.EvmlSource
		d.SVG = source.SVG
		d.Transcript = append([]ChatMessage(nil), source.Transcript...)
	} else {
		d.EvmlSource = fs.BaselineEvml
		d.SVG = fs.BaselineSVG
	}

	if err := ss.store.Save(d); err != nil {
		return nil, err
	}

	fs.Drafts[d.ID] = d
	fs.DraftOrder = append(fs.DraftOrder, d.ID)
	fs.ActiveDraftID = d.ID
	return d, nil
}
