package evml

import (
	"fmt"
	"strings"
	"unicode"
)

type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func Parse(input string) (*Model, error) {
	p := &parser{
		lines: normalizeNewlines(input),
	}
	return p.parse()
}

type parser struct {
	lines []string
	line  int
}

func (p *parser) parse() (*Model, error) {
	model := &Model{}
	for p.line < len(p.lines) && isIgnorableLine(p.lines[p.line]) {
		p.line++
	}
	if p.line >= len(p.lines) || strings.TrimSpace(p.lines[p.line]) != "eventmodeling" {
		return nil, p.errorf("expected eventmodeling header")
	}
	p.line++
	for p.line < len(p.lines) {
		if isIgnorableLine(p.lines[p.line]) {
			p.line++
			continue
		}
		trimmed := strings.TrimSpace(p.lines[p.line])
		switch {
		case hasKeyword(trimmed, "tf"), hasKeyword(trimmed, "timeframe"):
			frame, consumed, err := p.parseFrame(trimmed)
			if err != nil {
				return nil, err
			}
			frame.DeclarationIx = len(model.Frames)
			frame.Line, frame.LineCount = p.line+1, consumed
			model.Frames = append(model.Frames, frame)
			p.line += consumed
		case hasKeyword(trimmed, "rf"), hasKeyword(trimmed, "resetframe"):
			frame, consumed, err := p.parseFrame(trimmed)
			if err != nil {
				return nil, err
			}
			frame.Kind = FrameKindReset
			frame.DeclarationIx = len(model.Frames)
			frame.Line, frame.LineCount = p.line+1, consumed
			model.Frames = append(model.Frames, frame)
			p.line += consumed
		case hasKeyword(trimmed, "data"):
			entity, consumed, err := p.parseDataEntity(trimmed)
			if err != nil {
				return nil, err
			}
			model.DataEntities = append(model.DataEntities, entity)
			p.line += consumed
		case hasKeyword(trimmed, "note"):
			note, consumed, err := p.parseNoteEntity(trimmed)
			if err != nil {
				return nil, err
			}
			note.Line, note.LineCount = p.line+1, consumed
			model.NoteEntities = append(model.NoteEntities, note)
			p.line += consumed
		case hasKeyword(trimmed, "hotspot"):
			note, consumed, err := p.parseNoteEntity(trimmed)
			if err != nil {
				return nil, err
			}
			model.Hotspots = append(model.Hotspots, &Hotspot{
				SourceID: note.SourceID, DataType: note.DataType, Value: note.Value,
				Line: p.line + 1, LineCount: consumed,
			})
			p.line += consumed
		case hasKeyword(trimmed, "gwt"):
			gwt, consumed, err := p.parseGWT(trimmed)
			if err != nil {
				return nil, err
			}
			gwt.Line, gwt.LineCount = p.line+1, consumed
			model.GWTs = append(model.GWTs, gwt)
			p.line += consumed
		case hasKeyword(trimmed, "entity"):
			name, err := parseEntityDecl(trimmed)
			if err != nil {
				return nil, p.errorf("%s", err)
			}
			model.Entities = append(model.Entities, name)
			p.line++
		case hasKeyword(trimmed, "actor"):
			name, err := parseEntityDecl(trimmed)
			if err != nil {
				return nil, p.errorf("missing actor name")
			}
			model.Actors = append(model.Actors, name)
			p.line++
		case hasKeyword(trimmed, "chapter"):
			ch, err := p.parseChapter(trimmed)
			if err != nil {
				return nil, err
			}
			ch.Line = p.line + 1
			model.Chapters = append(model.Chapters, ch)
			p.line++
		case hasKeyword(trimmed, "slice"):
			sl, err := p.parseSlice(trimmed)
			if err != nil {
				return nil, err
			}
			sl.Line = p.line + 1
			model.Slices = append(model.Slices, sl)
			p.line++
		default:
			return nil, p.errorf("unrecognized top-level statement")
		}
	}
	if err := resolveReferences(model); err != nil {
		return nil, err
	}
	return model, nil
}

