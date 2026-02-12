package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

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
