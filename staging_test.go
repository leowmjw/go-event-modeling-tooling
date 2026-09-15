package evml

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func loadStagingFixture(t *testing.T) *Model {
	t.Helper()
	content, err := os.ReadFile("testdata/fixtures/staging-lens-hotspots.evml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m, err := Parse(string(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if errs := Validate(m); len(errs) > 0 {
		t.Fatalf("Validate: %v", errs)
	}
	return m
}

func TestParseWorkshopNotation(t *testing.T) {
	m := loadStagingFixture(t)
	if got := len(m.Actors); got != 2 {
		t.Fatalf("actors = %d, want 2", got)
	}
	if got := len(m.Chapters); got != 3 {
		t.Fatalf("chapters = %d, want 3", got)
	}
	if got := len(m.Slices); got != 4 {
		t.Fatalf("slices = %d, want 4", got)
	}
	if got := len(m.Hotspots); got != 2 {
		t.Fatalf("hotspots = %d, want 2", got)
	}
	if f := m.FrameByID("01"); f == nil || f.Actor != "Customer" {
		t.Fatalf("frame 01 actor = %v", f)
	}
	if f := m.FrameByID("12"); f == nil || f.StageTag != StageFuture {
		t.Fatalf("frame 12 stage tag = %v", f)
	}
	sl := m.Slices[1]
	if sl.Name != "Velocity check" || sl.Status != StatusInProgress || sl.Stage != StageStaging {
		t.Fatalf("slice 1 = %+v", sl)
	}
	if f := m.FrameByID("02"); f.Line != 21 || f.LineCount != 1 {
		t.Fatalf("frame 02 line = %d/%d, want 21/1", f.Line, f.LineCount)
	}
}

func TestFrameStageResolution(t *testing.T) {
	m := loadStagingFixture(t)
	cases := map[string]Stage{
		"01": StageCurrent, // slice without stage
		"05": StageStaging, // inherited from slice
		"09": StageStaging,
		"10": StageFuture, // inherited from slice
		"12": StageFuture, // explicit tag
	}
	for id, want := range cases {
		if got := m.FrameStage(m.FrameByID(id)); got != want {
			t.Errorf("frame %s stage = %s, want %s", id, got, want)
		}
	}
}

func TestFilterStages(t *testing.T) {
	m := loadStagingFixture(t)
	current := FilterStages(m, StageCurrent)
	if got := len(current.Frames); got != 4 {
		t.Fatalf("current-only frames = %d, want 4", got)
	}
	if len(current.Hotspots) != 0 {
		t.Fatalf("current-only hotspots = %d, want 0", len(current.Hotspots))
	}
	if len(current.Chapters) != 1 || current.Chapters[0].Name != "Top-up" {
		t.Fatalf("current-only chapters = %+v", current.Chapters)
	}
	withStaging := FilterStages(m, StageCurrent, StageStaging)
	if got := len(withStaging.Frames); got != 9 {
		t.Fatalf("current+staging frames = %d, want 9", got)
	}
	if got := len(withStaging.GWTs); got != 3 {
		t.Fatalf("current+staging gwts = %d, want 3", got)
	}
	// Processor 05 sources 03 explicitly; both survive so the edge must too.
	f := withStaging.FrameByID("05")
	if len(f.Sources) != 1 || f.Sources[0].ID != "03" {
		t.Fatalf("frame 05 sources after filter = %+v", f.SourceIDs)
	}
	all := FilterStages(m, StageCurrent, StageStaging, StageFuture)
	if all != m {
		t.Fatal("filtering with all stages should return the same model")
	}
}

func TestRenderWorkshopNotation(t *testing.T) {
	m := loadStagingFixture(t)
	svg, err := RenderSVG(m, RenderOptions{})
	if err != nil {
		t.Fatalf("RenderSVG: %v", err)
	}
	for _, want := range []string{
		`class="chapter"`, `Risk controls`,
		`class="slice slice-stage-staging"`, `Velocity check  [InProgress · staging]`,
		`class="box box-pcr box-stage-staging"`, `data-frame="05"`,
		`class="box box-evt box-stage-future"`,
		`class="hotspot"`, `Open question · FlagUnusualTopUp`,
		`class="actor"`, `@Customer`,
		`class="hotspot-dot"`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q", want)
		}
	}
}

func TestLintReportsHotspotsAndGaps(t *testing.T) {
	m := loadStagingFixture(t)
	findings := Lint(m)
	kinds := map[string]int{}
	for _, f := range findings {
		kinds[f.Kind]++
	}
	if kinds["hotspot"] != 2 {
		t.Fatalf("hotspot findings = %d, want 2; all: %v", kinds["hotspot"], findings)
	}
	// Command 11 is future-stage: no scenario required yet. Commands 02 and
	// 06 both have scenarios, so no uncovered-command finding is expected.
	if kinds["uncovered-command"] != 0 {
		t.Fatalf("uncovered-command findings = %d, want 0; all: %v", kinds["uncovered-command"], findings)
	}
}

