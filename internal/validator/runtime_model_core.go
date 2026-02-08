package validator

import (
	"fmt"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ModelState tracks the runtime state of a compiled content model.
type ModelState struct {
	NFA        []uint64
	nfaScratch []uint64
	All        []uint64
	DFA        uint32
	AllCount   uint32
	Kind       runtime.ModelKind
}

func (s *Session) InitModelState(ref runtime.ModelRef) (ModelState, error) {
	if s == nil || s.rt == nil {
		return ModelState{}, fmt.Errorf("session missing runtime schema")
	}
	switch ref.Kind {
	case runtime.ModelNone:
		return ModelState{Kind: runtime.ModelNone}, nil
	case runtime.ModelDFA:
		model, err := s.dfaByRef(ref)
		if err != nil {
			return ModelState{}, err
		}
		return ModelState{Kind: runtime.ModelDFA, DFA: model.Start}, nil
	case runtime.ModelNFA:
		model, err := s.nfaByRef(ref)
		if err != nil {
			return ModelState{}, err
		}
		size := int(model.Start.Len)
		return ModelState{
			Kind:       runtime.ModelNFA,
			NFA:        make([]uint64, size),
			nfaScratch: make([]uint64, size),
		}, nil
	case runtime.ModelAll:
		model, err := s.allByRef(ref)
		if err != nil {
			return ModelState{}, err
		}
		size := (len(model.Members) + 63) / 64
		return ModelState{
			Kind: runtime.ModelAll,
			All:  make([]uint64, size),
		}, nil
	default:
		return ModelState{}, fmt.Errorf("unknown model kind %d", ref.Kind)
	}
}

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
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
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
			return newValidationError(xsderrors.ErrContentModelInvalid, "content model not accepted")
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
			return newValidationError(xsderrors.ErrContentModelInvalid, "content model not accepted")
		}
		accept, ok := bitsetSlice(model.Bitsets, model.Accept)
		if !ok {
			return fmt.Errorf("nfa accept bitset out of range")
		}
		if !bitsetIntersects(state.NFA, accept) {
			return newValidationError(xsderrors.ErrContentModelInvalid, "content model not accepted")
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
				return newValidationError(xsderrors.ErrRequiredElementMissing, "required element missing from all group")
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown model kind %d", ref.Kind)
	}
}

func (s *Session) allMemberAllowsSubst(member runtime.AllMember, elem runtime.ElemID) bool {
	if s == nil || s.rt == nil || member.SubstLen == 0 {
		return false
	}
	start := int(member.SubstOff)
	end := start + int(member.SubstLen)
	if start < 0 || end < 0 || end > len(s.rt.Models.AllSubst) {
		return false
	}
	return slices.Contains(s.rt.Models.AllSubst[start:end], elem)
}

func (s *Session) dfaByRef(ref runtime.ModelRef) (*runtime.DFAModel, error) {
	if ref.ID == 0 || int(ref.ID) >= len(s.rt.Models.DFA) {
		return nil, fmt.Errorf("dfa model %d out of range", ref.ID)
	}
	return &s.rt.Models.DFA[ref.ID], nil
}

func (s *Session) nfaByRef(ref runtime.ModelRef) (*runtime.NFAModel, error) {
	if ref.ID == 0 || int(ref.ID) >= len(s.rt.Models.NFA) {
		return nil, fmt.Errorf("nfa model %d out of range", ref.ID)
	}
	return &s.rt.Models.NFA[ref.ID], nil
}

func (s *Session) allByRef(ref runtime.ModelRef) (*runtime.AllModel, error) {
	if ref.ID == 0 || int(ref.ID) >= len(s.rt.Models.All) {
		return nil, fmt.Errorf("all model %d out of range", ref.ID)
	}
	return &s.rt.Models.All[ref.ID], nil
}
