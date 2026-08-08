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
			model.Frames = append(model.Frames, frame)
			p.line += consumed
		case hasKeyword(trimmed, "rf"), hasKeyword(trimmed, "resetframe"):
			frame, consumed, err := p.parseFrame(trimmed)
			if err != nil {
				return nil, err
			}
			frame.Kind = FrameKindReset
			frame.DeclarationIx = len(model.Frames)
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
			model.NoteEntities = append(model.NoteEntities, note)
			p.line += consumed
		case hasKeyword(trimmed, "gwt"):
			gwt, consumed, err := p.parseGWT(trimmed)
			if err != nil {
				return nil, err
			}
			model.GWTs = append(model.GWTs, gwt)
			p.line += consumed
		case hasKeyword(trimmed, "entity"):
			name, err := parseEntityDecl(trimmed)
			if err != nil {
				return nil, p.errorf("%s", err)
			}
			model.Entities = append(model.Entities, name)
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
		default:
			dataType, data, consumed, err := p.parsePayload(rest, false)
			if err != nil {
				return nil, 0, err
			}
			frame.DataType = dataType
			frame.Data = data
			return frame, consumed, nil
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
		data, consumed, err := collectBalanced(rest, p.lines[p.line+1:], allowMultiline, p.line+1)
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
	return nil
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

func collectBalanced(initial string, extra []string, allowMultiline bool, lineNumber int) (string, int, error) {
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
			case ch == '"' || ch == '\'':
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
