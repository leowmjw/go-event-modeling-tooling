package evml

import (
	"errors"
	"strings"
	"testing"
)

func TestParseComplexModel(t *testing.T) {
	model, err := Parse(`eventmodeling
tf 01 cmd UpdateCartCommand
tf 02 evt CartUpdatedEvent ->> 01 ` + "`jsobj`" + `{ a: b }
tf 03 rmo CartItemsReadModel ->> 02 [[CartItemsReadModel03]]
tf 04 evt ProductDescriptionUpdatedEvent ->> 01 ` + "`jsobj`" + `{ a: { c: d } }
tf 05 evt ProductTitleUpdatedEvent ->> 01 { "a": { "c": true } }
tf 06 evt ProductCountIncrementedEvent ->> 01 ` + "`json`" + `" { "a": { "c": true } } "

data CartItemsReadModel03 {
  { a: b }
}

data NotAssignedData02 ` + "`jsobj`" + ` {
  { a: {
    d: true
  }}
}

data AnotherNotAssignedData06 {
  a: 'abc'
}

note 02 ` + "`md`" + ` {
    # head 1
    this is markdown note
}

note 05 {
  This is whatever <b>you</b> want
  On multiple lines
}

gwt 01 "user adds item to cart"
  given
    evt CartUpdatedEvent { a: true, b: "abc" }
    evt CartUpdatedEvent
  when
    evt ProductDescriptionUpdatedEvent {
      a: true,
      "b": "hello"
    }
    evt ProductTitleUpdatedEvent
  then
    evt ProductTitleUpdatedEvent

gwt 03 'cart already populated'
  given
    evt CartUpdatedEvent
    evt ProductTitleUpdatedEvent
  then
    evt ProductTitleUpdatedEvent
    evt CartUpdatedEvent
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := len(model.Frames); got != 6 {
		t.Fatalf("len(Frames) = %d, want 6", got)
	}
	if got := len(model.DataEntities); got != 3 {
		t.Fatalf("len(DataEntities) = %d, want 3", got)
	}
	if got := len(model.NoteEntities); got != 2 {
		t.Fatalf("len(NoteEntities) = %d, want 2", got)
	}
	if got := len(model.GWTs); got != 2 {
		t.Fatalf("len(GWTs) = %d, want 2", got)
	}
	if model.GWTs[0].Label != `"user adds item to cart"` {
		t.Fatalf("first GWT label = %q", model.GWTs[0].Label)
	}
	if model.GWTs[1].Label != `'cart already populated'` {
		t.Fatalf("second GWT label = %q", model.GWTs[1].Label)
	}
}

func TestParseGWTWithoutLabel(t *testing.T) {
	model, err := Parse(`eventmodeling
tf 01 evt Start

gwt 01
  given
    evt Start
  then
    evt Start
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(model.GWTs) != 1 || model.GWTs[0].Label != "" {
		t.Fatalf("unexpected gwt label: %+v", model.GWTs)
	}
}

func TestParseSimpleModel(t *testing.T) {
	model, err := Parse(`eventmodeling
timeframe 01 event Start
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(model.Frames) != 1 {
		t.Fatalf("len(Frames) = %d, want 1", len(model.Frames))
	}
	frame := model.Frames[0]
	if frame.ID != "01" || frame.EntityType != EntityEvent || frame.Identifier != "Start" {
		t.Fatalf("unexpected frame: %+v", frame)
	}
}

func TestParseQualifiedNamesAndResetFrames(t *testing.T) {
	model, err := Parse(`eventmodeling

tf 02 ui UI
resetframe 01 evt Product.PriceChanged
tf 03 evt Cart.ItemAdded
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := len(model.Frames); got != 3 {
		t.Fatalf("len(Frames) = %d, want 3", got)
	}
	frame := model.Frames[1]
	if frame.Kind != FrameKindReset || frame.Identifier != "Product.PriceChanged" {
		t.Fatalf("unexpected reset frame: %+v", frame)
	}
}

func TestParseNestedDataBlocksAndInlinePayloads(t *testing.T) {
	model, err := Parse(`eventmodeling
data Bar {
  a: {
    b: {
      c: 1
    },
    d: 2
  }
}

tf 01 evt Start
tf 07 evt X ->> 01 { "a": { "c": "}" }, "b": true }
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := len(model.DataEntities); got != 1 {
		t.Fatalf("len(DataEntities) = %d, want 1", got)
	}
	if got := strings.Count(model.DataEntities[0].Value, "{"); got != 3 {
		t.Fatalf("open brace count = %d, want 3", got)
	}
	if model.Frames[1].Data != `{ "a": { "c": "}" }, "b": true }` {
		t.Fatalf("inline data = %q", model.Frames[1].Data)
	}
}

func TestParseGWTMultilineNestedBlocks(t *testing.T) {
	model, err := Parse(`eventmodeling
tf 01 evt Start
tf 02 evt Done

gwt 01 "nested gwt payloads"
  given
    evt Start ` + "`jsobj`" + ` {
      a: {
        b: {
          c: 1
        },
        d: 2
      }
    }
  when
    evt Done {
      outer: {
        inner: true
      }
    }
  then
    evt Done {
      result: {
        ok: "}"
      }
    }
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	gwt := model.GWTs[0]
	if len(gwt.Given) != 1 || len(gwt.When) != 1 || len(gwt.Then) != 1 {
		t.Fatalf("unexpected gwt statement counts: %+v", gwt)
	}
	if !strings.Contains(gwt.Given[0].Data, "d: 2") || !strings.Contains(gwt.Then[0].Data, `ok: "}"`) {
		t.Fatalf("unexpected gwt data: %+v", gwt)
	}
}

func TestParseRejectsUnbalancedInlinePayload(t *testing.T) {
	_, err := Parse(`eventmodeling
tf 01 evt Start
tf 02 evt Bad ->> 01 { "a": { }
`)
	if err == nil {
		t.Fatal("expected parse error")
	}
	// Go 1.26: errors.AsType[T] replaces the two-step var+errors.As pattern.
	pe, ok := errors.AsType[*ParseError](err)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Line == 0 {
		t.Fatalf("ParseError should carry a non-zero line number, got %d", pe.Line)
	}
}

func TestValidateConnections(t *testing.T) {
	model, err := Parse(`eventmodeling
tf 01 ui UI
tf 02 cmd Make
tf 03 evt Made
tf 04 rmo Read ->> 03
tf 05 pcr Proc ->> 03
tf 06 ui Done ->> 04
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if errs := ValidateConnections(model); len(errs) != 0 {
		t.Fatalf("ValidateConnections() = %v, want no errors", errs)
	}
	invalid, err := Parse(`eventmodeling
tf 01 evt Start
tf 02 cmd Wrong ->> 01
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if errs := ValidateConnections(invalid); len(errs) != 1 {
		t.Fatalf("ValidateConnections() len = %d, want 1", len(errs))
	}
}

func TestParseMultipleSourceFrames(t *testing.T) {
	model, err := Parse(`eventmodeling
tf 01 evt Start
tf 02 evt End
rf 03 readmodel ReadModel01 ->> 01 ->> 02 { a: true }
rf 04 rmo ReadModel02 ->> 01 ->> 02
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(model.Frames) != 4 {
		t.Fatalf("len(Frames) = %d, want 4", len(model.Frames))
	}
	if got := len(model.Frames[2].Sources); got != 2 {
		t.Fatalf("len(Sources) = %d, want 2", got)
	}
	if got := len(model.Frames[3].Sources); got != 2 {
		t.Fatalf("len(Sources) = %d, want 2", got)
	}
}
