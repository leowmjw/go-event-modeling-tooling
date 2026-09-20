package evml

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// bandSpans returns the [top, bottom) y-range of every annotation box that
// carries the given CSS class, in render order.
func bandSpans(t *testing.T, svg, class string) [][2]float64 {
	t.Helper()
	re := regexp.MustCompile(`class="` + class + `"><rect x="[\d.]+" y="([\d.]+)" width="\d+" height="([\d.]+)"`)
	var spans [][2]float64
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		top, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatalf("parse y %q: %v", m[1], err)
		}
		h, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			t.Fatalf("parse height %q: %v", m[2], err)
		}
		spans = append(spans, [2]float64{top, top + h})
	}
	return spans
}

func TestRenderStacksAnnotationBandsWithoutOverlap(t *testing.T) {
	// Notes, GWT scenarios and hotspots each occupy their own band below the
	// swimlanes. Regression: hotspots used to be laid out relative to the GWT
	// band only, so a model with notes + hotspots but no GWTs rendered the
	// hotspot box flush against the last note.
	cases := []struct {
		name   string
		source string
		bands  []string
	}{
		{
			name: "notes and hotspots without gwts",
			source: `eventmodeling
tf 01 cmd BookRoom
note 01 { rate locked at search time }
hotspot 01 { concurrent booking: last write wins, or reject? }
`,
			bands: []string{"note", "hotspot"},
		},
		{
			name: "hotspots only",
			source: `eventmodeling
tf 01 cmd BookRoom
hotspot 01 { concurrent booking: last write wins, or reject? }
`,
			bands: []string{"hotspot"},
		},
		{
			name: "notes, gwts and hotspots",
			source: `eventmodeling
tf 01 evt RoomListed
tf 02 cmd BookRoom
note 02 { rate locked at search time }
hotspot 02 { concurrent booking: last write wins, or reject? }

gwt 02 "book a listed room"
  given
    evt RoomListed
  then
    evt RoomListed
`,
			bands: []string{"note", "gwt", "hotspot"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := Parse(tc.source)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			svg, err := RenderSVG(model, RenderOptions{})
			if err != nil {
				t.Fatalf("RenderSVG() error = %v", err)
			}
			// Every band is separated from the one above it by at least the
			// designed 20px gutter. Asserting only "no overlap" is too weak:
			// the buggy layout still left the note stack's 12px trailing pad,
			// so the boxes did not overlap — they were merely too tight.
			const minGutter = 20.0
			prevBottom := 0.0
			prevClass := "swimlanes"
			for _, class := range tc.bands {
				spans := bandSpans(t, svg, class)
				if len(spans) == 0 {
					t.Fatalf("no %q boxes rendered", class)
				}
				if gap := spans[0][0] - prevBottom; gap < minGutter {
					t.Fatalf("gap between %s (ends %v) and %s (starts %v) = %v, want >= %v",
						prevClass, prevBottom, class, spans[0][0], gap, minGutter)
				}
				for _, span := range spans {
					if span[1] > prevBottom {
						prevBottom = span[1]
					}
				}
				prevClass = class
			}
			m := regexp.MustCompile(`<svg[^>]*height="([\d.]+)"`).FindStringSubmatch(svg)
			if m == nil {
				t.Fatal("svg has no height attribute")
			}
			height, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				t.Fatalf("parse svg height %q: %v", m[1], err)
			}
			if height < prevBottom {
				t.Fatalf("svg height %v clips the last band ending at %v", height, prevBottom)
			}
		})
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
