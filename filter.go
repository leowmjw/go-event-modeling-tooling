package evml

// FilterStages returns a copy of model containing only frames whose
// effective stage is in keep, plus the notes, hotspots, scenarios, chapters
// and slices that still refer to surviving frames. Explicit ->> sources
// pointing at dropped frames are removed so the renderer falls back to
// inference; a nil or empty keep returns the model unchanged.
//
// This powers the "lens" a facilitator uses in a session: show the as-is
// process alone, layer the staging proposals on top, or show everything
// including future goals.
func FilterStages(model *Model, keep ...Stage) *Model {
	if model == nil || len(keep) == 0 {
		return model
	}
	allowed := map[Stage]bool{}
	for _, s := range keep {
		allowed[s] = true
	}
	if allowed[StageCurrent] && allowed[StageStaging] && allowed[StageFuture] {
		return model
	}

	out := &Model{
		DataEntities: model.DataEntities,
		Entities:     model.Entities,
		Actors:       model.Actors,
	}
	kept := map[string]*Frame{}
	for _, f := range model.Frames {
		if !allowed[model.FrameStage(f)] {
			continue
		}
		c := *f
		c.DeclarationIx = len(out.Frames)
		c.Sources = nil
		c.SourceIDs = nil
		out.Frames = append(out.Frames, &c)
		kept[c.ID] = &c
	}
	for _, f := range model.Frames {
		c, ok := kept[f.ID]
		if !ok {
			continue
		}
		for _, id := range f.SourceIDs {
			if src, ok := kept[id]; ok {
				c.SourceIDs = append(c.SourceIDs, id)
				c.Sources = append(c.Sources, src)
			}
		}
	}
	for _, n := range model.NoteEntities {
		if src, ok := kept[n.SourceID]; ok {
			c := *n
			c.Source = src
			out.NoteEntities = append(out.NoteEntities, &c)
		}
	}
	for _, h := range model.Hotspots {
		if src, ok := kept[h.SourceID]; ok {
			c := *h
			c.Source = src
			out.Hotspots = append(out.Hotspots, &c)
		}
	}
	for _, g := range model.GWTs {
		if src, ok := kept[g.SourceID]; ok {
			c := *g
			c.Source = src
			out.GWTs = append(out.GWTs, &c)
		}
	}
	for _, ch := range model.Chapters {
		if start, end, ok := clampRange(model, kept, ch.Start, ch.End); ok {
			c := *ch
			c.Start, c.End = start, end
			c.StartID, c.EndID = start.ID, end.ID
			out.Chapters = append(out.Chapters, &c)
		}
	}
	for _, sl := range model.Slices {
		if start, end, ok := clampRange(model, kept, sl.Start, sl.End); ok {
			c := *sl
			c.Start, c.End = start, end
			c.StartID, c.EndID = start.ID, end.ID
			out.Slices = append(out.Slices, &c)
		}
	}
	return out
}

// clampRange shrinks a declaration-order range to the surviving frames
// inside it; ok is false when nothing inside the range survived.
func clampRange(model *Model, kept map[string]*Frame, start, end *Frame) (*Frame, *Frame, bool) {
	if start == nil || end == nil {
		return nil, nil, false
	}
	var first, last *Frame
	for _, f := range model.Frames {
		if f.DeclarationIx < start.DeclarationIx || f.DeclarationIx > end.DeclarationIx {
			continue
		}
		if k, ok := kept[f.ID]; ok {
			if first == nil {
				first = k
			}
			last = k
		}
	}
	if first == nil {
		return nil, nil, false
	}
	return first, last, true
}
