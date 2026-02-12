package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// StepModel advances the current content-model state with one element symbol.
func (s *Session) StepModel(ref runtime.ModelRef, state *ModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
	if s == nil || s.rt == nil {
		return StartMatch{}, fmt.Errorf("session missing runtime schema")
	}
	if state == nil {
		return StartMatch{}, fmt.Errorf("model state is nil")
	}
	if state.Kind != ref.Kind {
		return StartMatch{}, fmt.Errorf("model state kind mismatch")
	}
	switch ref.Kind {
	case runtime.ModelNone:
		return StartMatch{}, noContentModelMatchError()
	case runtime.ModelDFA:
		model, err := s.dfaByRef(ref)
		if err != nil {
			return StartMatch{}, err
		}
		return s.stepDFA(model, state, sym, nsID, nsBytes)
	case runtime.ModelNFA:
		model, err := s.nfaByRef(ref)
		if err != nil {
			return StartMatch{}, err
		}
		return s.stepNFA(model, state, sym, nsID, nsBytes)
	case runtime.ModelAll:
		model, err := s.allByRef(ref)
		if err != nil {
			return StartMatch{}, err
		}
		return s.stepAll(model, state, sym)
	default:
		return StartMatch{}, fmt.Errorf("unknown model kind %d", ref.Kind)
	}
}