func TestValidateRanges(t *testing.T) {
	src := `eventmodeling
chapter "A" 01-03
chapter "B" 02-04
slice "S" 03-04
tf 01 ui X
tf 02 cmd Y
tf 03 evt Z
tf 04 rmo W
`
	m, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	errs := ValidateRanges(m)
	if len(errs) != 2 {
		t.Fatalf("errs = %v, want overlap + boundary crossing", errs)
	}
	if !strings.Contains(errs[0].Error(), "overlap") {
		t.Fatalf("first error = %v", errs[0])
	}
	if !strings.Contains(errs[1].Error(), "crosses") {
		t.Fatalf("second error = %v", errs[1])
	}
}

func TestParseErrorsForWorkshopNotation(t *testing.T) {
	cases := []struct{ src, want string }{
		{"eventmodeling\ntf 01 ui X #tomorrow\n", "unknown stage"},
		{"eventmodeling\nslice Foo 01-02\ntf 01 ui X\ntf 02 cmd Y\n", "quoted name"},
		{"eventmodeling\nslice \"Foo\" 01-02 status Nope\ntf 01 ui X\ntf 02 cmd Y\n", "unknown status"},
		{"eventmodeling\nchapter \"Foo\" 01-09\ntf 01 ui X\n", "unknown end frame"},
		{"eventmodeling\nhotspot 07 { why? }\ntf 01 ui X\n", "unknown hotspot source"},
	}
	for _, c := range cases {
		_, err := Parse(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Parse(%q) error = %v, want containing %q", c.src, err, c.want)
		}
	}
	_, err := Parse("eventmodeling\ntf 01 ui X #tomorrow\n")
	if pe, ok := errors.AsType[*ParseError](err); !ok || pe.Line != 2 {
		t.Fatalf("expected ParseError on line 2, got %v", err)
	}
}

func TestProseBlocksTolerateApostrophes(t *testing.T) {
	src := `eventmodeling
tf 01 ui X
tf 02 cmd Y
hotspot 02 {
  Does the bank's clock pause while we wait for the merchant's evidence?
}
note 01 {
  Collections' view isn't the same as Finance's.
}
tf 03 evt Z { note: "it's fine", other: 'a } b' }
`
	m, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Hotspots) != 1 || !strings.Contains(m.Hotspots[0].Value, "merchant's evidence") {
		t.Fatalf("hotspot = %+v", m.Hotspots)
	}
	if len(m.NoteEntities) != 1 || m.NoteEntities[0].LineCount != 3 {
		t.Fatalf("note = %+v", m.NoteEntities)
	}
	// Quote-aware parsing still wins for well-formed payloads.
	if f := m.FrameByID("03"); f == nil || f.Data != `{ note: "it's fine", other: 'a } b' }` {
		t.Fatalf("frame 03 payload = %q", f.Data)
	}
}

func TestDiffVersions(t *testing.T) {
	before, _ := Parse(`eventmodeling
tf 01 ui Screen
tf 02 cmd DoThing { a: 1 }
tf 03 evt ThingDone
tf 04 rmo Old
hotspot 02 { open? }
gwt 02 "happy"
  given
    evt X
  then
    evt ThingDone
`)
	after, _ := Parse(`eventmodeling
slice "Thing" 01-03 stage staging
tf 01 ui Screen
tf 02 cmd DoThing { a: 2 }
tf 03 evt ThingDone
tf 05 pcr Bot ->> 03
gwt 02 "happy"
  given
    evt X
  then
    evt ThingDone
gwt 02 "sad"
  given
    evt X
  then
    evt ThingRejected
`)
	d := Diff(before, after)
	if len(d.Added) != 1 || d.Added[0].ID != "05" {
		t.Fatalf("Added = %+v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0].ID != "04" {
		t.Fatalf("Removed = %+v", d.Removed)
	}
	if len(d.Changed) != 1 || d.Changed[0].ID != "02" {
		t.Fatalf("Changed = %+v", d.Changed)
	}
	if d.ScenariosAdded != 1 || d.HotspotsResolved != 1 {
		t.Fatalf("scenariosAdded=%d hotspotsResolved=%d", d.ScenariosAdded, d.HotspotsResolved)
	}
	if len(d.StageChanges) != 3 {
		t.Fatalf("StageChanges = %v, want 3 (frames 01-03 → staging)", d.StageChanges)
	}
	if len(d.SlicesAdded) != 1 {
		t.Fatalf("SlicesAdded = %v", d.SlicesAdded)
	}
	if Diff(before, before).Empty() != true {
		t.Fatal("self-diff should be empty")
	}
}

func TestCLILintAndDiff(t *testing.T) {
	var out, errOut strings.Builder
	code := Run([]string{"lint", "testdata/fixtures/staging-lens-hotspots.evml"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("lint exit = %d, stderr = %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "2 open question(s)") {
		t.Fatalf("lint output = %s", out.String())
	}
	out.Reset()
	code = Run([]string{"lint", "--strict", "testdata/fixtures/staging-lens-hotspots.evml"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("strict lint exit = %d, want 1", code)
	}
	out.Reset()
	code = Run([]string{"diff", "testdata/fixtures/simple-block.evml", "testdata/fixtures/staging-lens-hotspots.evml"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "+ 05 pcr VelocityMonitor") {
		t.Fatalf("diff exit = %d, out = %s", code, out.String())
	}
}
