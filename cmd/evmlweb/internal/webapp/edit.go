package webapp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

// This file holds the text-level edits behind the no-DSL forms in the
// workspace. They operate on the .evml *source* (not the parsed model) so
// comments, blank lines and the expert's own formatting survive; every
// edit is followed by a full re-parse, and an edit that fails to parse or
// validate is rejected wholesale.

var identRe = regexp.MustCompile(`^[_a-zA-Z][\w_]*(\.[_a-zA-Z][\w_]*)*$`)

// ParseEntityKeyword maps a form value ("ui", "cmd", "screen", …) onto the
// canonical short keyword the parser accepts.
func ParseEntityKeyword(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ui", "scn", "screen":
		return "ui", true
	case "cmd", "command":
		return "cmd", true
	case "evt", "event":
		return "evt", true
	case "rmo", "readmodel", "read model":
		return "rmo", true
	case "pcr", "processor", "automation":
		return "pcr", true
	}
	return "", false
}

// NextFrameID returns the next free numeric frame id, zero-padded to two
// digits (three once the model passes 99 frames).
func NextFrameID(m *evml.Model) string {
	maxID := 0
	for _, f := range m.Frames {
		if n, err := strconv.Atoi(f.ID); err == nil && n > maxID {
			maxID = n
		}
	}
	next := maxID + 1
	if next >= 100 {
		return strconv.Itoa(next)
	}
	return fmt.Sprintf("%02d", next)
}

// NormalizeIdentifier turns free text ("approve loan offer") into a
// PascalCase identifier ("ApproveLoanOffer"); dotted namespaces are kept.
func NormalizeIdentifier(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("a name is required")
	}
	if identRe.MatchString(raw) {
		return raw, nil
	}
	parts := strings.Split(raw, ".")
	for i, part := range parts {
		var b strings.Builder
		upper := true
		for _, r := range part {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
				if upper && r >= 'a' && r <= 'z' {
					r -= 'a' - 'A'
				}
				b.WriteRune(r)
				upper = false
			default:
				upper = true
			}
		}
		parts[i] = b.String()
	}
	out := strings.Join(parts, ".")
	if !identRe.MatchString(out) {
		return "", fmt.Errorf("%q cannot be turned into a valid name (letters, digits and _ only)", raw)
	}
	return out, nil
}

// normalizePayload ensures a payload is either empty or a single-line
// balanced { … } block.
func normalizePayload(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	raw = strings.Join(strings.Fields(raw), " ")
	if !strings.HasPrefix(raw, "{") {
		raw = "{ " + raw + " }"
	}
	depth := 0
	for _, r := range raw {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return "", fmt.Errorf("payload has an unmatched closing brace")
			}
		}
	}
	if depth != 0 {
		return "", fmt.Errorf("payload braces are not balanced")
	}
	return raw, nil
}

// StepInput is everything the "Add step" form collects.
type StepInput struct {
	Type    string // ui|cmd|evt|rmo|pcr (or a long alias)
	Name    string
	Payload string
	Actor   string
	Stage   string // current|staging|future; "" → staging
	Sources []string
	Reset   bool
}