func (p *parser) parseFrame(trimmed string) (*Frame, int, error) {
	kind := FrameKindTime
	rest := afterKeyword(trimmed)
	if strings.HasPrefix(trimmed, "rf ") || trimmed == "rf" || strings.HasPrefix(trimmed, "resetframe ") {
		kind = FrameKindReset
	}
	id, rest, ok := nextToken(rest)
	if !ok {
		return nil, 0, p.errorf("missing timeframe identifier")
	}
	rawType, rest, ok := nextToken(rest)
	if !ok {
		return nil, 0, p.errorf("missing entity type")
	}
	entityType, err := parseEntityType(rawType)
	if err != nil {
		return nil, 0, p.errorf("%s", err)
	}
	identifier, rest, ok := nextToken(rest)
	if !ok {
		return nil, 0, p.errorf("missing entity identifier")
	}
	frame := &Frame{
		Kind:       kind,
		ID:         id,
		EntityType: entityType,
		Identifier: identifier,
	}
	for {
		rest = strings.TrimSpace(rest)
		switch {
		case rest == "":
			return frame, 1, nil
		case strings.HasPrefix(rest, "->>"):
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "->>"))
			sourceID, remainder, found := nextToken(rest)
			if !found {
				return nil, 0, p.errorf("missing source frame identifier after ->>")
			}
			frame.SourceIDs = append(frame.SourceIDs, sourceID)
			rest = remainder
		case strings.HasPrefix(rest, "[["):
			end := strings.Index(rest, "]]")
			if end < 0 {
				return nil, 0, p.errorf("unterminated data reference")
			}
			frame.DataRefName = strings.TrimSpace(rest[2:end])
			rest = rest[end+2:]
		case strings.HasPrefix(rest, "@"):
			actor, remainder, _ := nextToken(rest[1:])
			if actor == "" {
				return nil, 0, p.errorf("missing actor name after @")
			}
			frame.Actor = actor
			rest = remainder
		case strings.HasPrefix(rest, "#"):
			raw, remainder, _ := nextToken(rest[1:])
			stage, ok := ParseStage(raw)
			if !ok {
				return nil, 0, p.errorf("unknown stage %q (want current, staging or future)", raw)
			}
			frame.StageTag = stage
			rest = remainder
		default:
			dataType, data, consumed, err := p.parsePayload(rest, false)
			if err != nil {
				return nil, 0, err
			}
			frame.DataType = dataType
			frame.Data = data
			// Modifiers may also trail a single-line payload:
			//   tf 12 evt Foo { a: 1 } #future @Ops
			if consumed == 1 && strings.HasPrefix(data, "{") {
				if ix := strings.Index(rest, data); ix >= 0 {
					if err := p.parseTrailingModifiers(frame, rest[ix+len(data):]); err != nil {
						return nil, 0, err
					}
				}
			}
			return frame, consumed, nil
		}
	}
}

// parseTrailingModifiers accepts only @Actor / #stage tokens after a payload.
func (p *parser) parseTrailingModifiers(frame *Frame, rest string) error {
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return nil
		}
		switch {
		case strings.HasPrefix(rest, "@"):
			actor, remainder, _ := nextToken(rest[1:])
			if actor == "" {
				return p.errorf("missing actor name after @")
			}
			frame.Actor = actor
			rest = remainder
		case strings.HasPrefix(rest, "#"):
			raw, remainder, _ := nextToken(rest[1:])
			stage, ok := ParseStage(raw)
			if !ok {
				return p.errorf("unknown stage %q (want current, staging or future)", raw)
			}
			frame.StageTag = stage
			rest = remainder
		default:
			return p.errorf("unexpected content after payload: %q", rest)
		}
	}
}

func (p *parser) parseDataEntity(trimmed string) (*DataEntity, int, error) {
	rest := afterKeyword(trimmed)
	name, rest, ok := nextToken(rest)
	if !ok {
		return nil, 0, p.errorf("missing data entity name")
	}
	dataType, data, consumed, err := p.parsePayload(rest, true)
	if err != nil {
		return nil, 0, err
	}
	if data == "" {
		return nil, 0, p.errorf("missing data block")
	}
	return &DataEntity{Name: name, DataType: dataType, Value: data}, consumed, nil
}

func (p *parser) parseNoteEntity(trimmed string) (*NoteEntity, int, error) {
	rest := afterKeyword(trimmed)
	sourceID, rest, ok := nextToken(rest)
	if !ok {
		return nil, 0, p.errorf("missing note source frame identifier")
	}
	dataType, data, consumed, err := p.parsePayload(rest, true)
	if err != nil {
		return nil, 0, err
	}
	if data == "" {
		return nil, 0, p.errorf("missing note payload")
	}
	return &NoteEntity{SourceID: sourceID, DataType: dataType, Value: data}, consumed, nil
}

