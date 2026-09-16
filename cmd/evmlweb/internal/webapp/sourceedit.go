package webapp

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	evml "github.com/leowmjw/go-event-modeling-tooling"
)

// SourceEditor performs structural edits on a draft's .evml source text as
// line surgery, anchored on the declaration line numbers the evml parser
// records. It never reformats the whole document: untouched lines
// (comments, banner blocks, gwts, data blocks) are preserved verbatim, so
// manual edits and LLM rewrites keep converging on the same artifact —
// the source text itself.
//
// Frame declarations are always a single line (the parser only accepts
// single-line frame payloads), so frame edits locate exactly one line via
// Frame.Line and rewrite it from the parsed model; data blocks are
// rewritten as a line range.
type SourceEditor struct {
	lines []string
	model *evml.Model
}

// NewSourceEditor parses source and returns an editor positioned on it.
// It fails when source doesn't parse — callers should only edit drafts
// whose source is currently valid.
func NewSourceEditor(source string) (*SourceEditor, error) {
	model, err := evml.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("draft doesn't parse cleanly: %w", err)
	}
	return &SourceEditor{
		lines: strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n"),
		model: model,
	}, nil
}

// Source returns the current (possibly edited) source text.
func (e *SourceEditor) Source() string {
	return strings.Join(e.lines, "\n")
}

