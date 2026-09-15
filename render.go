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
	// Chapters are drawn as a labelled band at the top of the diagram, so
	// reserve vertical space for them *before* laying out swimlanes. The
	// band only ever occupies one row (chapterPalette cycles, but we
	// intentionally stack nothing here — overlapping chapters would be a
	// validation error).
	chapterBandHeight := 36.0
	if len(model.Chapters) == 0 {
		chapterBandHeight = 0
	}
	y := 20.0 + chapterBandHeight
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
	sliceStartY := laneBottom + 20
	sliceHeight := sliceStackHeight(model.Slices)
	noteStartY := sliceStartY + sliceHeight
	if sliceHeight > 0 {
		noteStartY += 12
	}
	noteHeight := noteStackHeight(model.NoteEntities)
	hotspotStartY := noteStartY + noteHeight
	if noteHeight > 0 && len(model.Hotspots) > 0 {
		hotspotStartY += 12
	}
	hotspotHeight := 0.0
	for _, h := range model.Hotspots {
		hotspotHeight += 26 + float64(len(dataRows(h.Value)))*16 + 12
	}
	gwtStartY := hotspotStartY + hotspotHeight
	if (noteHeight > 0 || hotspotHeight > 0) && len(model.GWTs) > 0 {
		gwtStartY += 20
	}
	totalHeight := diagramHeight(laneBottom, noteHeight, gwtStackHeight(model.GWTs), len(model.GWTs) > 0)
	totalHeight += sliceHeight + hotspotHeight
	if chapterBandHeight > 0 {
		totalHeight += 16
	}
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%0.f" height="%0.f" viewBox="0 0 %0.f %0.f">`, math.Ceil(totalWidth), math.Ceil(totalHeight), math.Ceil(totalWidth), math.Ceil(totalHeight))
	b.WriteString(`<defs><marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto"><polygon points="0 0, 10 3.5, 0 7" fill="#444"/></marker></defs>`)
	b.WriteString(`<style>text{font-family:sans-serif;fill:#222;font-size:12px}.box-title{font-weight:bold}.note text,.gwt text{font-size:11px}.code{font-family:monospace}.lane-label{font-weight:bold;font-size:13px}.chapter-label{font-weight:bold;font-size:12px}.chapter-range{font-size:10px;fill:#666}.slice-label{font-size:11px}.slice-status{font-size:10px;fill:#444}</style>`)
	if chapterBandHeight > 0 {
		// We need the box layout to know each frame's X to draw a chapter
		// band, but we only have boxes after reflow — compute them now.
		boxByID := map[string]boxLayout{}
		for _, box := range boxes {
			boxByID[box.frame.ID] = box
		}
		renderChapters(&b, model, boxByID)
	}
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
	if sliceHeight > 0 {
		renderSlices(&b, model, boxByID, sliceStartY)
	}
	renderNotes(&b, model.NoteEntities, boxByID, noteStartY)
	if len(model.Hotspots) > 0 {
		renderHotspots(&b, model.Hotspots, boxByID, hotspotStartY)
	}
	renderHotspotPins(&b, model, boxByID)
	renderGWT(&b, model.GWTs, boxByID, gwtStartY)
	b.WriteString(`</svg>`)
	return b.String(), nil
}