func (p *parser) parseGWT(trimmed string) (*GWT, int, error) {
	rest := afterKeyword(trimmed)
	sourceID, rest, ok := nextToken(rest)
	if !ok {
		return nil, 0, p.errorf("missing gwt source frame identifier")
	}
	gwt := &GWT{SourceID: sourceID}
	rest = strings.TrimSpace(rest)
	if rest != "" {
		label, _, err := parseQuoted(rest)
		if err != nil {
			return nil, 0, p.errorf("invalid gwt label")
		}
		gwt.Label = label
	}
	consumed := 1
	section := ""
	for p.line+consumed < len(p.lines) {
		line := p.lines[p.line+consumed]
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			consumed++
			continue
		}
		if !isIndented(line) && isTopLevel(trimmedLine) {
			break
		}
		switch trimmedLine {
		case "given", "when", "then":
			section = trimmedLine
			consumed++
			continue
		}
		if section == "" {
			return nil, 0, &ParseError{Line: p.line + consumed + 1, Msg: "expected given/when/then section"}
		}
		stmt, stmtConsumed, err := p.parseGWTStatement(trimmedLine, p.line+consumed)
		if err != nil {
			return nil, 0, err
		}
		switch section {
		case "given":
			gwt.Given = append(gwt.Given, *stmt)
		case "when":
			gwt.When = append(gwt.When, *stmt)
		case "then":
			gwt.Then = append(gwt.Then, *stmt)
		}
		consumed += stmtConsumed
	}
	if len(gwt.Given) == 0 || len(gwt.Then) == 0 {
		return nil, 0, p.errorf("gwt requires given and then statements")
	}
	// Don't count trailing blank lines as part of the block, so LineCount
	// describes exactly the lines an editor should remove or replace.
	for consumed > 1 && strings.TrimSpace(p.lines[p.line+consumed-1]) == "" {
		consumed--
	}
	return gwt, consumed, nil
}

func (p *parser) parseGWTStatement(trimmed string, lineIndex int) (*Statement, int, error) {
	rawType, rest, ok := nextToken(trimmed)
	if !ok {
		return nil, 0, &ParseError{Line: lineIndex + 1, Msg: "missing statement entity type"}
	}
	entityType, err := parseEntityType(rawType)
	if err != nil {
		return nil, 0, &ParseError{Line: lineIndex + 1, Msg: err.Error()}
	}
	identifier, rest, ok := nextToken(rest)
	if !ok {
		return nil, 0, &ParseError{Line: lineIndex + 1, Msg: "missing statement identifier"}
	}
	pp := &parser{lines: p.lines, line: lineIndex}
	dataType, data, consumed, err := pp.parsePayload(rest, true)
	if err != nil {
		return nil, 0, err
	}
	return &Statement{EntityType: entityType, Identifier: identifier, DataType: dataType, Data: data}, consumed, nil
}

func (p *parser) parsePayload(rest string, allowMultiline bool) (string, string, int, error) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", "", 1, nil
	}
	dataType := ""
	if strings.HasPrefix(rest, "`") {
		end := strings.Index(rest[1:], "`")
		if end < 0 {
			return "", "", 0, p.errorf("unterminated data type")
		}
		dataType = rest[1 : end+1]
		rest = strings.TrimSpace(rest[end+2:])
	}
	if rest == "" {
		return dataType, "", 1, nil
	}
	switch rest[0] {
	case '{':
		data, consumed, err := collectBalanced(rest, p.lines[p.line+1:], allowMultiline, p.line+1, true)
		if err != nil && allowMultiline {
			// Prose blocks (notes, hotspots) often contain a lone apostrophe
			// ("the bank's clock") that would otherwise be read as an
			// unterminated string. Retry treating quotes as plain text.
			if data2, consumed2, err2 := collectBalanced(rest, p.lines[p.line+1:], allowMultiline, p.line+1, false); err2 == nil {
				return dataType, data2, consumed2, nil
			}
		}
		return dataType, data, consumed, err
	case '"', '\'':
		data, _, err := parseQuoted(rest)
		if err != nil {
			return "", "", 0, p.errorf("invalid quoted payload")
		}
		return dataType, data, 1, nil
	default:
		return "", "", 0, p.errorf("unexpected payload content")
	}
}

func parseEntityDecl(trimmed string) (string, error) {
	name, _, ok := nextToken(afterKeyword(trimmed))
	if !ok {
		return "", fmt.Errorf("missing entity name")
	}
	return name, nil
}

