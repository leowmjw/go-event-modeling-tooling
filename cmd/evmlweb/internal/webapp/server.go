package webapp

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ardanlabs/kronk/sdk/tools/models"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

// Config holds everything the App needs to start that isn't discovered at
// runtime.
type Config struct {
	Addr        string
	RepoRoot    string // root of go-event-modeling-tooling (for EVENT_MODELING.md/SKILL.md + testdata/fixtures)
	StateDir    string // where in-progress drafts are persisted
	StaticDir   string // cmd/evmlweb/static
	TemplateDir string // cmd/evmlweb/internal/webapp/templates
	// SeedDir holds committed example drafts (cmd/evmlweb/seed/<flow>/…)
	// copied into StateDir the first time a seeded flow is opened. Empty
	// disables seeding.
	SeedDir string
}

// App wires together the HTTP server, session/draft state, the local
// model catalog, and template rendering.
type App struct {
	cfg          Config
	log          *slog.Logger
	tmpl         *template.Template
	sessions     *SessionStore
	store        *DraftStore
	seedStore    *DraftStore // committed example drafts; nil when seeding is off
	models       *models.Models
	systemPrompt string

	llmMu  sync.Mutex
	llms   map[string]*LLM // modelID -> loaded model, process-wide cache
	chatFn chatFunc        // overridable for tests; defaults to llmFor + StreamChat
}

// chatFunc is the seam through which handlers drive the LLM. Tests stub
// it to exercise the chat loop (including the auto-repair loop) without
// loading a real model.
type chatFunc func(ctx context.Context, modelID, systemPrompt string, transcript []ChatMessage, onDelta func(delta string) error) (string, error)

// NewApp constructs an App, loading templates and preparing (but not yet
// loading) the local model catalog.
func NewApp(cfg Config, log *slog.Logger, m *models.Models) (*App, error) {
	tmpl, err := template.ParseGlob(filepath.Join(cfg.TemplateDir, "*.gohtml"))
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	store, err := NewDraftStore(cfg.StateDir, log)
	if err != nil {
		return nil, err
	}

	app := &App{
		cfg:          cfg,
		log:          log,
		tmpl:         tmpl,
		sessions:     NewSessionStore(store, log),
		store:        store,
		models:       m,
		systemPrompt: BuildSystemPrompt(cfg.RepoRoot),
		llms:         make(map[string]*LLM),
	}
	app.chatFn = app.defaultChat

	if cfg.SeedDir != "" {
		seedStore, err := NewDraftStore(cfg.SeedDir, log)
		if err != nil {
			return nil, err
		}
		app.seedStore = seedStore
	}
	return app, nil
}

// defaultChat is the production chatFunc: lazy-load the model, stream the
// response, calling onDelta per token batch.
func (a *App) defaultChat(ctx context.Context, modelID, systemPrompt string, transcript []ChatMessage, onDelta func(delta string) error) (string, error) {
	llm, err := a.llmFor(ctx, modelID)
	if err != nil {
		return "", err
	}
	return llm.StreamChat(ctx, systemPrompt, transcript, onDelta)
}

// Routes builds the HTTP handler for the app.
func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(a.cfg.StaticDir))))

	mux.HandleFunc("GET /{$}", a.handleIndex)
	mux.HandleFunc("POST /model", a.handleSelectModel)
	mux.HandleFunc("POST /flow/select", a.handleSelectFlow)
	mux.HandleFunc("GET /flow/{flow}/draft/{id}", a.handleSwitchDraft)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/chat", a.handleChat)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/new-version", a.handleNewVersion)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/activate", a.handleActivate)

	// Staging: frame inspection + direct manipulation.
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/frame/select", a.handleFrameSelect)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/frame/deselect", a.handleFrameDeselect)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/frame/update", a.handleFrameUpdate)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/frame/add", a.handleFrameAdd)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/frame/sources", a.handleFrameSources)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/frame/delete", a.handleFrameDelete)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/frame/move", a.handleFrameMove)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/compare", a.handleCompare)
	mux.HandleFunc("POST /flow/{flow}/draft/{id}/meta", a.handleDraftMeta)

	return withRequestLogging(a.log, mux)
}

// withRequestLogging logs every request's method, path, status, duration,
// and (when present) the caller's session cookie token, so a session token
// reported from the browser can be grepped straight out of the server log
// to reconstruct exactly what that browser did and when.
func withRequestLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		token := ""
		if c, err := r.Cookie(sessionCookieName); err == nil {
			token = c.Value
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"session", token,
		)
	})
}

// statusRecorder captures the status code written to an http.ResponseWriter
// so logging middleware can report it after the handler returns. It
// forwards Flush so SSE responses (chat streaming) keep working through
// this wrapper — datastar-go's SSE generator needs the underlying
// http.Flusher to push each event as it's written, not just at the end.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// sessionLog returns a logger with s's session token attached, for
// handlers to record per-session lifecycle events (model/flow/draft
// changes, chat turns) in a form that's greppable by that token.
func (a *App) sessionLog(s *Session) *slog.Logger {
	return a.log.With("session", s.Token)
}

