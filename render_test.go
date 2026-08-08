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
