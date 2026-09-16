package evml

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunSVGWritesRequestedOutput(t *testing.T) {
	origReadFile, origWriteFile, origMakeDirAll := readFile, writeFile, makeDirAll
	defer func() {
		readFile, writeFile, makeDirAll = origReadFile, origWriteFile, origMakeDirAll
	}()

	var wrotePath string
	var wroteContent []byte
	readFile = func(name string) ([]byte, error) {
		return []byte("eventmodeling\ntf 01 cmd AddItem\n"), nil
	}
	writeFile = func(name string, data []byte, perm fs.FileMode) error {
		wrotePath = name
		wroteContent = append([]byte(nil), data...)
		return nil
	}
	makeDirAll = func(path string, perm fs.FileMode) error { return nil }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"svg", "model.evml", "-d", "out"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	if want := filepath.Join("out", "model.svg"); wrotePath != want {
		t.Fatalf("output path = %q, want %q", wrotePath, want)
	}
	if !strings.Contains(string(wroteContent), "<svg") {
		t.Fatalf("wrote content is not svg: %s", string(wroteContent))
	}
	if !strings.Contains(stdout.String(), "SVG generated successfully") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

type jsonDoc struct {
	Schema   string `json:"schema"`
	Source   string `json:"source"`
	SHA256   string `json:"sha256"`
	Sections []struct {
		Name string `json:"name"`
		Line int    `json:"line"`
	} `json:"sections"`
	Frames []struct {
		ID        string   `json:"id"`
		Kind      string   `json:"kind"`
		Type      string   `json:"type"`
		Name      string   `json:"name"`
		Namespace string   `json:"namespace"`
		Line      int      `json:"line"`
		Section   string   `json:"section"`
		Sources   []string `json:"sources"`
	} `json:"frames"`
	GWTs []struct {
		Frame string `json:"frame"`
	} `json:"gwts"`
	Hotspots []struct {
		Frame    string `json:"frame"`
		DataType string `json:"dataType"`
		Value    string `json:"value"`
		Line     int    `json:"line"`
	} `json:"hotspots"`
}

func TestRunJSONBoundedContextFixture(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"json", "testdata/fixtures/bounded-context-order-fulfillment.evml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	var doc jsonDoc
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc.Schema != "evml-model/v1" {
		t.Fatalf("schema = %q", doc.Schema)
	}
	if len(doc.Sections) != 4 {
		t.Fatalf("len(sections) = %d, want 4", len(doc.Sections))
	}
	if doc.Sections[0].Name != "Sales bounded context" {
		t.Fatalf("first section = %q", doc.Sections[0].Name)
	}
	frames := map[string]struct {
		Kind      string
		Namespace string
		Line      int
		Section   string
		Sources   []string
	}{}
	for _, f := range doc.Frames {
		frames[f.ID] = struct {
			Kind      string
			Namespace string
			Line      int
			Section   string
			Sources   []string
		}{f.Kind, f.Namespace, f.Line, f.Section, f.Sources}
	}
	f07, ok := frames["07"]
	if !ok {
		t.Fatal("frame 07 missing")
	}
	if f07.Section != "Billing bounded context" {
		t.Fatalf("frame 07 section = %q", f07.Section)
	}
	if f07.Line <= 0 {
		t.Fatalf("frame 07 line = %d", f07.Line)
	}
	if len(f07.Sources) != 1 || f07.Sources[0] != "06" {
		t.Fatalf("frame 07 sources = %v", f07.Sources)
	}
	f06, ok := frames["06"]
	if !ok {
		t.Fatal("frame 06 missing")
	}
	if f06.Kind != "rf" || f06.Namespace != "Sales" {
		t.Fatalf("frame 06 kind=%q namespace=%q", f06.Kind, f06.Namespace)
	}
	foundGWT := false
	for _, g := range doc.GWTs {
		if g.Frame == "03" {
			foundGWT = true
		}
	}
	if !foundGWT {
		t.Fatal("no gwt anchored to frame 03")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(doc.SHA256) {
		t.Fatalf("sha256 = %q", doc.SHA256)
	}
}

func TestRunJSONEmitsHotspots(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"json", "testdata/fixtures/hotel-booking-with-hotspots.evml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	var doc jsonDoc
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(doc.Hotspots) != 3 {
		t.Fatalf("len(hotspots) = %d, want 3", len(doc.Hotspots))
	}
	byFrame := map[string]int{}
	for _, h := range doc.Hotspots {
		byFrame[h.Frame]++
		if h.Line <= 0 {
			t.Fatalf("hotspot on frame %s has line = %d", h.Frame, h.Line)
		}
	}
	if byFrame["03"] != 2 || byFrame["05"] != 1 {
		t.Fatalf("hotspots by frame = %v, want {03: 2, 05: 1}", byFrame)
	}
	if !strings.Contains(doc.Hotspots[0].Value, "concurrent booking") {
		t.Fatalf("first hotspot value = %q", doc.Hotspots[0].Value)
	}
	if doc.Hotspots[1].DataType != "md" {
		t.Fatalf("second hotspot dataType = %q, want md", doc.Hotspots[1].DataType)
	}
}

func TestRunJSONEmitsEmptyHotspotsArray(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"json", "testdata/fixtures/bounded-context-order-fulfillment.evml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	// A model with no hotspots must still emit [] rather than null so the
	// provenance script can index it unconditionally.
	if !strings.Contains(stdout.String(), `"hotspots": []`) {
		t.Fatalf("expected an empty hotspots array in output")
	}
}

func TestRunJSONWritesRequestedOutput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "nested", "model.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"json", "testdata/fixtures/bounded-context-order-fulfillment.evml", "-o", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var doc jsonDoc
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if doc.Schema != "evml-model/v1" {
		t.Fatalf("schema = %q", doc.Schema)
	}
}

func TestRunSVGRejectsInvalidInput(t *testing.T) {
	origReadFile := readFile
	defer func() { readFile = origReadFile }()
	readFile = func(name string) ([]byte, error) {
		return []byte("eventmodeling\ntf 01 evt Start\ntf 02 cmd Bad ->> 01\n"), nil
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"svg", "model.evml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "invalid model") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
