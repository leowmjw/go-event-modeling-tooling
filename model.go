package evml

type FrameKind string

const (
	FrameKindTime  FrameKind = "timeframe"
	FrameKindReset FrameKind = "resetframe"
)

type EntityType string

const (
	EntityUI        EntityType = "ui"
	EntityCommand   EntityType = "cmd"
	EntityEvent     EntityType = "evt"
	EntityReadModel EntityType = "rmo"
	EntityProcessor EntityType = "pcr"
)

// Stage classifies how "real" a part of the model is. It is the mechanism
// domain experts use to sketch changes next to the as-is process without
// committing to them:
//
//   - StageCurrent — the process as it runs today (the default).
//   - StageStaging — a proposed change being validated against reality.
//   - StageFuture  — a longer-term goal; kept on the model so the direction
//     is visible, but not expected to be built yet.
type Stage string

const (
	StageCurrent Stage = "current"
	StageStaging Stage = "staging"
	StageFuture  Stage = "future"
)

// AllStages lists the stages in promotion order (future → staging → current).
var AllStages = []Stage{StageCurrent, StageStaging, StageFuture}

// Slice status keywords, per the eventmodelers.ai cheat sheet.
const (
	StatusCreated       = "Created"
	StatusPlanned       = "Planned"
	StatusAssigned      = "Assigned"
	StatusInProgress    = "InProgress"
	StatusReview        = "Review"
	StatusDone          = "Done"
	StatusBlocked       = "Blocked"
	StatusInformational = "Informational"
)

// SliceStatuses lists every accepted slice status keyword in canonical form.
var SliceStatuses = []string{
	StatusCreated, StatusPlanned, StatusAssigned, StatusInProgress,
	StatusReview, StatusDone, StatusBlocked, StatusInformational,
}

type Model struct {
	Frames       []*Frame
	DataEntities []*DataEntity
	NoteEntities []*NoteEntity
	Hotspots     []*Hotspot
	GWTs         []*GWT
	Entities     []string
	Actors       []string
	Chapters     []*Chapter
	Slices       []*Slice
}

type Frame struct {
	Kind          FrameKind
	ID            string
	EntityType    EntityType
	Identifier    string
	Actor         string // optional persona from an @Actor suffix
	StageTag      Stage  // explicit #stage override; "" means inherit from slice
	SourceIDs     []string
	Sources       []*Frame
	DataRefName   string
	DataRef       *DataEntity
	DataType      string
	Data          string
	DeclarationIx int
	Line          int // 1-based source line the declaration starts on
	LineCount     int // number of source lines the declaration spans
}

type DataEntity struct {
	Name     string
	DataType string
	Value    string
}

type NoteEntity struct {
	SourceID  string
	Source    *Frame
	DataType  string
	Value     string
	Line      int
	LineCount int
}

// Hotspot is an unresolved question, blocker, or disputed rule attached to a
// frame. Unlike a NoteEntity it is expected to be resolved (converted into a
// note or removed) before the model is considered finished.
type Hotspot struct {
	SourceID  string
	Source    *Frame
	DataType  string
	Value     string
	Line      int
	LineCount int
}

// Chapter names a contiguous range of frames (by declaration order) so a
// large model reads like a table of contents.
type Chapter struct {
	Name    string
	StartID string
	EndID   string
	Start   *Frame
	End     *Frame
	Line    int
}

// Slice is one vertical cut through the timeline — a single business
// capability — with an optional delivery status and a stage.
type Slice struct {
	Name    string
	StartID string
	EndID   string
	Start   *Frame
	End     *Frame
	Status  string // one of SliceStatuses, or ""
	Stage   Stage  // "" means current
	Line    int
}

type GWT struct {
	SourceID  string
	Source    *Frame
	Label     string
	Given     []Statement
	When      []Statement
	Then      []Statement
	Line      int
	LineCount int
}

type Statement struct {
	EntityType EntityType
	Identifier string
	DataType   string
	Data       string
}

func (f *Frame) Namespace() string {
	for i := 0; i < len(f.Identifier); i++ {
		if f.Identifier[i] == '.' {
			return f.Identifier[:i]
		}
	}
	return ""
}

func (f *Frame) SwimlaneBand() string {
	switch f.EntityType {
	case EntityUI, EntityProcessor:
		return "UI/Automation"
	case EntityCommand, EntityReadModel:
		return "Command/Read Model"
	case EntityEvent:
		return "Events"
	default:
		return "Other"
	}
}

func (f *Frame) SwimlaneLabel() string {
	if ns := f.Namespace(); ns != "" {
		return f.SwimlaneBand() + ": " + ns
	}
	return f.SwimlaneBand()
}

func (f *Frame) DisplayData() string {
	if f.DataRef != nil {
		return StripOuterBraces(f.DataRef.Value)
	}
	return StripOuterBraces(f.Data)
}

// Contains reports whether f falls inside the slice's declaration-order range.
func (s *Slice) Contains(f *Frame) bool {
	if s.Start == nil || s.End == nil || f == nil {
		return false
	}
	return f.DeclarationIx >= s.Start.DeclarationIx && f.DeclarationIx <= s.End.DeclarationIx
}

// Contains reports whether f falls inside the chapter's declaration-order range.
func (c *Chapter) Contains(f *Frame) bool {
	if c.Start == nil || c.End == nil || f == nil {
		return false
	}
	return f.DeclarationIx >= c.Start.DeclarationIx && f.DeclarationIx <= c.End.DeclarationIx
}

// FrameStage resolves the effective stage of f: an explicit #stage tag wins,
// otherwise the innermost (last-declared) enclosing slice's stage, otherwise
// StageCurrent.
func (m *Model) FrameStage(f *Frame) Stage {
	if f.StageTag != "" {
		return f.StageTag
	}
	stage := StageCurrent
	for _, s := range m.Slices {
		if s.Stage != "" && s.Contains(f) {
			stage = s.Stage
		}
	}
	return stage
}

// FrameByID returns the frame declared with id, or nil.
func (m *Model) FrameByID(id string) *Frame {
	for _, f := range m.Frames {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// SliceFor returns the innermost slice containing f, or nil.
func (m *Model) SliceFor(f *Frame) *Slice {
	var found *Slice
	for _, s := range m.Slices {
		if s.Contains(f) {
			found = s
		}
	}
	return found
}

// HotspotsFor returns every hotspot attached to the frame with id.
func (m *Model) HotspotsFor(id string) []*Hotspot {
	var out []*Hotspot
	for _, h := range m.Hotspots {
		if h.SourceID == id {
			out = append(out, h)
		}
	}
	return out
}

// ParseStage normalises a stage keyword (case-insensitive, with the aliases
// "as-is"/"asis" → current and "proposed" → staging). ok is false for
// anything unrecognised.
func ParseStage(raw string) (Stage, bool) {
	switch lower(raw) {
	case "current", "as-is", "asis", "now":
		return StageCurrent, true
	case "staging", "proposed", "stage":
		return StageStaging, true
	case "future", "goal", "later":
		return StageFuture, true
	}
	return "", false
}

// ParseSliceStatus normalises a status keyword case-insensitively to its
// canonical spelling. ok is false for anything unrecognised.
func ParseSliceStatus(raw string) (string, bool) {
	l := lower(raw)
	for _, s := range SliceStatuses {
		if lower(s) == l {
			return s, true
		}
	}
	switch l {
	case "in-progress", "in_progress", "wip":
		return StatusInProgress, true
	case "info":
		return StatusInformational, true
	}
	return "", false
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