// sliceStackHeight returns the total vertical space renderSlices will
// consume for the given slice list (zero when none).
func sliceStackHeight(slices []*Slice) float64 {
	if len(slices) == 0 {
		return 0
	}
	const barHeight = 22.0
	const gap = 4.0
	return float64(len(slices))*(barHeight+gap) + gap
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

// sliceStatusColor returns the fill / stroke pair used to render a slice
// status bar. Unknown statuses fall back to a neutral grey so we never
// crash on a freshly-introduced status keyword — validation enforces the
// closed set at parse time, but defence-in-depth is cheap here.
func sliceStatusColor(status SliceStatus) (fill, stroke string) {
	switch status {
	case SliceDone:
		return "#c8e6c9", "#4caf50"
	case SliceInProgress:
		return "#bbdefb", "#2196f3"
	case SliceReview:
		return "#e1bee7", "#8e24aa"
	case SlicePlanned, SliceCreated, SliceAssigned:
		return "#eceff1", "#90a4ae"
	case SliceBlocked:
		return "#ffcdd2", "#e53935"
	case SliceInformational:
		return "#fff59d", "#f9a825"
	default:
		return "#eeeeee", "#999999"
	}
}

// chapterPalette returns a deterministic soft fill/stroke for a chapter,
// indexed by its position in model.Chapters. Three pastel pairs cycle so
// adjacent chapters are visually distinct.
func chapterPalette(ix int) (fill, stroke string) {
	pairs := [][2]string{
		{"#fde7e9", "#e57373"},
		{"#e0f2f1", "#4db6ac"},
		{"#e3f2fd", "#64b5f6"},
		{"#f3e5f5", "#ba68c8"},
		{"#fff8e1", "#ffb74d"},
		{"#e8f5e9", "#81c784"},
	}
	p := pairs[ix%len(pairs)]
	return p[0], p[1]
}

// renderChapters draws one labelled band per chapter across the top of
// the diagram, spanning the chapter's start-to-end frame X-centres.
// It returns the band's height so callers can adjust totalHeight.
func renderChapters(b *strings.Builder, model *Model, boxes map[string]boxLayout) float64 {
	if len(model.Chapters) == 0 {
		return 0
	}
	const bandHeight = 28.0
	for ix, ch := range model.Chapters {
		startBox, ok1 := boxes[ch.StartID]
		endBox, ok2 := boxes[ch.EndID]
		if !ok1 || !ok2 {
			continue
		}
		startX := startBox.x + startBox.width/2
		endX := endBox.x + endBox.width/2
		if endX < startX {
			startX, endX = endX, startX
		}
		width := endX - startX
		if width < 40 {
			width = 40
		}
		fill, stroke := chapterPalette(ix)
		y := 4.0 + float64(ix)*0 // stack only if needed — keep flat for clarity
		fmt.Fprintf(b, `<g class="chapter"><rect x="%0.f" y="%0.f" width="%0.f" height="%0.f" rx="4" fill="%s" stroke="%s" stroke-width="1"/>`, startX, y, width, bandHeight, fill, stroke)
		fmt.Fprintf(b, `<text class="chapter-label" x="%0.f" y="%0.f">%s</text>`, startX+10, y+18, esc(ch.Label))
		fmt.Fprintf(b, `<text class="chapter-range" x="%0.f" y="%0.f">%s–%s</text>`, startX+width-50, y+18, esc(ch.StartID), esc(ch.EndID))
		b.WriteString(`</g>`)
	}
	return bandHeight + 4
}

// renderSlices draws one coloured status bar per slice below the swimlanes
// (above the notes row). It returns the total height consumed.
func renderSlices(b *strings.Builder, model *Model, boxes map[string]boxLayout, startY float64) float64 {
	if len(model.Slices) == 0 {
		return 0
	}
	const barHeight = 22.0
	const gap = 4.0
	y := startY
	for ix, sl := range model.Slices {
		startBox, ok1 := boxes[sl.StartID]
		endBox, ok2 := boxes[sl.EndID]
		if !ok1 || !ok2 {
			continue
		}
		startX := startBox.x
		endX := endBox.x + endBox.width
		if endX < startX {
			startX, endX = endX, startX
		}
		width := endX - startX
		if width < 60 {
			width = 60
		}
		fill, stroke := sliceStatusColor(sl.Status)
		fmt.Fprintf(b, `<g class="slice"><rect x="%0.f" y="%0.f" width="%0.f" height="%0.f" rx="3" fill="%s" stroke="%s"/>`, startX, y, width, barHeight, fill, stroke)
		fmt.Fprintf(b, `<text class="slice-label" x="%0.f" y="%0.f">%s</text>`, startX+8, y+15, esc(sl.Name))
		fmt.Fprintf(b, `<text class="slice-status" x="%0.f" y="%0.f">%s</text>`, startX+width-90, y+15, esc(string(sl.Status)))
		b.WriteString(`</g>`)
		y += barHeight + gap
		_ = ix
	}
	return y - startY
}

// renderHotspotPins overlays a small red circle on every frame that has
// at least one open hotspot, and a grey circle for frames whose hotspots
// are all resolved. This is drawn last so it sits on top of the boxes.
func renderHotspotPins(b *strings.Builder, model *Model, boxes map[string]boxLayout) {
	open := map[string]bool{}
	resolved := map[string]bool{}
	for _, h := range model.Hotspots {
		if h.Status == HotspotOpen {
			open[h.SourceID] = true
		} else {
			resolved[h.SourceID] = true
		}
	}
	for id := range open {
		box, ok := boxes[id]
		if !ok {
			continue
		}
		fmt.Fprintf(b, `<circle class="hotspot-pin" cx="%0.f" cy="%0.f" r="6" fill="#e53935" stroke="#fff" stroke-width="1.5"/>`, box.x+box.width-8, box.y+8)
	}
	for id := range resolved {
		if open[id] {
			continue
		}
		box, ok := boxes[id]
		if !ok {
			continue
		}
		fmt.Fprintf(b, `<circle class="hotspot-pin hotspot-resolved" cx="%0.f" cy="%0.f" r="6" fill="#bdbdbd" stroke="#fff" stroke-width="1.5"/>`, box.x+box.width-8, box.y+8)
	}
}

// renderHotspots draws the hotspot sticky-note stack below the swimlanes,
// one per hotspot, mirroring renderNotes but red.
func renderHotspots(b *strings.Builder, hotspots []*HotspotEntity, boxes map[string]boxLayout, startY float64) float64 {
	if len(hotspots) == 0 {
		return 0
	}
	y := startY
	for _, h := range hotspots {
		box, ok := boxes[h.SourceID]
		if !ok {
			continue
		}
		rows := dataRows(h.Value)
		height := 26.0 + float64(len(rows))*16
		fill := "#fde8e8"
		if h.Status == HotspotResolved {
			fill = "#eeeeee"
		}
		fmt.Fprintf(b, `<g class="hotspot"><rect x="%0.f" y="%0.f" width="220" height="%0.f" rx="4" fill="%s" stroke="#e57373"/>`, box.x, y, height, fill)
		label := "Hotspot for " + h.Source.Identifier
		if h.Status == HotspotResolved {
			label = "Resolved — " + h.Source.Identifier
		}
		fmt.Fprintf(b, `<text x="%0.f" y="%0.f">%s</text>`, box.x+10, y+18, esc(label))
		for i, row := range rows {
			fmt.Fprintf(b, `<text class="code" x="%0.f" y="%0.f">%s</text>`, box.x+10, y+36+float64(i)*16, esc(row))
		}
		b.WriteString(`</g>`)
		y += height + 12
	}
	return y - startY
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
