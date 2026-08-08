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
	Frames       []*Frame
	DataEntities []*DataEntity
	NoteEntities []*NoteEntity
	GWTs         []*GWT
	Entities     []string
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