func resolveReferences(model *Model) error {
	frames := map[string]*Frame{}
	for _, frame := range model.Frames {
		if _, ok := frames[frame.ID]; ok {
			return fmt.Errorf("duplicate frame identifier %s", frame.ID)
		}
		frames[frame.ID] = frame
	}
	dataMap := map[string]*DataEntity{}
	for _, data := range model.DataEntities {
		dataMap[data.Name] = data
	}
	for _, frame := range model.Frames {
		for _, id := range frame.SourceIDs {
			source, ok := frames[id]
			if !ok {
				return fmt.Errorf("unknown source frame %s", id)
			}
			frame.Sources = append(frame.Sources, source)
		}
		if frame.DataRefName != "" {
			ref, ok := dataMap[frame.DataRefName]
			if !ok {
				return fmt.Errorf("unknown data reference %s", frame.DataRefName)
			}
			frame.DataRef = ref
		}
	}
	for _, note := range model.NoteEntities {
		source, ok := frames[note.SourceID]
		if !ok {
			return fmt.Errorf("unknown note source frame %s", note.SourceID)
		}
		note.Source = source
	}
	for _, gwt := range model.GWTs {
		source, ok := frames[gwt.SourceID]
		if !ok {
			return fmt.Errorf("unknown gwt source frame %s", gwt.SourceID)
		}
		gwt.Source = source
	}
	for _, h := range model.Hotspots {
		source, ok := frames[h.SourceID]
		if !ok {
			return fmt.Errorf("unknown hotspot source frame %s", h.SourceID)
		}
		h.Source = source
	}
	for _, ch := range model.Chapters {
		start, ok := frames[ch.StartID]
		if !ok {
			return fmt.Errorf("chapter %q: unknown start frame %s", ch.Name, ch.StartID)
		}
		end, ok := frames[ch.EndID]
		if !ok {
			return fmt.Errorf("chapter %q: unknown end frame %s", ch.Name, ch.EndID)
		}
		ch.Start, ch.End = start, end
	}
	for _, sl := range model.Slices {
		start, ok := frames[sl.StartID]
		if !ok {
			return fmt.Errorf("slice %q: unknown start frame %s", sl.Name, sl.StartID)
		}
		end, ok := frames[sl.EndID]
		if !ok {
			return fmt.Errorf("slice %q: unknown end frame %s", sl.Name, sl.EndID)
		}
		sl.Start, sl.End = start, end
	}
	return nil
}

// parseChapter handles: chapter "<Name>" <startId>-<endId>
func (p *parser) parseChapter(trimmed string) (*Chapter, error) {
	name, rest, err := p.parseQuotedName(afterKeyword(trimmed), "chapter")
	if err != nil {
		return nil, err
	}
	startID, endID, rest, err := p.parseRange(rest, "chapter")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(rest) != "" {
		return nil, p.errorf("unexpected trailing content after chapter range: %q", strings.TrimSpace(rest))
	}
	return &Chapter{Name: name, StartID: startID, EndID: endID}, nil
}

// parseSlice handles: slice "<Name>" <startId>-<endId> [status <Status>] [stage <Stage>]
func (p *parser) parseSlice(trimmed string) (*Slice, error) {
	name, rest, err := p.parseQuotedName(afterKeyword(trimmed), "slice")
	if err != nil {
		return nil, err
	}
	startID, endID, rest, err := p.parseRange(rest, "slice")
	if err != nil {
		return nil, err
	}
	sl := &Slice{Name: name, StartID: startID, EndID: endID}
	for {
		key, remainder, ok := nextToken(rest)
		if !ok {
			return sl, nil
		}
		value, remainder, ok := nextToken(remainder)
		if !ok {
			return nil, p.errorf("slice %q: missing value after %q", name, key)
		}
		switch key {
		case "status":
			status, ok := ParseSliceStatus(value)
			if !ok {
				return nil, p.errorf("slice %q: unknown status %q (want one of %s)", name, value, strings.Join(SliceStatuses, ", "))
			}
			sl.Status = status
		case "stage":
			stage, ok := ParseStage(value)
			if !ok {
				return nil, p.errorf("slice %q: unknown stage %q (want current, staging or future)", name, value)
			}
			sl.Stage = stage
		default:
			return nil, p.errorf("slice %q: unknown modifier %q (want status or stage)", name, key)
		}
		rest = remainder
	}
}

// parseQuotedName reads a mandatory quoted name at the start of rest and
// returns it without quotes.
func (p *parser) parseQuotedName(rest, what string) (string, string, error) {
	rest = strings.TrimSpace(rest)
	quoted, remainder, err := parseQuoted(rest)
	if err != nil {
		return "", "", p.errorf("%s requires a quoted name, e.g. %s \"Operations\" 01-07", what, what)
	}
	name := StripQuotes(quoted)
	if strings.TrimSpace(name) == "" {
		return "", "", p.errorf("%s name must not be empty", what)
	}
	return name, remainder, nil
}

