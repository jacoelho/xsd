package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

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
