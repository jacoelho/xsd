package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

// InitModelState allocates and initializes model state for a compiled model reference.
func (s *Session) InitModelState(ref runtime.ModelRef) (StartModelState, error) {
	if s == nil {
		return InitStartModelState(nil, ref)
	}
	if s.rt == nil {
		return StartModelState{}, schemaNotLoadedError()
	}
	switch ref.Kind {
	case runtime.ModelNone:
		return StartModelState{Kind: runtime.ModelNone}, nil
	case runtime.ModelDFA:
		model, err := dfaByRef(s.rt, ref)
		if err != nil {
			return StartModelState{}, err
		}
		return StartModelState{Kind: runtime.ModelDFA, DFA: model.Start}, nil
	case runtime.ModelNFA:
		model, err := nfaByRef(s.rt, ref)
		if err != nil {
			return StartModelState{}, err
		}
		size := int(model.Start.Len)
		return StartModelState{
			Kind:       runtime.ModelNFA,
			NFA:        s.buffers.modelWordSlice(size),
			nfaScratch: s.buffers.modelWordSlice(size),
		}, nil
	case runtime.ModelAll:
		model, err := allByRef(s.rt, ref)
		if err != nil {
			return StartModelState{}, err
		}
		size := (len(model.Members) + 63) / 64
		return StartModelState{
			Kind: runtime.ModelAll,
			All:  s.buffers.modelWordSlice(size),
		}, nil
	default:
		return StartModelState{}, unknownModelKindError(ref.Kind)
	}
}

// StepModel advances the current content-model state with one element symbol.
func (s *Session) StepModel(ref runtime.ModelRef, state *StartModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
	return StepStartModel(s.rt, ref, state, sym, nsID, nsBytes)
}

// AcceptModel verifies that the current model state is in an accepting state.
func (s *Session) AcceptModel(ref runtime.ModelRef, state *StartModelState) error {
	return AcceptStartModel(s.rt, ref, state)
}