// parseRange reads a <startId>-<endId> token (or "<startId> - <endId>").
func (p *parser) parseRange(rest, what string) (string, string, string, error) {
	tok, remainder, ok := nextToken(rest)
	if !ok {
		return "", "", "", p.errorf("%s requires a frame range, e.g. 01-07", what)
	}
	// Allow "01 - 07" spelled with spaces.
	if !strings.Contains(tok, "-") {
		dash, r2, ok := nextToken(remainder)
		if ok && dash == "-" {
			end, r3, ok := nextToken(r2)
			if !ok {
				return "", "", "", p.errorf("%s range is missing its end frame", what)
			}
			tok, remainder = tok+"-"+end, r3
		}
	}
	parts := strings.SplitN(tok, "-", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", p.errorf("%s range %q must look like <start>-<end>, e.g. 01-07", what, tok)
	}
	for _, id := range parts {
		if !isFrameID(id) {
			return "", "", "", p.errorf("%s range %q: %q is not a 1-3 digit frame id", what, tok, id)
		}
	}
	return parts[0], parts[1], remainder, nil
}

func isFrameID(s string) bool {
	if len(s) == 0 || len(s) > 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func normalizeNewlines(input string) []string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	return strings.Split(input, "\n")
}

func isIgnorableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "%%") || strings.HasPrefix(trimmed, "//")
}

func isIndented(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
}

func isTopLevel(trimmed string) bool {
	return hasKeyword(trimmed, "tf") || hasKeyword(trimmed, "timeframe") ||
		hasKeyword(trimmed, "rf") || hasKeyword(trimmed, "resetframe") ||
		hasKeyword(trimmed, "data") || hasKeyword(trimmed, "note") ||
		hasKeyword(trimmed, "hotspot") || hasKeyword(trimmed, "actor") ||
		hasKeyword(trimmed, "chapter") || hasKeyword(trimmed, "slice") ||
		hasKeyword(trimmed, "gwt") || hasKeyword(trimmed, "entity")
}

func hasKeyword(s, kw string) bool {
	return s == kw || strings.HasPrefix(s, kw+" ")
}

func afterKeyword(s string) string {
	for i := 0; i < len(s); i++ {
		if unicode.IsSpace(rune(s[i])) {
			return strings.TrimSpace(s[i:])
		}
	}
	return ""
}

func nextToken(s string) (token, rest string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}
	for i := 0; i < len(s); i++ {
		if unicode.IsSpace(rune(s[i])) {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", true
}

func parseQuoted(s string) (string, string, error) {
	if len(s) == 0 || (s[0] != '"' && s[0] != '\'') {
		return "", "", fmt.Errorf("expected quoted string")
	}
	quote := s[0]
	escaped := false
	for i := 1; i < len(s); i++ {
		switch {
		case escaped:
			escaped = false
		case s[i] == '\\':
			escaped = true
		case s[i] == quote:
			return s[:i+1], s[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("unterminated quoted string")
}

func collectBalanced(initial string, extra []string, allowMultiline bool, lineNumber int, quoteAware bool) (string, int, error) {
	var b strings.Builder
	b.WriteString(initial)
	depth := 0
	inString := byte(0)
	escaped := false
	consumedLines := 1
	for {
		s := b.String()
		depth = 0
		inString = 0
		escaped = false
		started := false
		for i := 0; i < len(s); i++ {
			ch := s[i]
			switch {
			case inString != 0:
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
				} else if ch == inString {
					inString = 0
				}
			case quoteAware && (ch == '"' || ch == '\''):
				inString = ch
			case ch == '{':
				depth++
				started = true
			case ch == '}':
				depth--
				if started && depth == 0 {
					return s[:i+1], consumedLines, nil
				}
				if depth < 0 {
					return "", 0, &ParseError{Line: lineNumber, Msg: "unexpected closing brace"}
				}
			}
		}
		if !allowMultiline || consumedLines > len(extra) {
			return "", 0, &ParseError{Line: lineNumber, Msg: "unbalanced payload braces"}
		}
		b.WriteByte('\n')
		b.WriteString(extra[consumedLines-1])
		consumedLines++
	}
}

func parseEntityType(raw string) (EntityType, error) {
	switch raw {
	case "ui", "scn", "screen":
		return EntityUI, nil
	case "cmd", "command":
		return EntityCommand, nil
	case "evt", "event":
		return EntityEvent, nil
	case "rmo", "readmodel":
		return EntityReadModel, nil
	case "pcr", "processor":
		return EntityProcessor, nil
	default:
		return "", fmt.Errorf("unknown entity type %q", raw)
	}
}

func (p *parser) errorf(format string, args ...any) error {
	return &ParseError{Line: p.line + 1, Msg: fmt.Sprintf(format, args...)}
}
