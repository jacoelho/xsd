package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/names"
	"github.com/jacoelho/xsd/internal/validator/start"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) buildStartFrame(entry names.Entry, ev *xmlstream.ResolvedEvent, result start.Result, typ runtime.Type) (elemFrame, error) {
	plan, err := start.PlanFrame(
		start.NameInput{
			Local:  ev.Local,
			NS:     ev.NS,
			Cached: entry.LocalLen != 0 || entry.NSLen != 0,
		},
		result,
		typ,
		s.rt.ComplexTypes,
	)
	if err != nil {
		return elemFrame{}, err
	}

	frame := elemFrame{
		local:   plan.Local,
		ns:      plan.NS,
		model:   plan.Model,
		name:    names.ID(ev.NameID),
		elem:    result.Elem,
		typ:     result.Type,
		content: plan.Content,
		mixed:   plan.Mixed,
		nilled:  result.Nilled,
	}
	if frame.model.Kind != runtime.ModelNone {
		state, err := s.InitModelState(frame.model)
		if err != nil {
			return elemFrame{}, err
		}
		frame.modelState = state
	}

	return frame, nil
}
