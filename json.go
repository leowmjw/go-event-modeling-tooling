package evml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type sectionJSON struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

type frameJSON struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Type      string   `json:"type"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Line      int      `json:"line"`
	Section   string   `json:"section"`
	Sources   []string `json:"sources"`
	DataRef   string   `json:"dataRef"`
	DataType  string   `json:"dataType"`
	Data      string   `json:"data"`
}

type dataJSON struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	Value    string `json:"value"`
	Line     int    `json:"line"`
}

type noteJSON struct {
	Frame    string `json:"frame"`
	DataType string `json:"dataType"`
	Value    string `json:"value"`
	Line     int    `json:"line"`
}

type hotspotJSON struct {
	Frame    string `json:"frame"`
	DataType string `json:"dataType"`
	Value    string `json:"value"`
	Line     int    `json:"line"`
}

type statementJSON struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	Data     string `json:"data"`
}

type gwtJSON struct {
	Frame string          `json:"frame"`
	Label string          `json:"label"`
	Line  int             `json:"line"`
	Given []statementJSON `json:"given"`
	When  []statementJSON `json:"when"`
	Then  []statementJSON `json:"then"`
}

type modelJSON struct {
	Schema   string        `json:"schema"`
	Source   string        `json:"source"`
	SHA256   string        `json:"sha256"`
	Sections []sectionJSON `json:"sections"`
	Entities []string      `json:"entities"`
	Frames   []frameJSON   `json:"frames"`
	Data     []dataJSON    `json:"data"`
	Notes    []noteJSON    `json:"notes"`
	Hotspots []hotspotJSON `json:"hotspots"`
	GWTs     []gwtJSON     `json:"gwts"`
}

func statementsJSON(stmts []Statement) []statementJSON {
	out := make([]statementJSON, 0, len(stmts))
	for _, s := range stmts {
		out = append(out, statementJSON{
			Type:     string(s.EntityType),
			Name:     s.Identifier,
			DataType: s.DataType,
			Data:     s.Data,
		})
	}
	return out
}

// ModelJSON serializes a parsed model into the evml-model/v1 JSON shape used
// by tooling (e.g. the compile-evml-rote-ir skill). All arrays are emitted as
// [] rather than null, and no back-references are included.
func ModelJSON(model *Model, source string, content []byte) ([]byte, error) {
	sum := sha256.Sum256(content)
	out := modelJSON{
		Schema:   "evml-model/v1",
		Source:   source,
		SHA256:   hex.EncodeToString(sum[:]),
		Sections: make([]sectionJSON, 0, len(model.Sections)),
		Entities: make([]string, 0, len(model.Entities)),
		Frames:   make([]frameJSON, 0, len(model.Frames)),
		Data:     make([]dataJSON, 0, len(model.DataEntities)),
		Notes:    make([]noteJSON, 0, len(model.NoteEntities)),
		Hotspots: make([]hotspotJSON, 0, len(model.HotspotEntities)),
		GWTs:     make([]gwtJSON, 0, len(model.GWTs)),
	}
	for _, s := range model.Sections {
		out.Sections = append(out.Sections, sectionJSON{Name: s.Name, Line: s.Line})
	}
	out.Entities = append(out.Entities, model.Entities...)
	for _, f := range model.Frames {
		kind := "tf"
		if f.Kind == FrameKindReset {
			kind = "rf"
		}
		out.Frames = append(out.Frames, frameJSON{
			ID:        f.ID,
			Kind:      kind,
			Type:      string(f.EntityType),
			Name:      f.Identifier,
			Namespace: f.Namespace(),
			Line:      f.Line,
			Section:   f.Section,
			Sources:   append([]string{}, f.SourceIDs...),
			DataRef:   f.DataRefName,
			DataType:  f.DataType,
			Data:      f.Data,
		})
	}
	for _, d := range model.DataEntities {
		out.Data = append(out.Data, dataJSON{Name: d.Name, DataType: d.DataType, Value: d.Value, Line: d.Line})
	}
	for _, n := range model.NoteEntities {
		out.Notes = append(out.Notes, noteJSON{Frame: n.SourceID, DataType: n.DataType, Value: n.Value, Line: n.Line})
	}
	for _, h := range model.HotspotEntities {
		out.Hotspots = append(out.Hotspots, hotspotJSON{Frame: h.SourceID, DataType: h.DataType, Value: h.Value, Line: h.Line})
	}
	for _, g := range model.GWTs {
		out.GWTs = append(out.GWTs, gwtJSON{
			Frame: g.SourceID,
			Label: StripQuotes(g.Label),
			Line:  g.Line,
			Given: statementsJSON(g.Given),
			When:  statementsJSON(g.When),
			Then:  statementsJSON(g.Then),
		})
	}
	return json.MarshalIndent(out, "", "  ")
}
