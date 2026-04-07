package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestStartFrameBuildsRuntimeState(t *testing.T) {
	fx := buildSelectionFixture(t)
	var state State[RuntimeFrame]

	if err := StartFrame(fx.rt, &state, StartInput{
		Elem: fx.rootElem,
		Type: fx.rt.Elements[fx.rootElem].Type,
		Sym:  fx.rt.Elements[fx.rootElem].Name,
		NS:   fx.nsID,
	}, nil); err != nil {
		t.Fatalf("StartFrame(root): %v", err)
	}
	if !state.Active {
		t.Fatalf("state.Active = false, want true")
	}
	if got := state.Frames.Len(); got != 1 {
		t.Fatalf("frames len = %d, want 1", got)
	}
	if got := state.Scopes.Len(); got != 1 {
		t.Fatalf("scopes len = %d, want 1", got)
	}

	if err := StartFrame(fx.rt, &state, StartInput{
		Elem: 2,
		Type: fx.itemType,
		Sym:  fx.itemSym,
		NS:   fx.nsID,
	}, func() []Attr {
		return []Attr{{
			Sym:      fx.attrSym,
			NS:       fx.emptyNS,
			KeyKind:  runtime.VKString,
			KeyBytes: []byte("one"),
		}}
	}); err != nil {
		t.Fatalf("StartFrame(item): %v", err)
	}

	if got := state.Frames.Len(); got != 2 {
		t.Fatalf("frames len = %d, want 2", got)
	}
	frame := state.Frames.Items()[1]
	if frame.ID != 2 {
		t.Fatalf("frame.ID = %d, want 2", frame.ID)
	}
	if len(frame.Matches) != 1 {
		t.Fatalf("frame matches = %d, want 1", len(frame.Matches))
	}
	if len(frame.Captures) != 0 {
		t.Fatalf("frame captures = %d, want 0", len(frame.Captures))
	}

	match := state.Scopes.Items()[0].Constraints[0].Matches[frame.ID]
	if match == nil {
		t.Fatalf("expected selector match to be registered")
	}
	field := match.Fields[0]
	if !field.HasValue {
		t.Fatalf("field.HasValue = false, want true")
	}
	if got := string(field.KeyBytes); got != "one" {
		t.Fatalf("field.KeyBytes = %q, want %q", got, "one")
	}
}

func TestStartFrameSkipsInactiveElementsWithoutConstraints(t *testing.T) {
	fx := buildSelectionFixture(t)
	var state State[RuntimeFrame]
	called := false

	if err := StartFrame(fx.rt, &state, StartInput{
		Elem:   2,
		Type:   fx.itemType,
		Sym:    fx.itemSym,
		NS:     fx.nsID,
		Nilled: false,
	}, func() []Attr {
		called = true
		return []Attr{{Sym: fx.attrSym}}
	}); err != nil {
		t.Fatalf("StartFrame(item): %v", err)
	}

	if state.Active {
		t.Fatalf("state.Active = true, want false")
	}
	if got := state.Frames.Len(); got != 0 {
		t.Fatalf("frames len = %d, want 0", got)
	}
	if got := state.NextNodeID; got != 0 {
		t.Fatalf("next node id = %d, want 0", got)
	}
	if called {
		t.Fatalf("attr loader called for skipped frame")
	}
}
