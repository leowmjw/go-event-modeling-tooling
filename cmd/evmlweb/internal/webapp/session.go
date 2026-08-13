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
type Session struct {
	mu         sync.Mutex
	Token      string
	ActiveFlow string
	Flows      map[string]*FlowState
	ModelID    string // selected LLM, empty until the picker is used
}

// SessionStore keeps one Session per browser, identified by an opaque
// cookie token. It is the top-level, process-wide piece of mutable state
// for the whole app.
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

// ForRequest returns the Session for r, issuing a fresh cookie/token on w
// if the request has none yet.
func (ss *SessionStore) ForRequest(w http.ResponseWriter, r *http.Request) *Session {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		ss.mu.Lock()
		s, ok := ss.sessions[c.Value]
		ss.mu.Unlock()
		if ok {
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
	ss.log.Info("session created", "token", token)
	return s
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