// FormatFrameLine renders one tf/rf declaration line.
func FormatFrameLine(id string, in StepInput) (string, error) {
	kw, ok := ParseEntityKeyword(in.Type)
	if !ok {
		return "", fmt.Errorf("unknown step type %q (want screen, command, event, read model or automation)", in.Type)
	}
	name, err := NormalizeIdentifier(in.Name)
	if err != nil {
		return "", err
	}
	payload, err := normalizePayload(in.Payload)
	if err != nil {
		return "", err
	}
	stage := evml.StageStaging
	if strings.TrimSpace(in.Stage) != "" {
		st, ok := evml.ParseStage(in.Stage)
		if !ok {
			return "", fmt.Errorf("unknown stage %q", in.Stage)
		}
		stage = st
	}
	decl := "tf"
	if in.Reset {
		decl = "rf"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s %s", decl, id, kw, name)
	if a := strings.TrimSpace(in.Actor); a != "" {
		actor, err := NormalizeIdentifier(a)
		if err != nil {
			return "", fmt.Errorf("actor: %w", err)
		}
		b.WriteString(" @" + strings.ToUpper(actor[:1]) + actor[1:])
	}
	for _, src := range in.Sources {
		if s := strings.TrimSpace(src); s != "" {
			b.WriteString(" ->> " + s)
		}
	}
	if payload != "" {
		b.WriteString(" " + payload)
	}
	if stage != evml.StageCurrent {
		b.WriteString(" #" + string(stage))
	}
	return b.String(), nil
}

// InsertAfterFrame inserts block (one or more lines, no trailing newline
// needed) immediately after the declaration of frame afterID. When afterID
// is "" the block goes after the last frame declaration, or at the end of
// the file if there are none.
func InsertAfterFrame(src string, m *evml.Model, afterID, block string) (string, error) {
	lines := splitLines(src)
	insertAt := len(lines)
	if afterID != "" {
		f := m.FrameByID(afterID)
		if f == nil {
			return "", fmt.Errorf("unknown step %q", afterID)
		}
		insertAt = f.Line - 1 + f.LineCount
	} else if len(m.Frames) > 0 {
		last := m.Frames[0]
		for _, f := range m.Frames {
			if f.Line > last.Line {
				last = f
			}
		}
		insertAt = last.Line - 1 + last.LineCount
	}
	if insertAt > len(lines) {
		insertAt = len(lines)
	}
	out := make([]string, 0, len(lines)+2)
	out = append(out, lines[:insertAt]...)
	out = append(out, splitLines(block)...)
	out = append(out, lines[insertAt:]...)
	return joinLines(out), nil
}

// AppendBlock appends a top-level block (gwt, hotspot, slice, …) to the
// end of the source, separated by a blank line.
func AppendBlock(src, block string) string {
	src = strings.TrimRight(src, "\n")
	return src + "\n\n" + strings.TrimRight(block, "\n") + "\n"
}

// RemoveLines deletes lineCount lines starting at 1-based line.
func RemoveLines(src string, line, lineCount int) (string, error) {
	lines := splitLines(src)
	start := line - 1
	end := start + lineCount
	if start < 0 || end > len(lines) || lineCount <= 0 {
		return "", fmt.Errorf("line range %d+%d is outside the file", line, lineCount)
	}
	// Also drop one adjacent blank line so removals don't leave gaps.
	if end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	} else if start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		start--
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, lines[end:]...)
	return joinLines(out), nil
}

// ReplaceLines swaps lineCount lines starting at 1-based line for block.
func ReplaceLines(src string, line, lineCount int, block string) (string, error) {
	lines := splitLines(src)
	start := line - 1
	end := start + lineCount
	if start < 0 || end > len(lines) || lineCount <= 0 {
		return "", fmt.Errorf("line range %d+%d is outside the file", line, lineCount)
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, splitLines(block)...)
	out = append(out, lines[end:]...)
	return joinLines(out), nil
}

// FormatHotspot renders a hotspot block for frameID.
func FormatHotspot(frameID, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("the question text is required")
	}
	if strings.ContainsAny(text, "{}") {
		return "", fmt.Errorf("question text cannot contain braces")
	}
	return fmt.Sprintf("hotspot %s {\n  %s\n}", frameID, strings.ReplaceAll(text, "\n", "\n  ")), nil
}

