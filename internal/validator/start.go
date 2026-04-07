package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// RuntimeFrame stores the active identity state for one open element.
type RuntimeFrame struct {
	Captures []FieldCapture
	Matches  []*Match
	ID       uint64
	Depth    int
	Sym      runtime.SymbolID
	NS       runtime.NamespaceID
	Elem     runtime.ElemID
	Type     runtime.TypeID
	Nilled   bool
}

func (f RuntimeFrame) MatchSymbol() runtime.SymbolID {
	return f.Sym
}

func (f RuntimeFrame) MatchNamespace() runtime.NamespaceID {
	return f.NS
}

// StartInput describes one element entry for identity processing.
type StartInput struct {
	Elem   runtime.ElemID
	Type   runtime.TypeID
	Sym    runtime.SymbolID
	NS     runtime.NamespaceID
	Nilled bool
}

// StartFrame pushes one runtime frame, opens rooted scopes, and applies any
// selector/field matches for the current element.
func StartFrame(rt *runtime.Schema, state *State[RuntimeFrame], in StartInput, loadAttrs func() []Attr) error {
	if rt == nil {
		return fmt.Errorf("identity: schema missing")
	}
	elem, ok := elementByID(rt, in.Elem)
	if !ok {
		return fmt.Errorf("identity: element %d not found", in.Elem)
	}
	hasConstraints := elem.ICLen > 0
	if !state.Active && !hasConstraints {
		return nil
	}
	state.Active = true

	state.NextNodeID++
	frame := RuntimeFrame{
		ID:     state.NextNodeID,
		Depth:  state.Frames.Len(),
		Sym:    in.Sym,
		NS:     in.NS,
		Elem:   in.Elem,
		Type:   in.Type,
		Nilled: in.Nilled,
	}
	state.Frames.Push(frame)
	frames := state.Frames.Items()
	current := &frames[len(frames)-1]

	if hasConstraints {
		scope, ok, err := OpenScope(rt, current.ID, current.Depth, current.Elem, elem)
		if err != nil {
			return err
		}
		if ok {
			state.Scopes.Push(scope)
		}
	}
	if state.Scopes.Len() == 0 {
		return nil
	}

	var attrs []Attr
	if loadAttrs != nil {
		attrs = loadAttrs()
	}
	current.Matches = append(current.Matches, MatchSelectors(rt, state.Scopes.Items(), state.Frames.Items(), current.ID, current.Depth)...)
	captures, errs := ApplySelections(rt, state.Scopes.Items(), state.Frames.Items(), current.Depth, current.ID, current.Type, attrs)
	current.Captures = append(current.Captures, captures...)
	state.Uncommitted = append(state.Uncommitted, errs...)
	return nil
}

func elementByID(rt *runtime.Schema, id runtime.ElemID) (*runtime.Element, bool) {
	if rt == nil || id == 0 || int(id) >= len(rt.Elements) {
		return nil, false
	}
	return &rt.Elements[id], true
}
