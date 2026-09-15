package webapp

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ardanlabs/kronk/sdk/tools/models"
	evml "github.com/leowmjw/go-event-modeling-tooling"
)

// moduleRoot returns the absolute path of the package source directory
// — used so tests can locate templates and static assets regardless of
// the working directory `go test` happens to be invoked from.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func TestNewEndpointsReachable(t *testing.T) {
	root := moduleRoot(t)
	repoRoot, err := filepath.Abs(filepath.Join(root, "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repoRoot: %v", err)
	}
	tplDir := filepath.Join(root, "templates")
	staticDir, err := filepath.Abs(filepath.Join(root, "..", "..", "static"))
	if err != nil {
		t.Fatalf("resolve staticDir: %v", err)
	}

	app, err := NewApp(Config{
		Addr:        ":0",
		RepoRoot:    repoRoot,
		StateDir:    t.TempDir(),
		StaticDir:   staticDir,
		TemplateDir: tplDir,
	}, slog.Default(), &models.Models{})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want 200", resp.StatusCode)
	}

	// Confirm the new 2026-09 endpoints are wired. Each call hits a
	// brand-new session, so the handler responds with 404 (no such
	// flow/draft) — which proves the mux dispatched to the handler
	// rather than 404-ing on the pattern itself.
	newEndpoints := []struct {
		method, path string
	}{
		{"POST", "/flow/card-payment-lifecycle/draft/foo/rename"},
		{"POST", "/flow/card-payment-lifecycle/draft/foo/duplicate"},
		{"POST", "/flow/card-payment-lifecycle/draft/foo/frame/01"},
		{"POST", "/flow/card-payment-lifecycle/draft/foo/frame/01/delete"},
		{"POST", "/flow/card-payment-lifecycle/draft/foo/hotspot"},
		{"POST", "/flow/card-payment-lifecycle/draft/foo/hotspot/01/resolve"},
	}
	for _, e := range newEndpoints {
		var resp2 *http.Response
		var err2 error
		if e.method == "POST" {
			resp2, err2 = http.Post(srv.URL+e.path, "application/json", strings.NewReader(`{"frameId":"01","name":"X","entityType":"cmd","label":"x","body":"q?"}`))
		} else {
			resp2, err2 = http.Get(srv.URL + e.path)
		}
		if err2 != nil {
			t.Errorf("%s %s: %v", e.method, e.path, err2)
			continue
		}
		resp2.Body.Close()
		// The handler runs and returns 404 because the session has no
		// such flow/draft. The point of this test is that the route is
		// wired (i.e. the mux dispatched to our handler, not to the
		// mux's default 404). To distinguish, hit the same path with
		// the wrong method — a real wired route returns 405, an
		// unwired one returns 404.
		wrong := ""
		if e.method == "POST" {
			wrong = "GET"
		} else {
			wrong = "POST"
		}
		var probe *http.Response
		var errProbe error
		if wrong == "POST" {
			probe, errProbe = http.Post(srv.URL+e.path, "application/json", strings.NewReader(`{}`))
		} else {
			probe, errProbe = http.Get(srv.URL + e.path)
		}
		if errProbe != nil {
			t.Errorf("%s probe %s: %v", wrong, e.path, errProbe)
			continue
		}
		probe.Body.Close()
		if probe.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: route not registered (got %d, want 405)", wrong, e.path, probe.StatusCode)
		}
	}
}

func TestUpdateFrameLineAndDeleteSanity(t *testing.T) {
	src := `eventmodeling
tf 01 ui Screen
tf 02 cmd Submit { id: "x" }
tf 03 evt Done ->> 02
`
	out := updateFrameLine(src, "01", frameEditRequest{Name: "CheckoutScreen"})
	if _, err := evml.Parse(out); err != nil {
		t.Errorf("renamed source doesn't parse: %v\n%s", err, out)
	}

	out2 := deleteFrameLine(src, "02")
	if _, err := evml.Parse(out2); err != nil {
		t.Errorf("deleted source doesn't parse: %v\n%s", err, out2)
	}
}
