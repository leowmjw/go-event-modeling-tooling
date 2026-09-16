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

// SliceStatus describes how well a slice matches reality. It is the
// "staging" vocabulary domain experts use in a live modelling session:
// what runs today, what is being trialled, what is a stated future goal,
// what is blocked on an open decision, and what is being retired.
type SliceStatus string

const (
	StatusLive       SliceStatus = "live"       // in production today — the as-is process
	StatusStaging    SliceStatus = "staging"    // being trialled / proposed in this session
	StatusFuture     SliceStatus = "future"     // agreed direction, not yet being built
	StatusBlocked    SliceStatus = "blocked"    // cannot proceed until a hotspot is resolved
	StatusDeprecated SliceStatus = "deprecated" // still exists but is being retired
)

// AllSliceStatuses lists every accepted status keyword, in display order.
var AllSliceStatuses = []SliceStatus{StatusLive, StatusStaging, StatusFuture, StatusBlocked, StatusDeprecated}

// ParseSliceStatus maps a keyword (case-insensitive) to a SliceStatus.
func ParseSliceStatus(raw string) (SliceStatus, bool) {
	for _, s := range AllSliceStatuses {
		if string(s) == lower(raw) {
			return s, true
		}
	}
	return "", false
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

type Model struct {
	Frames       []*Frame
	DataEntities []*DataEntity
	NoteEntities []*NoteEntity
	Hotspots     []*Hotspot
	Chapters     []*Chapter
	Slices       []*Slice
	Actors       []string
	GWTs         []*GWT
	Entities     []string
}

type Frame struct {
	Kind          FrameKind
	ID            string
	EntityType    EntityType
	Identifier    string
	Actor         string // optional persona, from an "@Actor" suffix
	SourceIDs     []string
	Sources       []*Frame
	DataRefName   string
	DataRef       *DataEntity
	DataType      string
	Data          string
	DeclarationIx int
}

// Hotspot is an unresolved question or blocker attached to a frame. Unlike
// a NoteEntity it is meant to be resolved (turned into a note, or removed)
// before a model is considered finished — see OpenHotspots and Lint.
type Hotspot struct {
	SourceID string
	Source   *Frame
	DataType string
	Value    string
}

// Chapter groups a contiguous numeric range of frame IDs under a heading
// (typically a bounded context or a phase of the process). Purely a
// rendering/navigation concern.
type Chapter struct {
	Name    string
	StartID string
	EndID   string
}

// Slice is a vertical cut through the timeline covering one coherent user
// story, with a status describing how well it matches reality today.
type Slice struct {
	Name    string
	StartID string
	EndID   string
	Status  SliceStatus // "" when not stated
}

// FrameIDNumber converts a 1-3 digit frame ID ("01", "7", "123") to an
// int for range comparisons. ok is false for non-numeric IDs.
func FrameIDNumber(id string) (int, bool) {
	if id == "" || len(id) > 3 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// Contains reports whether frame f falls inside the numeric ID range
// [StartID, EndID] of s.
func (s *Slice) Contains(f *Frame) bool { return idInRange(f.ID, s.StartID, s.EndID) }

// Contains reports whether frame f falls inside the numeric ID range
// [StartID, EndID] of c.
func (c *Chapter) Contains(f *Frame) bool { return idInRange(f.ID, c.StartID, c.EndID) }

func idInRange(id, start, end string) bool {
	n, ok := FrameIDNumber(id)
	if !ok {
		return false
	}
	lo, ok1 := FrameIDNumber(start)
	hi, ok2 := FrameIDNumber(end)
	if !ok1 || !ok2 {
		return false
	}
	return n >= lo && n <= hi
}

// StatusOf returns the status of the first slice containing f, or "" when
// f is not covered by any slice with a stated status.
func (m *Model) StatusOf(f *Frame) SliceStatus {
	for _, s := range m.Slices {
		if s.Status != "" && s.Contains(f) {
			return s.Status
		}
	}
	return ""
}

// HotspotsFor returns every hotspot attached to frame f.
func (m *Model) HotspotsFor(f *Frame) []*Hotspot {
	var out []*Hotspot
	for _, h := range m.Hotspots {
		if h.Source == f {
			out = append(out, h)
		}
	}
	return out
}

// FrameByID looks up a frame by its declared ID.
func (m *Model) FrameByID(id string) *Frame {
	for _, f := range m.Frames {
		if f.ID == id {
			return f
		}
	}
	return nil
}

type DataEntity struct {
	Name     string
	DataType string
	Value    string
}

type NoteEntity struct {
	SourceID string
	Source   *Frame
	DataType string
	Value    string
}

type GWT struct {
	SourceID string
	Source   *Frame
	Label    string
	Given    []Statement
	When     []Statement
	Then     []Statement
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
