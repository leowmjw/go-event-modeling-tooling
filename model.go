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

type Model struct {
	Frames         []*Frame
	DataEntities   []*DataEntity
	NoteEntities   []*NoteEntity
	Hotspots       []*HotspotEntity
	Chapters       []*Chapter
	Slices         []*Slice
	GWTs           []*GWT
	Entities       []string
}

// HotspotStatus is the resolution state of a HotspotEntity.
type HotspotStatus string

const (
	HotspotOpen     HotspotStatus = "open"
	HotspotResolved HotspotStatus = "resolved"
)

// HotspotEntity attaches an open question / blocker to a frame. Distinct
// from NoteEntity semantically: a hotspot marks an unresolved decision
// the workshop owes an answer to, not a finished annotation.
type HotspotEntity struct {
	SourceID string
	Source   *Frame
	DataType string
	Value    string
	Status   HotspotStatus
}

// Chapter is a labelled, contiguous frame range that represents a bounded
// context across the timeline. Frame ranges must be non-overlapping.
type Chapter struct {
	Label   string
	StartID string
	EndID   string
}

// SliceStatus is one of the workflow-build stages a slice can be in.
// See EVENT_MODELING.md §16.
type SliceStatus string

const (
	SliceCreated       SliceStatus = "Created"
	SlicePlanned       SliceStatus = "Planned"
	SliceAssigned      SliceStatus = "Assigned"
	SliceInProgress    SliceStatus = "InProgress"
	SliceReview        SliceStatus = "Review"
	SliceDone          SliceStatus = "Done"
	SliceBlocked       SliceStatus = "Blocked"
	SliceInformational SliceStatus = "Informational"
)

// AllSliceStatuses returns every status the parser will accept.
func AllSliceStatuses() []SliceStatus {
	return []SliceStatus{
		SliceCreated, SlicePlanned, SliceAssigned, SliceInProgress,
		SliceReview, SliceDone, SliceBlocked, SliceInformational,
	}
}

// IsValidSliceStatus reports whether s is one of the recognised
// SliceStatus keywords. The check is case-sensitive; documentation and
// fixtures both use the PascalCase spelling.
func IsValidSliceStatus(s string) bool {
	for _, k := range AllSliceStatuses() {
		if string(k) == s {
			return true
		}
	}
	return false
}

// Slice is a named, framed range of the timeline with an explicit build
// status. Used to turn "MVP / next / future" into something the diagram
// renders and tooling can query, rather than living only in section
// comments.
type Slice struct {
	Name    string
	StartID string
	EndID   string
	Status  SliceStatus
}

type Frame struct {
	Kind          FrameKind
	ID            string
	EntityType    EntityType
	Identifier    string
	SourceIDs     []string
	Sources       []*Frame
	DataRefName   string
	DataRef       *DataEntity
	DataType      string
	Data          string
	DeclarationIx int
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
