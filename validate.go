package evml

import (
	"fmt"
	"strconv"
)

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
	if errs2 := ValidateRanges(model); len(errs2) > 0 {
		errs = append(errs, errs2...)
	}
	return errs
}

// ValidateRanges checks that every chapter and every slice is a valid
// forward-going frame-ID range, and that chapters do not overlap one
// another (slices are intentionally allowed to overlap — they model
// build-stage decomposition across the timeline). Chapter and slice
// start/end frames are already known to exist (resolveReferences in
// parse.go enforces that).
func ValidateRanges(model *Model) []error {
	var errs []error
	for _, ch := range model.Chapters {
		startN, endN, ok := rangeOrder(ch.StartID, ch.EndID)
		if !ok {
			errs = append(errs, fmt.Errorf("chapter %q: end frame %s must come at or after start frame %s", ch.Label, ch.EndID, ch.StartID))
			continue
		}
		_ = startN
		_ = endN
	}
	for i, a := range model.Chapters {
		for j, b := range model.Chapters {
			if j <= i {
				continue
			}
			if rangesOverlap(a.StartID, a.EndID, b.StartID, b.EndID) {
				errs = append(errs, fmt.Errorf("chapters %q (%s-%s) and %q (%s-%s) overlap", a.Label, a.StartID, a.EndID, b.Label, b.StartID, b.EndID))
			}
		}
	}
	for _, sl := range model.Slices {
		if _, _, ok := rangeOrder(sl.StartID, sl.EndID); !ok {
			errs = append(errs, fmt.Errorf("slice %q: end frame %s must come at or after start frame %s", sl.Name, sl.EndID, sl.StartID))
		}
	}
	return errs
}

// rangeOrder returns the numeric values of start and end (so the caller
// can compare them); ok=false if either ID fails to parse as a number.
func rangeOrder(startID, endID string) (int, int, bool) {
	s, err1 := strconv.Atoi(startID)
	e, err2 := strconv.Atoi(endID)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if e < s {
		return s, e, false
	}
	return s, e, true
}

func rangesOverlap(aStart, aEnd, bStart, bEnd string) bool {
	as, ae, ok1 := rangeOrder(aStart, aEnd)
	bs, be, ok2 := rangeOrder(bStart, bEnd)
	if !ok1 || !ok2 {
		return false
	}
	return as <= be && bs <= ae
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
