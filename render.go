package evml

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
)

type RenderOptions struct {
	MeasureTextWidth func(text string, monospace bool) float64
}

type boxLayout struct {
	frame       *Frame
	x           float64
	y           float64
	width       float64
	height      float64
	contentRows []string
}

type swimlaneLayout struct {
	label  string
	y      float64
	height float64
}

func RenderSVG(model *Model, opts RenderOptions) (string, error) {
	if opts.MeasureTextWidth == nil {
		opts.MeasureTextWidth = defaultMeasureTextWidth
	}
	edges := inferEdges(model)
	boxes := make([]boxLayout, 0, len(model.Frames))
	swimlanes := orderedSwimlanes(model)
	swimlaneByLabel := map[string]*swimlaneLayout{}
	y := 20.0
	for _, label := range swimlanes {
		swimlaneByLabel[label] = &swimlaneLayout{label: label, y: y, height: 120}
		y += 140
	}
	x := 220.0
	totalWidth := 260.0
	for _, frame := range model.Frames {
		rows := frameContentRows(frame)
		width, height := measureBox(rows, opts)
		lane := swimlaneByLabel[frame.SwimlaneLabel()]
		if height+30 > lane.height {
			lane.height = height + 30
		}
		boxes = append(boxes, boxLayout{
			frame:       frame,
			x:           x,
			y:           lane.y + 20,
			width:       width,
			height:      height,
			contentRows: rows,
		})
		x += width + 40
		totalWidth = x + 40
	}
	sort.SliceStable(boxes, func(i, j int) bool { return boxes[i].frame.DeclarationIx < boxes[j].frame.DeclarationIx })
	reflowSwimlanes(swimlaneByLabel, boxes)
	var b strings.Builder
	laneBottom := swimlaneBottom(swimlaneByLabel)
	noteStartY := laneBottom + 20
	noteHeight := noteStackHeight(model.NoteEntities)
	gwtStartY := noteStartY + noteHeight
	if noteHeight > 0 && len(model.GWTs) > 0 {
		gwtStartY += 20
	}
	totalHeight := diagramHeight(laneBottom, noteHeight, gwtStackHeight(model.GWTs), len(model.GWTs) > 0)
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%0.f" height="%0.f" viewBox="0 0 %0.f %0.f">`, math.Ceil(totalWidth), math.Ceil(totalHeight), math.Ceil(totalWidth), math.Ceil(totalHeight))
	b.WriteString(`<defs><marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto"><polygon points="0 0, 10 3.5, 0 7" fill="#444"/></marker></defs>`)
	b.WriteString(`<style>text{font-family:sans-serif;fill:#222;font-size:12px}.box-title{font-weight:bold}.note text,.gwt text{font-size:11px}.code{font-family:monospace}.lane-label{font-weight:bold;font-size:13px}</style>`)
	for _, label := range swimlanes {
		lane := swimlaneByLabel[label]
		fmt.Fprintf(&b, `<g class="swimlane"><rect x="10" y="%0.f" width="%0.f" height="%0.f" rx="6" fill="#fafafa" stroke="#e0e0e0"/><text class="lane-label" x="24" y="%0.f">%s</text></g>`, lane.y, totalWidth-20, lane.height, lane.y+24, esc(label))
	}
	boxByID := map[string]boxLayout{}
	for _, box := range boxes {
		boxByID[box.frame.ID] = box
	}
	for _, edge := range edges {
		from := boxByID[edge.from.ID]
		to := boxByID[edge.to.ID]
		x1 := from.x + from.width
		y1 := from.y + from.height/2
		x2 := to.x
		y2 := to.y + to.height/2
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f" fill="none" stroke="#666" stroke-width="1.5" marker-end="url(#arrowhead)"/>`, x1, y1, x2, y2)
	}
	for _, box := range boxes {
		renderBox(&b, box)
	}
	renderNotes(&b, model.NoteEntities, boxByID, noteStartY)
	renderGWT(&b, model.GWTs, boxByID, gwtStartY)
	b.WriteString(`</svg>`)
	return b.String(), nil
}

type inferredEdge struct {
	from *Frame
	to   *Frame
}

