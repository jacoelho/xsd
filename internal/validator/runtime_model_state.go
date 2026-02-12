package validator

import (
	"fmt"

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

// InitModelState is an exported function.
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
