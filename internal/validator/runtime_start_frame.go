package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) buildStartFrame(entry nameEntry, ev *xmlstream.ResolvedEvent, result StartResult, typ runtime.Type) (elemFrame, error) {
	frame := elemFrame{
		name:   NameID(ev.NameID),
		elem:   result.Elem,
		typ:    result.Type,
		nilled: result.Nilled,
	}
	if entry.LocalLen == 0 && entry.NSLen == 0 {
		if len(ev.Local) > 0 {
			frame.local = append([]byte(nil), ev.Local...)
		}
		if len(ev.NS) > 0 {
			frame.ns = append([]byte(nil), ev.NS...)
		}
	}

	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		frame.content = runtime.ContentSimple
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			return elemFrame{}, fmt.Errorf("complex type %d missing", result.Type)
		}
		ct := s.rt.ComplexTypes[typ.Complex.ID]
		frame.content = ct.Content
		frame.mixed = ct.Mixed
		frame.model = ct.Model
		if frame.model.Kind != runtime.ModelNone {
			state, err := s.InitModelState(frame.model)
			if err != nil {
				return elemFrame{}, err
			}
			frame.modelState = state
		}
	default:
		return elemFrame{}, fmt.Errorf("unknown type kind %d", typ.Kind)
	}
	return frame, nil
}