func inferEdges(model *Model) []inferredEdge {
	var edges []inferredEdge
	for idx, frame := range model.Frames {
		if len(frame.Sources) > 0 {
			for _, src := range frame.Sources {
				edges = append(edges, inferredEdge{from: src, to: frame})
			}
			continue
		}
		if frame.Kind == FrameKindReset {
			continue
		}
		allowed, _, _ := allowedSources(frame.EntityType)
		for i := idx - 1; i >= 0; i-- {
			candidate := model.Frames[i]
			if allowed[candidate.EntityType] {
				edges = append(edges, inferredEdge{from: candidate, to: frame})
				break
			}
			if candidate.Kind == FrameKindReset {
				break
			}
		}
	}
	return edges
}

func orderedSwimlanes(model *Model) []string {
	seen := map[string]bool{}
	var labels []string
	for _, frame := range model.Frames {
		label := frame.SwimlaneLabel()
		if !seen[label] {
			seen[label] = true
			labels = append(labels, label)
		}
	}
	return labels
}

func reflowSwimlanes(lanes map[string]*swimlaneLayout, boxes []boxLayout) {
	seen := map[string]bool{}
	y := 20.0
	for _, box := range boxes {
		label := box.frame.SwimlaneLabel()
		if seen[label] {
			continue
		}
		seen[label] = true
		lane := lanes[label]
		lane.y = y
		y += lane.height + 20
	}
	for i := range boxes {
		lane := lanes[boxes[i].frame.SwimlaneLabel()]
		boxes[i].y = lane.y + 20
	}
}

func diagramHeight(laneBottom, notesHeight, gwtHeight float64, hasGWT bool) float64 {
	total := laneBottom + 20 + notesHeight
	if hasGWT {
		total += 20 + gwtHeight
	}
	return total + 20
}

func swimlaneBottom(lanes map[string]*swimlaneLayout) float64 {
	maxY := 0.0
	for _, lane := range lanes {
		if lane.y+lane.height > maxY {
			maxY = lane.y + lane.height
		}
	}
	return maxY
}

func noteStackHeight(notes []*NoteEntity) float64 {
	total := 0.0
	for _, note := range notes {
		total += 26 + float64(len(dataRows(note.Value)))*16 + 12
	}
	return total
}

func gwtStackHeight(gwts []*GWT) float64 {
	if len(gwts) == 0 {
		return 0
	}
	groupHeights := map[string]float64{}
	for _, gwt := range gwts {
		lines := 3 + len(gwt.Given) + len(gwt.When) + len(gwt.Then)
		groupHeights[gwt.SourceID] += 30 + float64(lines)*15 + 12
	}
	maxHeight := 0.0
	for _, height := range groupHeights {
		if height > maxHeight {
			maxHeight = height
		}
	}
	return maxHeight
}

func frameContentRows(frame *Frame) []string {
	rows := []string{frame.Identifier}
	data := frame.DisplayData()
	if data == "" {
		return rows
	}
	rows = append(rows, dataRows(data)...)
	return rows
}

