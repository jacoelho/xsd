package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

// InitModelState allocates and initializes model state for a compiled model reference.
func (s *Session) InitModelState(ref runtime.ModelRef) (StartModelState, error) {
	return InitStartModelState(s.rt, ref)
}

// StepModel advances the current content-model state with one element symbol.
func (s *Session) StepModel(ref runtime.ModelRef, state *StartModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
	return StepStartModel(s.rt, ref, state, sym, nsID, nsBytes)
}

// AcceptModel verifies that the current model state is in an accepting state.
func (s *Session) AcceptModel(ref runtime.ModelRef, state *StartModelState) error {
	return AcceptStartModel(s.rt, ref, state)
}
