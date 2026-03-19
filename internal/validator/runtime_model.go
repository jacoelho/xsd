package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/model"
)

// InitModelState allocates and initializes model state for a compiled model reference.
func (s *Session) InitModelState(ref runtime.ModelRef) (model.State, error) {
	return model.Init(s.rt, ref)
}

// StepModel advances the current content-model state with one element symbol.
func (s *Session) StepModel(ref runtime.ModelRef, state *model.State, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (model.Match, error) {
	return model.Step(s.rt, ref, state, sym, nsID, nsBytes)
}

// AcceptModel verifies that the current model state is in an accepting state.
func (s *Session) AcceptModel(ref runtime.ModelRef, state *model.State) error {
	return model.Accept(s.rt, ref, state)
}
