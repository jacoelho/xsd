package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

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