// fixturesDir returns cfg.RepoRoot/testdata/fixtures.
func (a *App) fixturesDir() string {
	return filepath.Join(a.cfg.RepoRoot, "testdata", "fixtures")
}

// listFixtures returns every fixture's flow-name slug (filename without
// .evml), sorted.
func (a *App) listFixtures() ([]string, error) {
	entries, err := os.ReadDir(a.fixturesDir())
	if err != nil {
		return nil, fmt.Errorf("reading fixtures dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".evml" {
			continue
		}
		names = append(names, name[:len(name)-len(".evml")])
	}
	sort.Strings(names)
	return names, nil
}

// readFixture parses and renders the baseline .evml for flow, returning
// its source and rendered SVG.
func (a *App) readFixture(flow string) (source, svg string, err error) {
	path := filepath.Join(a.fixturesDir(), flow+".evml")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("reading fixture %q: %w", flow, err)
	}
	source = string(b)
	svg, err = renderEvml(source)
	if err != nil {
		return source, "", err
	}
	return source, svg, nil
}

// renderEvml parses and renders .evml source, returning a parse/validation
// error message (not a Go error) so callers can show it in the chat
// transcript instead of failing the request.
func renderEvml(source string) (string, error) {
	svg, parseErr, issues := evaluateEvml(source)
	if parseErr != "" {
		return "", fmt.Errorf("%s", parseErr)
	}
	if len(issues) > 0 {
		return "", fmt.Errorf("%s", ValidationErrorsText(issues))
	}
	return svg, nil
}

// evaluateEvml splits "can we draw this at all" from "is the wiring
// tidy": a parse error blocks the render (parseErr non-empty), while
// connection-rule violations still render the diagram and come back as
// issues for the UI to surface as warnings.
func evaluateEvml(source string) (svg, parseErr string, issues []error) {
	m, err := evml.Parse(source)
	if err != nil {
		return "", "parse error: " + err.Error(), nil
	}
	issues = evml.ValidateConnections(m)
	svg, _ = evml.RenderSVG(m, evml.RenderOptions{})
	return svg, "", issues
}

// displaySVG renders the SVG the expert actually sees: interactive (every
// frame box clickable, wired to the current flow/draft's select route),
// with diff highlighting when a compare mode is active, and the selected
// frame outlined. It falls back to the cached SVG when the draft's source
// no longer parses (the parse error is surfaced elsewhere).
func (a *App) displaySVG(fs *FlowState, d *DraftVersion) string {
	fallback := activeSVG(fs, d)
	m, err := evml.Parse(d.EvmlSource)
	if err != nil {
		return fallback
	}

	highlight := map[string]string{}
	if mode := fs.CompareMode; mode != "" {
		var base *evml.Model
		switch mode {
		case "baseline":
			if bm, err := evml.Parse(fs.BaselineEvml); err == nil {
				base = bm
			}
		case "prev":
			if p := previousDraft(fs, d); p != nil {
				if pm, err := evml.Parse(p.EvmlSource); err == nil {
					base = pm
				}
			}
		}
		if base != nil {
			diff := evml.Diff(base, m)
			for id, state := range diff.Frames {
				if state != evml.DiffUnchanged {
					highlight[id] = string(state)
				}
			}
		}
	}
	if fs.SelectedFrame != "" {
		highlight[fs.SelectedFrame] = "selected"
	}

	opts := evml.RenderOptions{
		Interactive: true,
		FrameClickExpr: fmt.Sprintf(
			"@post('/flow/%s/draft/%s/frame/select', {payload:{frame:'{id}'}})",
			fs.Name, d.ID),
		Highlight: highlight,
	}
	svg, err := evml.RenderSVG(m, opts)
	if err != nil {
		return fallback
	}
	return svg
}

// previousDraft returns the draft immediately before d in tab order, or
// nil when d is the oldest.
func previousDraft(fs *FlowState, d *DraftVersion) *DraftVersion {
	var prev *DraftVersion
	for _, id := range fs.DraftOrder {
		if id == d.ID {
			return prev
		}
		if cand, ok := fs.Drafts[id]; ok {
			prev = cand
		}
	}
	return nil
}

// llmFor returns the cached LLM for modelID, loading it on first use.
func (a *App) llmFor(ctx context.Context, modelID string) (*LLM, error) {
	a.llmMu.Lock()
	defer a.llmMu.Unlock()

	if l, ok := a.llms[modelID]; ok {
		return l, nil
	}

	l, err := LoadLLM(ctx, a.models, modelID)
	if err != nil {
		return nil, err
	}
	a.llms[modelID] = l
	a.log.Info("model loaded", "model_id", modelID)
	return l, nil
}
