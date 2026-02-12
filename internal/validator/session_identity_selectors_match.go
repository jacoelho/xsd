package validator

import "github.com/jacoelho/xsd/internal/runtime"

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
