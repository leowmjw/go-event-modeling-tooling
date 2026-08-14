// Command evmlweb is a local web app that lets non-technical domain
// experts build and iterate on .evml event models conversationally,
// backed by a locally-running LLM via the Kronk SDK.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/ardanlabs/kronk/sdk/tools/libs"
	"github.com/ardanlabs/kronk/sdk/tools/models"

	"github.com/leowmjw/go-event-modeling-tooling/cmd/evmlweb/internal/webapp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("evmlweb exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr     = flag.String("addr", "localhost:8080", "HTTP listen address")
		repoRoot = flag.String("repo-root", ".", "root of the go-event-modeling-tooling checkout (contains testdata/fixtures, EVENT_MODELING.md, SKILL.md)")
		stateDir = flag.String("state-dir", "", "directory to persist in-progress draft versions (default: <evmlweb-module>/.state)")
		logJSON  = flag.Bool("log-json", false, "emit structured logs as JSON instead of text")
	)
	flag.Parse()

	logHandler := logHandlerFor(*logJSON)
	log := slog.New(logHandler)
	slog.SetDefault(log)

	absRepoRoot, err := filepath.Abs(*repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repo root: %w", err)
	}
	if *stateDir == "" {
		// Anchor to the evmlweb module dir (where main.go lives), not repo-root.
		// repo-root defaults to "." when launched from cmd/evmlweb, so joining
		// repo-root + "cmd/evmlweb/.state" would wrongly nest cmd/evmlweb twice.
		*stateDir = filepath.Join(mustSourceDir(), ".state")
	}

	ctx := context.Background()

	if err := initKronk(ctx, log); err != nil {
		return fmt.Errorf("initializing kronk: %w", err)
	}

	m, err := models.New()
	if err != nil {
		return fmt.Errorf("opening local model catalog: %w", err)
	}

	app, err := webapp.NewApp(webapp.Config{
		Addr:        *addr,
		RepoRoot:    absRepoRoot,
		StateDir:    *stateDir,
		StaticDir:   filepath.Join(mustSourceDir(), "static"),
		TemplateDir: filepath.Join(mustSourceDir(), "internal", "webapp", "templates"),
	}, log, m)
	if err != nil {
		return fmt.Errorf("building app: %w", err)
	}

	log.Info("evmlweb starting", "addr", *addr, "repo_root", absRepoRoot, "state_dir", *stateDir)

	srv := &http.Server{
		Addr:    *addr,
		Handler: app.Routes(),
	}

	// Shut down on SIGINT/SIGTERM instead of dying mid-request, so the
	// listening socket is actually released before the process exits —
	// otherwise a hot-reloader (air) that kills and immediately restarts
	// this binary can race the OS into reporting the old port as still in
	// use.
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-sigCtx.Done():
		stop()
		log.Info("evmlweb shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func logHandlerFor(json bool) slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if json {
		return slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.NewTextHandler(os.Stdout, opts)
}

// initKronk downloads (if needed) and initializes the llama.cpp runtime
// libraries this SDK build needs. A machine that already ran the `kronk`
// CLI may have libraries cached for a *different* SDK/llama.cpp version —
// Download() checks the installed version against what this build expects
// and only fetches on a mismatch, so this is a no-op once versions line up.
func initKronk(ctx context.Context, log *slog.Logger) error {
	if kronk.Initialized() {
		return nil
	}

	appLog := slogAppLoggerFunc(log)

	l, err := libs.New()
	if err != nil {
		return fmt.Errorf("detecting runtime libraries: %w", err)
	}
	if _, err := l.Download(ctx, appLog); err != nil {
		return fmt.Errorf("downloading runtime libraries: %w", err)
	}

	return kronk.Init(kronk.WithLibPath(l.LibsPath()), kronk.WithLogLevel(kronk.LogNormal))
}

func slogAppLoggerFunc(log *slog.Logger) func(ctx context.Context, msg string, args ...any) {
	return func(ctx context.Context, msg string, args ...any) {
		log.InfoContext(ctx, msg, args...)
	}
}

// mustSourceDir returns the directory containing this file, so static
// assets and templates can be found regardless of the working directory
// evmlweb is launched from.
func mustSourceDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}
