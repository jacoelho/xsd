package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

// OpenScope builds one identity-constraint scope rooted at the current element.
func OpenScope(rt *runtime.Schema, frameID uint64, frameDepth int, elemID runtime.ElemID, elem *runtime.Element) (Scope, bool, error) {
	if elem == nil {
		return Scope{}, false, fmt.Errorf("identity: element missing")
	}
	icIDs, err := sliceElemICs(rt, elem)
	if err != nil {
		return Scope{}, false, err
	}
	if len(icIDs) == 0 {
		return Scope{}, false, nil
	}

	scope := Scope{
		RootID:      frameID,
		RootDepth:   frameDepth,
		RootElem:    elemID,
		Constraints: make([]ConstraintState, 0, len(icIDs)),
	}
	for _, id := range icIDs {
		constraint, ok := rt.IdentityConstraint(id)
		if !ok {
			return Scope{}, false, fmt.Errorf("identity: constraint %d out of range", id)
		}
		selectors, err := slicePathIDs(rt.IdentitySelectors(), constraint.SelectorOff, constraint.SelectorLen)
		if err != nil {
			return Scope{}, false, err
		}
		fieldsFlat, err := slicePathIDs(rt.IdentityFields(), constraint.FieldOff, constraint.FieldLen)
		if err != nil {
			return Scope{}, false, err
		}
		fields, err := splitFieldPaths(fieldsFlat)
		if err != nil {
			return Scope{}, false, err
		}
		scope.Constraints = append(scope.Constraints, ConstraintState{
			ID:         id,
			Name:       constraintName(rt, constraint.Name),
			Category:   constraint.Category,
			Referenced: constraint.Referenced,
			Selectors:  selectors,
			Fields:     fields,
			Matches:    make(map[uint64]*Match),
		})
	}
	return scope, true, nil
}

// MatchSelectors starts any selector matches rooted at the current frame.
func MatchSelectors[F Frame](rt *runtime.Schema, scopes []Scope, frames []F, frameID uint64, currentDepth int, dst []*Match) []*Match {
	if currentDepth < 0 || currentDepth >= len(frames) {
		return dst
	}
	for scopeIdx := range scopes {
		scope := &scopes[scopeIdx]
		for cidx := range scope.Constraints {
			state := &scope.Constraints[cidx]
			if _, exists := state.Matches[frameID]; exists {
				continue
			}
			if !MatchesAnySelector(rt, state.Selectors, frames, scope.RootDepth, currentDepth) {
				continue
			}
			match := &Match{
				Constraint: state,
				ID:         frameID,
				Depth:      currentDepth,
			}
			if len(state.Fields) <= len(match.fields) {
				match.Fields = match.fields[:len(state.Fields)]
			} else {
				match.Fields = make([]FieldState, len(state.Fields))
			}
			state.Matches[frameID] = match
			dst = append(dst, match)
		}
	}
	return dst
}

// ApplySelections evaluates field paths for the current frame and returns any
// deferred element-value captures plus internal invariant errors.
func ApplySelections[F Frame](rt *runtime.Schema, scopes []Scope, frames []F, currentDepth int, frameID uint64, frameType runtime.TypeID, attrs []Attr, captures []FieldCapture) ([]FieldCapture, []error) {
	if currentDepth < 0 || currentDepth >= len(frames) {
		return captures, nil
	}

	var errs []error

	for scopeIdx := range scopes {
		scope := &scopes[scopeIdx]
		for cidx := range scope.Constraints {
			state := &scope.Constraints[cidx]
			for _, match := range state.Matches {
				if match.Invalid {
					continue
				}
				for fieldIndex := range match.Fields {
					fieldState := &match.Fields[fieldIndex]
					if fieldState.Multiple {
						continue
					}
					for _, pathID := range state.Fields[fieldIndex] {
						ops, ok := PathOps(rt, pathID)
						if !ok {
							errs = append(errs, fmt.Errorf("identity: path %d out of range", pathID))
							continue
						}
						elemOps, attrOp, hasAttr := SplitAttrOp(ops)
						if !MatchProgramPath(elemOps, frames, match.Depth, currentDepth) {
							continue
						}
						if hasAttr {
							if err := applyAttributeSelection(fieldState, attrOp, attrs, frameID, rt); err != nil {
								errs = append(errs, err)
							}
						} else if capture, ok := applyElementSelection(fieldState, frameID, frameType, match, fieldIndex, rt); ok {
							captures = append(captures, capture)
						}
						if fieldState.Multiple {
							break
						}
					}
				}
			}
		}
	}

	return captures, errs
}