// FormatNote renders a note block for frameID.
func FormatNote(frameID, text string) string {
	return fmt.Sprintf("note %s {\n  %s\n}", frameID, strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n  "))
}

// ScenarioInput is the "Add scenario" form: one statement per line, each
// "<type> <Name> [payload]".
type ScenarioInput struct {
	FrameID string
	Label   string
	Given   string
	When    string
	Then    string
}

// FormatScenario renders a gwt block, normalising each statement line.
func FormatScenario(in ScenarioInput) (string, error) {
	if strings.TrimSpace(in.FrameID) == "" {
		return "", fmt.Errorf("pick the step this scenario belongs to")
	}
	given, err := formatStatements(in.Given, "given")
	if err != nil {
		return "", err
	}
	when, err := formatStatements(in.When, "when")
	if err != nil {
		return "", err
	}
	then, err := formatStatements(in.Then, "then")
	if err != nil {
		return "", err
	}
	if len(given) == 0 || len(then) == 0 {
		return "", fmt.Errorf("a scenario needs at least one Given and one Then line")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "gwt %s", in.FrameID)
	if l := strings.TrimSpace(in.Label); l != "" {
		fmt.Fprintf(&b, " %q", strings.ReplaceAll(l, `"`, "'"))
	}
	b.WriteString("\n  given\n")
	for _, s := range given {
		b.WriteString("    " + s + "\n")
	}
	if len(when) > 0 {
		b.WriteString("  when\n")
		for _, s := range when {
			b.WriteString("    " + s + "\n")
		}
	}
	b.WriteString("  then\n")
	for _, s := range then {
		b.WriteString("    " + s + "\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func formatStatements(raw, section string) ([]string, error) {
	var out []string
	for _, line := range splitLines(raw) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s line %q needs a type and a name, e.g. \"event PaymentSettled { amount: 10 }\"", section, line)
		}
		kw, ok := ParseEntityKeyword(fields[0])
		if !ok {
			return nil, fmt.Errorf("%s line %q: unknown type %q (use event, command, read model, screen or automation)", section, line, fields[0])
		}
		name, err := NormalizeIdentifier(fields[1])
		if err != nil {
			return nil, fmt.Errorf("%s line %q: %w", section, line, err)
		}
		payload, err := normalizePayload(strings.Join(fields[2:], " "))
		if err != nil {
			return nil, fmt.Errorf("%s line %q: %w", section, line, err)
		}
		stmt := kw + " " + name
		if payload != "" {
			stmt += " " + payload
		}
		out = append(out, stmt)
	}
	return out, nil
}

// SliceInput is the "Mark slice" form.
type SliceInput struct {
	Name   string
	Start  string
	End    string
	Status string
	Stage  string
}

// FormatSlice renders a slice declaration line.
func FormatSlice(in SliceInput) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", fmt.Errorf("a slice name is required")
	}
	if in.Start == "" || in.End == "" {
		return "", fmt.Errorf("pick the first and last step of the slice")
	}
	line := fmt.Sprintf("slice %q %s-%s", strings.ReplaceAll(name, `"`, "'"), in.Start, in.End)
	if s := strings.TrimSpace(in.Status); s != "" {
		status, ok := evml.ParseSliceStatus(s)
		if !ok {
			return "", fmt.Errorf("unknown status %q", s)
		}
		line += " status " + status
	}
	if s := strings.TrimSpace(in.Stage); s != "" {
		stage, ok := evml.ParseStage(s)
		if !ok {
			return "", fmt.Errorf("unknown stage %q", s)
		}
		if stage != evml.StageCurrent {
			line += " stage " + string(stage)
		}
	}
	return line, nil
}

// SetFrameStage rewrites frame f's first declaration line so its explicit
// stage tag becomes stage. StageCurrent is written as an explicit #current
// only when the frame would otherwise inherit a different stage from a
// slice, so promoted frames read cleanly.
func SetFrameStage(src string, m *evml.Model, f *evml.Frame, stage evml.Stage) (string, error) {
	lines := splitLines(src)
	if f.Line < 1 || f.Line > len(lines) {
		return "", fmt.Errorf("frame %s has no source position", f.ID)
	}
	line := stripStageTag(lines[f.Line-1])
	inherited := evml.StageCurrent
	for _, s := range m.Slices {
		if s.Stage != "" && s.Contains(f) {
			inherited = s.Stage
		}
	}
	if stage != inherited {
		line += " #" + string(stage)
	}
	lines[f.Line-1] = line
	return joinLines(lines), nil
}

