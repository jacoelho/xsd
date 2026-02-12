package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// AcceptModel verifies that the current model state is in an accepting state.
func (s *Session) AcceptModel(ref runtime.ModelRef, state *ModelState) error {
	if s == nil || s.rt == nil {
		return fmt.Errorf("session missing runtime schema")
	}
	if state == nil {
		return fmt.Errorf("model state is nil")
	}
	if state.Kind != ref.Kind {
		return fmt.Errorf("model state kind mismatch")
	}
	switch ref.Kind {
	case runtime.ModelNone:
		return nil
	case runtime.ModelDFA:
		model, err := s.dfaByRef(ref)
		if err != nil {
			return err
		}
		if int(state.DFA) >= len(model.States) {
			return fmt.Errorf("dfa state %d out of range", state.DFA)
		}
		if !model.States[state.DFA].Accept {
			return newValidationErrorWithDetails(
				xsderrors.ErrContentModelInvalid,
				"content model not accepted",
				"",
				s.expectedFromDFAState(model, state.DFA),
			)
		}
		return nil
	case runtime.ModelNFA:
		model, err := s.nfaByRef(ref)
		if err != nil {
			return err
		}
		if bitsetEmpty(state.NFA) {
			if model.Nullable {
				return nil
			}
			return newValidationErrorWithDetails(
				xsderrors.ErrContentModelInvalid,
				"content model not accepted",
				"",
				s.expectedFromNFAStart(model),
			)
		}
		accept, ok := bitsetSlice(model.Bitsets, model.Accept)
		if !ok {
			return fmt.Errorf("nfa accept bitset out of range")
		}
		if !bitsetIntersects(state.NFA, accept) {
			return newValidationErrorWithDetails(
				xsderrors.ErrContentModelInvalid,
				"content model not accepted",
				"",
				s.expectedFromNFAFollow(model, state.NFA),
			)
		}
		return nil
	case runtime.ModelAll:
		model, err := s.allByRef(ref)
		if err != nil {
			return err
		}
		if state.AllCount == 0 && model.MinOccurs == 0 {
			return nil
		}
		for i, member := range model.Members {
			if member.Optional {
				continue
			}
			if !allHas(state.All, i) {
				return newValidationErrorWithDetails(
					xsderrors.ErrRequiredElementMissing,
					"required element missing from all group",
					"",
					s.expectedFromAllRemaining(model, state.All, true),
				)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown model kind %d", ref.Kind)
	}
}
