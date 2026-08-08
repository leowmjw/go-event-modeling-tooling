package evml

import "fmt"

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

func allowedSources(target EntityType) (map[EntityType]bool, string, string) {
	switch target {
	case EntityCommand:
		return map[EntityType]bool{EntityUI: true, EntityProcessor: true}, "command", "ui or processor"
	case EntityEvent:
		return map[EntityType]bool{EntityCommand: true}, "event", "command"
	case EntityReadModel:
		return map[EntityType]bool{EntityEvent: true}, "read model", "event"
	case EntityProcessor:
		return map[EntityType]bool{EntityReadModel: true}, "processor", "read model"
	case EntityUI:
		return map[EntityType]bool{EntityReadModel: true}, "ui", "read model"
	default:
		return map[EntityType]bool{}, "frame", "known source"
	}
}
