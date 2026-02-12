package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

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
