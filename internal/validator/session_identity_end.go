package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *identityState) end(rt *runtime.Schema, in identityEndInput) error {
	if rt == nil || !s.Active || s.Frames.Len() == 0 {
		return nil
	}
	frames := s.Frames.Items()
	index := len(frames) - 1
	frame := &frames[index]
	if err := CloseFrame(rt, s.arena, &s.State, frame.ID, frame.Elem, frame.Nilled, frame.Captures, frame.Matches, in.KeyKind, in.KeyBytes); err != nil {
		return err
	}

	s.Frames.Pop()
	if s.Frames.Len() == 0 && s.Scopes.Len() == 0 {
		s.Active = false
	}
	return nil
}
