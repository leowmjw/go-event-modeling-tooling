package webapp

import (
	"strings"
	"testing"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

func TestUpdateFrameLineRename(t *testing.T) {
	src := "eventmodeling\ntf 01 ui OldName\ntf 02 cmd X\ntf 03 evt Y ->> 02\n"
	out := updateFrameLine(src, "01", frameEditRequest{Name: "NewName"})
	if !strings.Contains(out, "tf 01 ui NewName") {
		t.Errorf("rename failed; got:\n%s", out)
	}
	if !strings.Contains(out, "tf 02 cmd X") {
		t.Errorf("unrelated frame disturbed:\n%s", out)
	}
}

func TestUpdateFrameLineRetype(t *testing.T) {
	src := "eventmodeling\ntf 01 ui Screen { foo: 1 }\n"
	out := updateFrameLine(src, "01", frameEditRequest{EntityType: "cmd"})
	if !strings.Contains(out, "tf 01 cmd Screen") {
		t.Errorf("retype failed; got:\n%s", out)
	}
	if !strings.Contains(out, "{ foo: 1 }") {
		t.Errorf("payload tail dropped; got:\n%s", out)
	}
}

func TestUpdateFrameLineNoChange(t *testing.T) {
	src := "eventmodeling\ntf 01 ui X\n"
	out := updateFrameLine(src, "01", frameEditRequest{})
	if out != src {
		t.Errorf("empty edit should be a no-op; got change:\n%s", out)
	}
}

func TestDeleteFrameLine(t *testing.T) {
	src := "eventmodeling\ntf 01 ui X\ntf 02 cmd Y\ntf 03 evt Z ->> 02\n"
	out := deleteFrameLine(src, "02")
	if strings.Contains(out, "tf 02 cmd Y") {
		t.Errorf("frame 02 still present:\n%s", out)
	}
	if strings.Contains(out, "->> 02") {
		t.Errorf("frame 02 reference still present:\n%s", out)
	}
	if !strings.Contains(out, "tf 03 evt Z") {
		t.Errorf("unrelated frame 03 disturbed:\n%s", out)
	}
}

func TestResolveHotspot(t *testing.T) {
	src := "eventmodeling\ntf 01 ui X\nhotspot 01 { what is X? }\nhotspot 01 status open { another question }\n"
	out := resolveHotspot(src, "01")
	if !strings.Contains(out, "status resolved") {
		t.Errorf("expected status resolved in:\n%s", out)
	}
	// The second hotspot should be untouched on the second call.
	out2 := resolveHotspot(out, "01")
	if out2 != out {
		t.Errorf("second call should be a no-op after the first resolved")
	}
}

func TestSliceSummaryHelper(t *testing.T) {
	src := `eventmodeling
tf 01 ui X
slice "A" 01-01 status Done
slice "B" 01-01 status InProgress
slice "C" 01-01 status Planned
slice "D" 01-01 status Review
`
	got := sliceSummary(src)
	if got == "" {
		t.Fatalf("sliceSummary() = empty, want summary")
	}
	// Just check the numeric contents — order is fixed in the helper.
	for _, want := range []string{"1 done", "1 in-progress", "1 planned"} {
		if !strings.Contains(got, want) {
			t.Errorf("sliceSummary() = %q, missing %q", got, want)
		}
	}
}

func TestUnifiedDiffAddsAndRemoves(t *testing.T) {
	a := "eventmodeling\ntf 01 ui X\ntf 02 cmd Y\n"
	b := "eventmodeling\ntf 01 ui X\ntf 02 cmd Y2\n"
	diff := unifiedDiff(a, b, "a", "b")
	if !strings.Contains(diff, "- tf 02 cmd Y") {
		t.Errorf("diff missing removal line:\n%s", diff)
	}
	if !strings.Contains(diff, "+ tf 02 cmd Y2") {
		t.Errorf("diff missing addition line:\n%s", diff)
	}
}

func TestHotspotSnippet(t *testing.T) {
	body := `{
  First line here
  Second line
}`
	got := hotspotSnippet(body)
	if !strings.HasPrefix(got, "First line here") {
		t.Errorf("hotspotSnippet() = %q, want first-line prefix", got)
	}
}

func TestIsFrameID(t *testing.T) {
	cases := map[string]bool{
		"1": true, "01": true, "123": true,
		"": false, "abc": false, "1234": false, "a1": false,
	}
	for in, want := range cases {
		if got := isFrameID(in); got != want {
			t.Errorf("isFrameID(%q) = %v, want %v", in, got, want)
		}
	}
}

// Sanity check that the helper functions actually parse with the real
// evml library — guards against accidental type mismatches if the
// upstream model evolves.
func TestParseRealFixture(t *testing.T) {
	src := `eventmodeling
tf 01 ui X
hotspot 01 { q? }
chapter "Foo" 01-01
slice "S" 01-01 status Done
`
	m, err := evml.Parse(src)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(m.Hotspots) != 1 || len(m.Chapters) != 1 || len(m.Slices) != 1 {
		t.Fatalf("parse produced unexpected counts: %+v", m)
	}
}