// Frame returns the frame with the given ID, or nil.
func (e *SourceEditor) Frame(id string) *evml.Frame {
	for _, f := range e.model.Frames {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// Frames returns every frame in timeline (declaration) order.
func (e *SourceEditor) Frames() []*evml.Frame {
	return e.model.Frames
}

// FrameRefs lists every other frame whose ->> sources include id.
func (e *SourceEditor) FrameRefs(id string) []string {
	var refs []string
	for _, f := range e.model.Frames {
		if f.ID == id {
			continue
		}
		for _, src := range f.SourceIDs {
			if src == id {
				refs = append(refs, f.ID)
				break
			}
		}
	}
	sort.Strings(refs)
	return refs
}

// AddFrame inserts a new frame after the frame with ID afterID (or appends
// at the end of the document when afterID is ""). The new frame sources
// from the anchor when that connection is legal per the wiring rules. The
// payload is the inner content (without outer braces) of a single-line
// inline payload; empty means no payload. Returns the new frame's ID.
func (e *SourceEditor) AddFrame(afterID string, entityType evml.EntityType, identifier, payload string) (string, error) {
	if identifier == "" {
		return "", fmt.Errorf("a name is required")
	}
	if strings.ContainsAny(identifier, " \t") {
		return "", fmt.Errorf("names can't contain spaces — use CamelCase or dots, e.g. Payments.PaymentSent")
	}
	if payload != "" && strings.Contains(payload, "\n") {
		return "", fmt.Errorf("frame payloads must fit on one line")
	}

	anchor := e.Frame(afterID)
	if afterID != "" && anchor == nil {
		return "", fmt.Errorf("no frame %s to insert after", afterID)
	}

	id := e.nextFrameID()
	var b strings.Builder
	fmt.Fprintf(&b, "tf %s %s %s", id, entityType, identifier)
	if anchor != nil && evml.CanConnect(anchor.EntityType, entityType) {
		fmt.Fprintf(&b, " ->> %s", anchor.ID)
	}
	if payload != "" {
		fmt.Fprintf(&b, " { %s }", payload)
	}

	insertAt := len(e.lines)
	if anchor != nil {
		insertAt = anchor.EndLine // 1-indexed inclusive == slice insert position
	}
	e.lines = append(e.lines[:insertAt], append([]string{b.String()}, e.lines[insertAt:]...)...)
	return id, nil
}

// RenameFrame changes the frame's identifier (its name in the diagram).
func (e *SourceEditor) RenameFrame(id, identifier string) error {
	if identifier == "" {
		return fmt.Errorf("a name is required")
	}
	if strings.ContainsAny(identifier, " \t") {
		return fmt.Errorf("names can't contain spaces — use CamelCase or dots, e.g. Payments.PaymentSent")
	}
	f := e.Frame(id)
	if f == nil {
		return fmt.Errorf("no frame %s", id)
	}
	f.Identifier = identifier
	e.rewriteFrameLine(f)
	return nil
}

// SetFramePayload replaces the frame's payload. payload is the inner
// content without outer braces; empty removes the payload. Frames with a
// [[data reference]] have their data block's value edited instead of an
// inline payload. Data-block payloads may span lines; inline payloads
// must stay on one line.
func (e *SourceEditor) SetFramePayload(id, payload string) error {
	f := e.Frame(id)
	if f == nil {
		return fmt.Errorf("no frame %s", id)
	}
	if payload != "" {
		if err := checkPayloadBalanced(payload); err != nil {
			return err
		}
	}
	if f.DataRefName != "" {
		return e.setDataBlockValue(f.DataRefName, payload)
	}
	if payload != "" && strings.Contains(payload, "\n") {
		return fmt.Errorf("frame payloads must fit on one line; data blocks can span lines")
	}
	f.Data = wrapPayload(payload)
	f.DataType = ""
	e.rewriteFrameLine(f)
	return nil
}

// SetFrameSources replaces the frame's ->> wiring with sourceIDs (given in
// declaration order; duplicates are dropped).
func (e *SourceEditor) SetFrameSources(id string, sourceIDs []string) error {
	f := e.Frame(id)
	if f == nil {
		return fmt.Errorf("no frame %s", id)
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(sourceIDs))
	for _, src := range sourceIDs {
		src = strings.TrimSpace(src)
		if src == "" || seen[src] {
			continue
		}
		seen[src] = true
		if e.Frame(src) == nil {
			return fmt.Errorf("no frame %s to connect from", src)
		}
		clean = append(clean, src)
	}
	f.SourceIDs = clean
	f.Sources = nil
	e.rewriteFrameLine(f)
	return nil
}

// DeleteFrame removes the frame's declaration and every ->> reference to
// it from the other frames. It refuses (rather than cascades) when notes
// or gwt scenarios are attached to the frame.
func (e *SourceEditor) DeleteFrame(id string) error {
	f := e.Frame(id)
	if f == nil {
		return fmt.Errorf("no frame %s", id)
	}
	var blockers []string
	for _, note := range e.model.NoteEntities {
		if note.SourceID == id {
			blockers = append(blockers, fmt.Sprintf("note on line %d", note.Line))
		}
	}
	for _, gwt := range e.model.GWTs {
		if gwt.SourceID == id {
			label := "scenario"
			if gwt.Label != "" {
				label = "scenario " + gwt.Label
			}
			blockers = append(blockers, fmt.Sprintf("%s on line %d", label, gwt.Line))
		}
	}
	if len(blockers) > 0 {
		return fmt.Errorf("frame %s still has a %s attached — remove that first", id, strings.Join(blockers, ", "))
	}

	// Strip ->> id references from every other frame line, then drop the
	// frame's own line. The match must end at whitespace or end-of-line so
	// "03" never eats into a longer ID like "034" (Go's regexp has no
	// lookahead, hence the capture-and-restore group).
	refPattern := regexp.MustCompile(`->>\s*` + regexp.QuoteMeta(id) + `([ \t]|$)`)
	for _, other := range e.model.Frames {
		if other.ID == id {
			continue
		}
		idx := other.Line - 1
		e.lines[idx] = refPattern.ReplaceAllString(e.lines[idx], "$1")
		e.lines[idx] = collapseDoubles(e.lines[idx])
	}
	idx := f.Line - 1
	e.lines = append(e.lines[:idx], e.lines[idx+1:]...)
	return nil
}

// MoveFrame moves the frame one slot earlier (dir -1) or later (dir +1) in
// the timeline, i.e. swaps its declaration line with the neighbouring
// frame's.
func (e *SourceEditor) MoveFrame(id string, dir int) error {
	f := e.Frame(id)
	if f == nil {
		return fmt.Errorf("no frame %s", id)
	}
	ix := -1
	for i, fr := range e.model.Frames {
		if fr.ID == id {
			ix = i
			break
		}
	}
	nx := ix + dir
	if nx < 0 || nx >= len(e.model.Frames) {
		return fmt.Errorf("frame %s is already at the %s of the timeline", id, map[int]string{-1: "start", 1: "end"}[dir])
	}
	neighbour := e.model.Frames[nx]
	a, b := f.Line-1, neighbour.Line-1
	e.lines[a], e.lines[b] = e.lines[b], e.lines[a]
	return nil
}

// setDataBlockValue replaces a named data block's value with payload (the
// inner content without outer braces; may span lines).
func (e *SourceEditor) setDataBlockValue(name, payload string) error {
	for _, data := range e.model.DataEntities {
		if data.Name != name {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "data %s", data.Name)
		if data.DataType != "" {
			fmt.Fprintf(&b, " `%s`", data.DataType)
		}
		b.WriteString(" {\n")
		if payload != "" {
			for _, line := range strings.Split(payload, "\n") {
				b.WriteString("  " + line + "\n")
			}
		}
		b.WriteString("}")
		replacement := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
		start, end := data.Line-1, data.EndLine // EndLine is inclusive
		tail := append([]string(nil), e.lines[end:]...)
		e.lines = append(e.lines[:start], append(replacement, tail...)...)
		return nil
	}
	return fmt.Errorf("no data block %s", name)
}

// rewriteFrameLine rebuilds the frame's (single-line) declaration from its
// parsed state, preserving the original leading keyword (tf/timeframe vs
// rf/resetframe).
func (e *SourceEditor) rewriteFrameLine(f *evml.Frame) {
	kw := "tf"
	line := e.lines[f.Line-1]
	if fields := strings.Fields(line); len(fields) > 0 {
		kw = fields[0]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s %s", kw, f.ID, f.EntityType, f.Identifier)
	for _, src := range f.SourceIDs {
		fmt.Fprintf(&b, " ->> %s", src)
	}
	if f.DataRefName != "" {
		fmt.Fprintf(&b, " [[%s]]", f.DataRefName)
	}
	if f.DataType != "" {
		fmt.Fprintf(&b, " `%s`", f.DataType)
	}
	if f.Data != "" {
		fmt.Fprintf(&b, " %s", f.Data)
	}
	e.lines[f.Line-1] = b.String()
}

// nextFrameID returns the next free numeric frame ID, zero-padded to the
// width of the current highest ID.
func (e *SourceEditor) nextFrameID() string {
	maxID := 0
	width := 2
	for _, f := range e.model.Frames {
		if n, err := strconv.Atoi(f.ID); err == nil && n > maxID {
			maxID = n
			width = len(f.ID)
		}
	}
	return fmt.Sprintf("%0*d", width, maxID+1)
}

// wrapPayload normalizes payload inner content into a balanced payload
// value: "" stays empty, everything else gets wrapped in braces.
func wrapPayload(payload string) string {
	if payload == "" {
		return ""
	}
	payload = strings.TrimSpace(payload)
	if strings.HasPrefix(payload, "{") && strings.HasSuffix(payload, "}") {
		return payload
	}
	return "{ " + payload + " }"
}

// checkPayloadBalanced rejects obviously broken payloads before the
// re-parse would, so the editor can blame the payload rather than the
// whole document.
func checkPayloadBalanced(payload string) error {
	depth := 0
	inString := byte(0)
	escaped := false
	for i := 0; i < len(payload); i++ {
		ch := payload[i]
		switch {
		case inString != 0:
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == inString {
				inString = 0
			}
		case ch == '"' || ch == '\'':
			inString = ch
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			if depth < 0 {
				return fmt.Errorf("payload has an unmatched closing brace")
			}
		}
	}
	if depth != 0 {
		return fmt.Errorf("payload has an unmatched opening brace")
	}
	if inString != 0 {
		return fmt.Errorf("payload has an unterminated string")
	}
	return nil
}

// collapseDoubles squeezes runs of spaces left behind by reference
// stripping back to single spaces.
func collapseDoubles(s string) string {
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}
