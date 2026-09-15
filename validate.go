package evml

import (
	"fmt"
	"strings"
)

// Validate runs every structural check (connections, chapter/slice ranges)
// and returns all problems found. An empty slice means the model is sound.
func Validate(model *Model) []error {
	errs := ValidateConnections(model)
	errs = append(errs, ValidateRanges(model)...)
	return errs
}

func ValidateConnections(model *Model) []error {
	var errs []error
	for _, frame := range model.Frames {
		if len(frame.Sources) == 0 {
			continue
		}
		allowed, target, expected := allowedSources(frame.EntityType)
		for _, source := range frame.Sources {
			if !allowed[source.EntityType] {
				errs = append(errs, fmt.Errorf("a %s can only receive input from a %s, not from %q", target, expected, source.EntityType))
			}
		}
	}
	return errs
}

// ValidateRanges checks chapters and slices: each range must run forwards
// in declaration order, chapters must not overlap one another, and slices
// must not straddle a chapter boundary.
func ValidateRanges(model *Model) []error {
	var errs []error
	for _, ch := range model.Chapters {
		if ch.Start == nil || ch.End == nil {
			continue
		}
		if ch.Start.DeclarationIx > ch.End.DeclarationIx {
			errs = append(errs, fmt.Errorf("chapter %q: start frame %s is declared after end frame %s", ch.Name, ch.StartID, ch.EndID))
		}
	}
	for i, a := range model.Chapters {
		for _, b := range model.Chapters[i+1:] {
			if a.Start == nil || a.End == nil || b.Start == nil || b.End == nil {
				continue
			}
			if a.Start.DeclarationIx <= b.End.DeclarationIx && b.Start.DeclarationIx <= a.End.DeclarationIx {
				errs = append(errs, fmt.Errorf("chapters %q (%s-%s) and %q (%s-%s) overlap", a.Name, a.StartID, a.EndID, b.Name, b.StartID, b.EndID))
			}
		}
	}
	for _, sl := range model.Slices {
		if sl.Start == nil || sl.End == nil {
			continue
		}
		if sl.Start.DeclarationIx > sl.End.DeclarationIx {
			errs = append(errs, fmt.Errorf("slice %q: start frame %s is declared after end frame %s", sl.Name, sl.StartID, sl.EndID))
			continue
		}
		for _, ch := range model.Chapters {
			if ch.Start == nil || ch.End == nil {
				continue
			}
			inStart := ch.Contains(sl.Start)
			inEnd := ch.Contains(sl.End)
			if inStart != inEnd {
				errs = append(errs, fmt.Errorf("slice %q (%s-%s) crosses the boundary of chapter %q (%s-%s)", sl.Name, sl.StartID, sl.EndID, ch.Name, ch.StartID, ch.EndID))
			}
		}
	}
	return errs
}

func allowedSources(target EntityType) (map[EntityType]bool, string, string) {
	switch target {
	case EntityCommand:
		return map[EntityType]bool{EntityUI: true, EntityProcessor: true}, "command", "ui or processor"
	case EntityEvent:
		return map[EntityType]bool{EntityCommand: true}, "event", "command"
	case EntityReadModel:
		return map[EntityType]bool{EntityEvent: true}, "read model", "event"
	case EntityProcessor:
		return map[EntityType]bool{EntityEvent: true, EntityReadModel: true}, "processor", "event or read model"
	case EntityUI:
		return map[EntityType]bool{EntityReadModel: true}, "ui", "read model"
	default:
		return map[EntityType]bool{}, "frame", "known source"
	}
}

// LintFinding is one advisory problem: not a parse or validation error, but
// something a facilitator should resolve before calling the model finished.
type LintFinding struct {
	Kind    string // "hotspot", "uncovered-command", "orphan-processor", "sent-without-response"
	FrameID string
	Message string
}

func (f LintFinding) String() string {
	if f.FrameID != "" {
		return fmt.Sprintf("[%s] frame %s: %s", f.Kind, f.FrameID, f.Message)
	}
	return fmt.Sprintf("[%s] %s", f.Kind, f.Message)
}

// Lint reports open hotspots and common completeness gaps drawn from the
// SKILL.md checklist. Findings never block rendering; `evml lint --strict`
// turns them into a non-zero exit so they can gate a merge.
func Lint(model *Model) []LintFinding {
	var out []LintFinding
	for _, h := range model.Hotspots {
		first := strings.TrimSpace(strings.Split(StripOuterBraces(h.Value), "\n")[0])
		out = append(out, LintFinding{Kind: "hotspot", FrameID: h.SourceID, Message: "open question: " + first})
	}
	gwtByFrame := map[string]int{}
	for _, g := range model.GWTs {
		gwtByFrame[g.SourceID]++
	}
	for idx, f := range model.Frames {
		switch f.EntityType {
		case EntityCommand:
			if model.FrameStage(f) == StageFuture {
				continue
			}
			if gwtByFrame[f.ID] == 0 {
				out = append(out, LintFinding{Kind: "uncovered-command", FrameID: f.ID, Message: fmt.Sprintf("command %s has no given/when/then scenario", f.Identifier)})
			}
			if !followedBy(model, idx, EntityEvent) {
				out = append(out, LintFinding{Kind: "command-without-event", FrameID: f.ID, Message: fmt.Sprintf("command %s is not followed by an event", f.Identifier)})
			}
		case EntityProcessor:
			if len(f.Sources) == 0 && idx == 0 {
				out = append(out, LintFinding{Kind: "orphan-processor", FrameID: f.ID, Message: fmt.Sprintf("processor %s has nothing to react to", f.Identifier)})
			}
		case EntityEvent:
			if strings.HasSuffix(f.Identifier, "Sent") && !hasResponseFor(model, f) {
				out = append(out, LintFinding{Kind: "sent-without-response", FrameID: f.ID, Message: fmt.Sprintf("%s has no Accepted/Declined/Responded outcome event", f.Identifier)})
			}
		}
	}
	return out
}

func followedBy(model *Model, idx int, want EntityType) bool {
	for i := idx + 1; i < len(model.Frames) && i <= idx+2; i++ {
		if model.Frames[i].EntityType == want {
			return true
		}
	}
	return false
}

func hasResponseFor(model *Model, sent *Frame) bool {
	base := strings.TrimSuffix(sent.Identifier, "Sent")
	if i := strings.LastIndex(base, "."); i >= 0 {
		base = base[i+1:]
	}
	for _, f := range model.Frames {
		if f.EntityType != EntityEvent || f == sent {
			continue
		}
		name := f.Identifier
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if strings.HasPrefix(name, base) && name != sent.Identifier {
			return true
		}
	}
	for _, g := range model.GWTs {
		for _, t := range g.Then {
			if strings.HasPrefix(t.Identifier, base) && t.Identifier != sent.Identifier {
				return true
			}
		}
	}
	return false
}
