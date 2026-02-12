package validator

import "github.com/jacoelho/xsd/internal/runtime"

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