var stageTagRe = regexp.MustCompile(`\s+#[A-Za-z-]+\s*$`)

func stripStageTag(line string) string {
	// Only strip a trailing tag that sits outside any payload braces.
	trimmed := strings.TrimRight(line, " \t")
	if strings.HasSuffix(trimmed, "}") {
		return trimmed
	}
	return strings.TrimRight(stageTagRe.ReplaceAllString(trimmed, ""), " \t")
}

// RemoveFrame deletes a frame declaration together with every note,
// hotspot and scenario anchored to it, drops ->> references to it from
// other frames, and shrinks (or removes) chapters and slices whose range
// starts or ends on it, so the result still parses.
func RemoveFrame(src string, m *evml.Model, f *evml.Frame) (string, error) {
	lines := splitLines(src)
	if f.Line < 1 || f.Line-1+f.LineCount > len(lines) {
		return "", fmt.Errorf("frame %s has no source position", f.ID)
	}
	remove := map[int]bool{} // 0-based line indexes to drop
	mark := func(line, count int) {
		for i := line - 1; i < line-1+count && i < len(lines); i++ {
			if i >= 0 {
				remove[i] = true
			}
		}
	}
	mark(f.Line, f.LineCount)
	for _, n := range m.NoteEntities {
		if n.SourceID == f.ID {
			mark(n.Line, n.LineCount)
		}
	}
	for _, h := range m.Hotspots {
		if h.SourceID == f.ID {
			mark(h.Line, h.LineCount)
		}
	}
	for _, g := range m.GWTs {
		if g.SourceID == f.ID {
			mark(g.Line, g.LineCount)
		}
	}

	// Neighbours in declaration order, for shrinking ranges.
	var prev, next *evml.Frame
	for _, o := range m.Frames {
		if o.DeclarationIx == f.DeclarationIx-1 {
			prev = o
		}
		if o.DeclarationIx == f.DeclarationIx+1 {
			next = o
		}
	}
	fixRange := func(line int, start, end *evml.Frame, startID, endID string) {
		if line < 1 || line > len(lines) || start == nil || end == nil {
			return
		}
		if start == f && end == f {
			remove[line-1] = true
			return
		}
		newStart, newEnd := startID, endID
		if start == f && next != nil {
			newStart = next.ID
		}
		if end == f && prev != nil {
			newEnd = prev.ID
		}
		if start == f && next == nil || end == f && prev == nil {
			remove[line-1] = true
			return
		}
		lines[line-1] = strings.Replace(lines[line-1], startID+"-"+endID, newStart+"-"+newEnd, 1)
	}
	for _, ch := range m.Chapters {
		fixRange(ch.Line, ch.Start, ch.End, ch.StartID, ch.EndID)
	}
	for _, sl := range m.Slices {
		fixRange(sl.Line, sl.Start, sl.End, sl.StartID, sl.EndID)
	}

	// Drop "->> <id>" references from other frames' first lines.
	for _, o := range m.Frames {
		if o == f || o.Line < 1 || o.Line > len(lines) {
			continue
		}
		for _, id := range o.SourceIDs {
			if id == f.ID {
				lines[o.Line-1] = strings.Join(strings.Fields(strings.Replace(lines[o.Line-1], "->> "+id, "", 1)), " ")
			}
		}
	}

	out := make([]string, 0, len(lines))
	for i, l := range lines {
		if remove[i] {
			continue
		}
		// Collapse runs of blank lines left behind by removed blocks.
		if strings.TrimSpace(l) == "" && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		out = append(out, l)
	}
	return joinLines(out), nil
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}
