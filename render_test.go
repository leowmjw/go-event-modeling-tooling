package evml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderFixturesToSVG(t *testing.T) {
	files, err := filepath.Glob("testdata/fixtures/*.evml")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			model, err := Parse(string(content))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			svg, err := RenderSVG(model, RenderOptions{
				MeasureTextWidth: func(text string, monospace bool) float64 {
					if monospace {
						return float64(len(text)) * 7
					}
					return float64(len(text)) * 8
				},
			})
			if err != nil {
				t.Fatalf("RenderSVG() error = %v", err)
			}
			if !strings.HasPrefix(svg, `<svg`) || !strings.Contains(svg, `</svg>`) {
				t.Fatalf("unexpected svg output: %s", svg)
			}
			if !strings.Contains(svg, "swimlane") || !strings.Contains(svg, "box") {
				t.Fatalf("svg missing expected structure: %s", svg)
			}
		})
	}
}

func TestRenderGWTScenarios(t *testing.T) {
	content, err := os.ReadFile("testdata/fixtures/gwt-scenarios.evml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	model, err := Parse(string(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	svg, err := RenderSVG(model, RenderOptions{})
	if err != nil {
		t.Fatalf("RenderSVG() error = %v", err)
	}
	for _, want := range []string{"happy path", "duplicate add increments qty", "audit", "Given", "When", "Then"} {
		if !strings.Contains(svg, want) {
			t.Fatalf("svg missing %q", want)
		}
	}
}

func TestRenderUsesReferencedDataBlockContent(t *testing.T) {
	content, err := os.ReadFile("testdata/fixtures/simple-block.evml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	model, err := Parse(string(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	svg, err := RenderSVG(model, RenderOptions{})
	if err != nil {
		t.Fatalf("RenderSVG() error = %v", err)
	}
	for _, want := range []string{"AddItem", "description: &#39;john&#39;", "image: &#39;avatar_john&#39;", "price: 20.4"} {
		if !strings.Contains(svg, want) {
			t.Fatalf("svg missing %q", want)
		}
	}
	if strings.Contains(svg, "&lt;pre") {
		t.Fatalf("expected rendered data block to omit outer braces: %s", svg)
	}
}

func TestRenderInteractiveFrames(t *testing.T) {
	model, err := Parse(`eventmodeling
tf 01 cmd AddItem { sku: "x" }
tf 02 evt ItemAdded ->> 01
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	svg, err := RenderSVG(model, RenderOptions{
		Interactive:    true,
		FrameClickExpr: `@post('/flow/x/draft/y/frame/select', {payload:{frame:'{id}'}})`,
	})
	if err != nil {
		t.Fatalf("RenderSVG() error = %v", err)
	}
	for _, want := range []string{
		`id="frame-01"`,
		`data-frame-id="01"`,
		`class="box evml-frame"`,
		`data-on:click="@post(&#39;/flow/x/draft/y/frame/select&#39;, {payload:{frame:&#39;01&#39;}})"`,
	} {
		if !strings.Contains(svg, want) {
			t.Fatalf("svg missing %q", want)
		}
	}
}

func TestRenderHighlightOverridesBoxColors(t *testing.T) {
	model, err := Parse(`eventmodeling
tf 01 cmd AddItem
tf 02 evt ItemAdded ->> 01
tf 03 rmo CartView ->> 02
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	svg, err := RenderSVG(model, RenderOptions{
		Highlight: map[string]string{
			"01": "added",
			"02": "changed",
			"03": "selected",
		},
	})
	if err != nil {
		t.Fatalf("RenderSVG() error = %v", err)
	}
	if !strings.Contains(svg, `fill="#e6f4e6" stroke="#2e7d32"`) {
		t.Fatalf("svg missing added highlight: %s", svg)
	}
	if !strings.Contains(svg, `fill="#fff3d6" stroke="#b8860b"`) {
		t.Fatalf("svg missing changed highlight: %s", svg)
	}
	if !strings.Contains(svg, `stroke="#1565c0" stroke-width="3"`) {
		t.Fatalf("svg missing selected highlight: %s", svg)
	}
}

func TestValidateAllFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/fixtures/*.evml")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			model, err := Parse(string(content))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if errs := ValidateConnections(model); len(errs) > 0 {
				t.Fatalf("ValidateConnections() = %v, want no errors", errs)
			}
		})
	}
}