func dataRows(data string) []string {
	data = StripOuterBraces(data)
	if strings.TrimSpace(data) == "" {
		return nil
	}
	lines := strings.Split(data, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return lines
}

func measureBox(rows []string, opts RenderOptions) (float64, float64) {
	width := 120.0
	for i, row := range rows {
		lineWidth := opts.MeasureTextWidth(strings.TrimSpace(row), i > 0)
		if lineWidth+20 > width {
			width = lineWidth + 20
		}
	}
	height := 34.0
	if len(rows) > 1 {
		height += float64(len(rows)-1) * 16
	}
	return width, height
}

func renderBox(b *strings.Builder, box boxLayout) {
	fill, stroke := frameColors(box.frame.EntityType)
	fmt.Fprintf(b, `<g class="box"><rect x="%0.f" y="%0.f" width="%0.f" height="%0.f" rx="4" fill="%s" stroke="%s"/>`, box.x, box.y, box.width, box.height, fill, stroke)
	fmt.Fprintf(b, `<text class="box-title" x="%0.f" y="%0.f">%s</text>`, box.x+12, box.y+22, esc(box.contentRows[0]))
	for i, row := range box.contentRows[1:] {
		fmt.Fprintf(b, `<text class="code" x="%0.f" y="%0.f">%s</text>`, box.x+12, box.y+42+float64(i)*16, esc(row))
	}
	b.WriteString(`</g>`)
}

func renderNotes(b *strings.Builder, notes []*NoteEntity, boxes map[string]boxLayout, startY float64) {
	y := startY
	for _, note := range notes {
		box := boxes[note.Source.ID]
		rows := dataRows(note.Value)
		height := 26.0 + float64(len(rows))*16
		fmt.Fprintf(b, `<g class="note"><rect x="%0.f" y="%0.f" width="220" height="%0.f" rx="4" fill="#fff6cc" stroke="#c9b458"/>`, box.x, y, height)
		fmt.Fprintf(b, `<text x="%0.f" y="%0.f">Note for %s</text>`, box.x+10, y+18, esc(note.Source.Identifier))
		for i, row := range rows {
			fmt.Fprintf(b, `<text class="code" x="%0.f" y="%0.f">%s</text>`, box.x+10, y+36+float64(i)*16, esc(row))
		}
		b.WriteString(`</g>`)
		y += height + 12
	}
}

func renderGWT(b *strings.Builder, gwts []*GWT, boxes map[string]boxLayout, startY float64) {
	if len(gwts) == 0 {
		return
	}
	type group struct {
		sourceID string
		items    []*GWT
	}
	grouped := map[string]*group{}
	var order []string
	for _, gwt := range gwts {
		if grouped[gwt.SourceID] == nil {
			grouped[gwt.SourceID] = &group{sourceID: gwt.SourceID}
			order = append(order, gwt.SourceID)
		}
		grouped[gwt.SourceID].items = append(grouped[gwt.SourceID].items, gwt)
	}
	for _, sourceID := range order {
		source := boxes[sourceID]
		y := startY
		for _, gwt := range grouped[sourceID].items {
			lines := []string{}
			lines = append(lines, "Given")
			for _, stmt := range gwt.Given {
				lines = append(lines, statementSummary(stmt))
			}
			if len(gwt.When) > 0 {
				lines = append(lines, "When")
				for _, stmt := range gwt.When {
					lines = append(lines, statementSummary(stmt))
				}
			}
			lines = append(lines, "Then")
			for _, stmt := range gwt.Then {
				lines = append(lines, statementSummary(stmt))
			}
			height := 30.0 + float64(len(lines))*15
			fmt.Fprintf(b, `<g class="gwt"><rect x="%0.f" y="%0.f" width="240" height="%0.f" rx="4" fill="#f8f8f8" stroke="#bbbbbb"/>`, source.x, y, height)
			title := "Scenario"
			if gwt.Label != "" {
				title = StripQuotes(gwt.Label)
			}
			fmt.Fprintf(b, `<text x="%0.f" y="%0.f">%s</text>`, source.x+10, y+18, esc(title))
			for i, line := range lines {
				className := ""
				if line == "Given" || line == "When" || line == "Then" {
					className = ` class="box-title"`
				}
				fmt.Fprintf(b, `<text%s x="%0.f" y="%0.f">%s</text>`, className, source.x+10, y+38+float64(i)*15, esc(line))
			}
			b.WriteString(`</g>`)
			y += height + 12
		}
	}
}

func statementSummary(stmt Statement) string {
	if data := strings.TrimSpace(StripOuterBraces(stmt.Data)); data != "" {
		firstLine := strings.TrimSpace(strings.Split(data, "\n")[0])
		return fmt.Sprintf("%s %s { %s }", stmt.EntityType, stmt.Identifier, firstLine)
	}
	return fmt.Sprintf("%s %s", stmt.EntityType, stmt.Identifier)
}

func frameColors(entityType EntityType) (fill, stroke string) {
	switch entityType {
	case EntityUI:
		return "#f8d4bc", "#d38e5f"
	case EntityCommand:
		return "#bcd6fe", "#679ac3"
	case EntityEvent:
		return "#d3f1a2", "#84af49"
	case EntityReadModel:
		return "#bcd6fe", "#679ac3"
	case EntityProcessor:
		return "#f8d4bc", "#d38e5f"
	default:
		return "#eeeeee", "#999999"
	}
}

func defaultMeasureTextWidth(text string, monospace bool) float64 {
	if monospace {
		return float64(len(text)) * 7.2
	}
	return float64(len(text)) * 7.6
}

func StripOuterBraces(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

func StripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func esc(s string) string {
	return html.EscapeString(s)
}
