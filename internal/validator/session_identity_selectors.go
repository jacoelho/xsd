package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *identityState) matchSelectors(rt *runtime.Schema, currentDepth int) {
	frames := s.frames.Items()
	frame := &frames[currentDepth]
	scopes := s.scopes.Items()
	for scopeIdx := range scopes {
		scope := &scopes[scopeIdx]
		for cidx := range scope.constraints {
			state := &scope.constraints[cidx]
			if _, exists := state.matches[frame.id]; exists {
				continue
			}
			if !matchesAnySelector(rt, state.selectors, frames, scope.rootDepth, currentDepth) {
				continue
			}
			match := &rtSelectorMatch{
				constraint: state,
				id:         frame.id,
				depth:      currentDepth,
				fields:     make([]rtFieldState, len(state.fields)),
			}
			state.matches[frame.id] = match
			frame.matches = append(frame.matches, match)
		}
	}
}

func (s *identityState) applyFieldSelections(rt *runtime.Schema, currentDepth int, attrs []rtIdentityAttr) {
	frames := s.frames.Items()
	frame := &frames[currentDepth]
	scopes := s.scopes.Items()
	for scopeIdx := range scopes {
		scope := &scopes[scopeIdx]
		for cidx := range scope.constraints {
			state := &scope.constraints[cidx]
			for _, match := range state.matches {
				if match.invalid {
					continue
				}
				for fieldIndex := range match.fields {
					fieldState := &match.fields[fieldIndex]
					if fieldState.multiple {
						continue
					}
					for _, pathID := range state.fields[fieldIndex] {
						ops, ok := pathOps(rt, pathID)
						if !ok {
							s.uncommittedViolations = append(s.uncommittedViolations, fmt.Errorf("identity: path %d out of range", pathID))
							continue
						}
						elemOps, attrOp, hasAttr := splitAttrOp(ops)
						if !matchProgramPath(elemOps, frames, match.depth, currentDepth) {
							continue
						}
						if hasAttr {
							s.applyAttributeSelection(fieldState, attrOp, attrs, frame, rt)
						} else {
							s.applyElementSelection(fieldState, frame, match, fieldIndex, rt)
						}
						if fieldState.multiple {
							break
						}
					}
				}
			}
		}
	}
}

func (s *identityState) applyElementSelection(state *rtFieldState, frame *rtIdentityFrame, match *rtSelectorMatch, fieldIndex int, rt *runtime.Schema) {
	if state.multiple {
		return
	}
	key := rtFieldNodeKey{kind: rtFieldNodeElement, elemID: frame.id}
	if !state.addNode(key) {
		return
	}
	if state.count > 1 {
		state.multiple = true
		return
	}
	if !isSimpleContent(rt, frame.typ) {
		state.invalid = true
		return
	}
	frame.captures = append(frame.captures, rtFieldCapture{match: match, fieldIndex: fieldIndex})
}

func (s *identityState) applyAttributeSelection(state *rtFieldState, op runtime.PathOp, attrs []rtIdentityAttr, frame *rtIdentityFrame, rt *runtime.Schema) {
	if state.multiple {
		return
	}
	switch op.Op {
	case runtime.OpAttrAny:
		for i := range attrs {
			attr := &attrs[i]
			if isXMLNSAttr(attr, rt) {
				continue
			}
			s.addAttributeValue(state, frame.id, attr)
			if state.multiple {
				return
			}
		}
	case runtime.OpAttrNSAny:
		for i := range attrs {
			attr := &attrs[i]
			if isXMLNSAttr(attr, rt) {
				continue
			}
			if !attrNamespaceMatches(attr, op.NS, rt) {
				continue
			}
			s.addAttributeValue(state, frame.id, attr)
			if state.multiple {
				return
			}
		}
	case runtime.OpAttrName:
		for i := range attrs {
			attr := &attrs[i]
			if isXMLNSAttr(attr, rt) {
				continue
			}
			if attrNameMatches(attr, op, rt) {
				s.addAttributeValue(state, frame.id, attr)
				return
			}
		}
	default:
		s.uncommittedViolations = append(s.uncommittedViolations, fmt.Errorf("identity: unknown attribute op %d", op.Op))
	}
}

func (s *identityState) addAttributeValue(state *rtFieldState, elemID uint64, attr *rtIdentityAttr) {
	if attr == nil {
		return
	}
	if state.multiple {
		return
	}
	key := rtFieldNodeKey{
		kind:       rtFieldNodeAttribute,
		elemID:     elemID,
		attrSym:    attr.sym,
		attrNameID: attr.nameID,
	}
	if !state.addNode(key) {
		return
	}
	if state.count > 1 {
		state.multiple = true
		return
	}
	if attr.keyKind == runtime.VKInvalid {
		state.invalid = true
		return
	}
	state.keyKind = attr.keyKind
	state.keyBytes = append(state.keyBytes[:0], attr.keyBytes...)
	state.hasValue = true
}
