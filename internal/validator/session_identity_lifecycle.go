package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *identityState) reset() {
	if s == nil {
		return
	}
	s.active = false
	s.nextNodeID = 0
	s.frames.Reset()
	s.scopes.Reset()
	s.uncommittedViolations = s.uncommittedViolations[:0]
	s.committedViolations = s.committedViolations[:0]
}

func (s *identityState) checkpoint() identitySnapshot {
	if s == nil {
		return identitySnapshot{}
	}
	return identitySnapshot{
		nextNodeID:  s.nextNodeID,
		framesLen:   s.frames.Len(),
		scopesLen:   s.scopes.Len(),
		uncommitted: len(s.uncommittedViolations),
		committed:   len(s.committedViolations),
		active:      s.active,
	}
}

func (s *identityState) rollback(snapshot identitySnapshot) {
	if s == nil {
		return
	}
	for s.frames.Len() > snapshot.framesLen {
		s.frames.Pop()
	}
	for s.scopes.Len() > snapshot.scopesLen {
		s.scopes.Pop()
	}
	if snapshot.uncommitted <= len(s.uncommittedViolations) {
		s.uncommittedViolations = s.uncommittedViolations[:snapshot.uncommitted]
	}
	if snapshot.committed <= len(s.committedViolations) {
		s.committedViolations = s.committedViolations[:snapshot.committed]
	}
	s.nextNodeID = snapshot.nextNodeID
	s.active = snapshot.active
}

func (s *identityState) start(rt *runtime.Schema, in identityStartInput) error {
	if rt == nil {
		return fmt.Errorf("identity: schema missing")
	}
	elem, ok := elementByID(rt, in.Elem)
	if !ok {
		return fmt.Errorf("identity: element %d not found", in.Elem)
	}
	hasConstraints := elem.ICLen > 0
	if !s.active && !hasConstraints {
		return nil
	}
	s.active = true

	s.nextNodeID++
	frame := rtIdentityFrame{
		id:     s.nextNodeID,
		depth:  s.frames.Len(),
		sym:    in.Sym,
		ns:     in.NS,
		elem:   in.Elem,
		typ:    in.Type,
		nilled: in.Nilled,
	}
	s.frames.Push(frame)
	frames := s.frames.Items()
	current := &frames[len(frames)-1]

	if hasConstraints {
		if err := s.openScope(rt, current, elem); err != nil {
			return err
		}
	}
	if s.scopes.Len() == 0 {
		return nil
	}

	s.matchSelectors(rt, current.depth)
	attrs := collectIdentityAttrs(rt, in.Attrs, in.Applied)
	s.applyFieldSelections(rt, current.depth, attrs)
	return nil
}

func (s *identityState) end(rt *runtime.Schema, in identityEndInput) error {
	if rt == nil || !s.active || s.frames.Len() == 0 {
		return nil
	}
	frames := s.frames.Items()
	index := len(frames) - 1
	frame := &frames[index]
	elem, ok := elementByID(rt, frame.elem)
	if !ok {
		return fmt.Errorf("identity: element %d not found", frame.elem)
	}

	s.applyFieldCaptures(frame, elem, in)
	s.finalizeMatches(frame)
	s.closeScopes(frame.id)

	s.frames.Pop()
	if s.frames.Len() == 0 && s.scopes.Len() == 0 {
		s.active = false
	}
	return nil
}

func (s *identityState) openScope(rt *runtime.Schema, frame *rtIdentityFrame, elem *runtime.Element) error {
	if elem == nil {
		return fmt.Errorf("identity: element missing")
	}
	icIDs, err := sliceElemICs(rt, elem)
	if err != nil {
		return err
	}
	if len(icIDs) == 0 {
		return nil
	}
	scope := rtIdentityScope{
		rootID:    frame.id,
		rootDepth: frame.depth,
		rootElem:  frame.elem,
	}
	scope.constraints = make([]rtConstraintState, 0, len(icIDs))
	for _, id := range icIDs {
		if id == 0 || int(id) >= len(rt.ICs) {
			return fmt.Errorf("identity: constraint %d out of range", id)
		}
		constraint := rt.ICs[id]
		selectors, err := slicePathIDs(rt.ICSelectors, constraint.SelectorOff, constraint.SelectorLen)
		if err != nil {
			return err
		}
		fieldsFlat, err := slicePathIDs(rt.ICFields, constraint.FieldOff, constraint.FieldLen)
		if err != nil {
			return err
		}
		fields, err := splitFieldPaths(fieldsFlat)
		if err != nil {
			return err
		}
		scope.constraints = append(scope.constraints, rtConstraintState{
			id:         id,
			name:       constraintName(rt, constraint.Name),
			category:   constraint.Category,
			referenced: constraint.Referenced,
			selectors:  selectors,
			fields:     fields,
			matches:    make(map[uint64]*rtSelectorMatch),
		})
	}
	s.scopes.Push(scope)
	return nil
}

func constraintName(rt *runtime.Schema, sym runtime.SymbolID) string {
	if rt == nil || sym == 0 {
		return ""
	}
	if int(sym) >= len(rt.Symbols.NS) {
		return ""
	}
	local := rt.Symbols.LocalBytes(sym)
	if len(local) == 0 {
		return ""
	}
	nsID := rt.Symbols.NS[sym]
	ns := rt.Namespaces.Bytes(nsID)
	if len(ns) == 0 {
		return string(local)
	}
	return "{" + string(ns) + "}" + string(local)
}