func applyAttributeSelection(state *FieldState, op runtime.PathOp, attrs []Attr, elemID uint64, rt *runtime.Schema) error {
	if state.Multiple {
		return nil
	}
	switch op.Op {
	case runtime.OpAttrAny:
		for i := range attrs {
			attr := &attrs[i]
			if IsXMLNSAttr(attr, rt) {
				continue
			}
			addAttributeValue(state, elemID, attr)
			if state.Multiple {
				return nil
			}
		}
		return nil
	case runtime.OpAttrNSAny:
		for i := range attrs {
			attr := &attrs[i]
			if IsXMLNSAttr(attr, rt) {
				continue
			}
			if !AttrNamespaceMatches(attr, op.NS, rt) {
				continue
			}
			addAttributeValue(state, elemID, attr)
			if state.Multiple {
				return nil
			}
		}
		return nil
	case runtime.OpAttrName:
		for i := range attrs {
			attr := &attrs[i]
			if IsXMLNSAttr(attr, rt) {
				continue
			}
			if AttrNameMatches(attr, op, rt) {
				addAttributeValue(state, elemID, attr)
				return nil
			}
		}
		return nil
	default:
		return fmt.Errorf("identity: unknown attribute op %d", op.Op)
	}
}

func addAttributeValue(state *FieldState, elemID uint64, attr *Attr) {
	if attr == nil || state.Multiple {
		return
	}
	key := FieldNodeKey{
		Kind:       FieldNodeAttribute,
		ElemID:     elemID,
		AttrSym:    attr.Sym,
		AttrNameID: attr.NameID,
	}
	if !state.AddNode(key) {
		return
	}
	if state.Count > 1 {
		state.Multiple = true
		return
	}
	if attr.KeyKind == runtime.VKInvalid {
		state.Invalid = true
		return
	}
	state.KeyKind = attr.KeyKind
	state.KeyBytes = append(state.KeyBytes[:0], attr.KeyBytes...)
	state.HasValue = true
}

func applyElementSelection(state *FieldState, elemID uint64, frameType runtime.TypeID, match *Match, fieldIndex int, rt *runtime.Schema) (FieldCapture, bool) {
	if state.Multiple {
		return FieldCapture{}, false
	}
	key := FieldNodeKey{Kind: FieldNodeElement, ElemID: elemID}
	if !state.AddNode(key) {
		return FieldCapture{}, false
	}
	if state.Count > 1 {
		state.Multiple = true
		return FieldCapture{}, false
	}
	if !isSimpleContent(rt, frameType) {
		state.Invalid = true
		return FieldCapture{}, false
	}
	return FieldCapture{Match: match, FieldIndex: fieldIndex}, true
}

func constraintName(rt *runtime.Schema, sym runtime.SymbolID) string {
	if rt == nil {
		return ""
	}
	nsID, local, ok := rt.SymbolBytes(sym)
	if !ok || len(local) == 0 {
		return ""
	}
	ns := rt.NamespaceBytes(nsID)
	if len(ns) == 0 {
		return string(local)
	}
	return "{" + string(ns) + "}" + string(local)
}

func isSimpleContent(rt *runtime.Schema, typeID runtime.TypeID) bool {
	typ, ok := rt.Type(typeID)
	if !ok {
		return false
	}
	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		return true
	case runtime.TypeComplex:
		ct, ok := rt.ComplexType(typ.Complex.ID)
		if !ok {
			return false
		}
		return ct.Content == runtime.ContentSimple
	default:
		return false
	}
}

func sliceElemICs(rt *runtime.Schema, elem *runtime.Element) ([]runtime.ICID, error) {
	if elem == nil {
		return nil, fmt.Errorf("identity: element missing")
	}
	if elem.ICLen == 0 {
		return nil, nil
	}
	ids := rt.ElementIdentityConstraintIDs(*elem)
	if ids == nil {
		return nil, fmt.Errorf("identity: elem ICs out of range")
	}
	return ids, nil
}

func slicePathIDs(list []runtime.PathID, off, ln uint32) ([]runtime.PathID, error) {
	if ln == 0 {
		return nil, fmt.Errorf("identity: empty path list")
	}
	start, end, ok := checkedSpan(off, ln, len(list))
	if !ok {
		return nil, fmt.Errorf("identity: path list out of range")
	}
	return list[start:end], nil
}

func splitFieldPaths(ids []runtime.PathID) ([][]runtime.PathID, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("identity: field paths empty")
	}
	if !slices.Contains(ids, 0) {
		return [][]runtime.PathID{slices.Clone(ids)}, nil
	}
	fields := make([][]runtime.PathID, 0, len(ids))
	cur := make([]runtime.PathID, 0, 4)
	for _, id := range ids {
		if id == 0 {
			if len(cur) == 0 {
				return nil, fmt.Errorf("identity: empty field path set")
			}
			fields = append(fields, cur)
			cur = make([]runtime.PathID, 0, 4)
			continue
		}
		cur = append(cur, id)
	}
	if len(cur) == 0 {
		return nil, fmt.Errorf("identity: trailing field separator")
	}
	fields = append(fields, cur)
	return fields, nil
}
