package validator

import (
	"fmt"
	"sort"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

// InitStartModelState allocates and initializes model state for one compiled model reference.
func InitStartModelState(rt *runtime.Schema, ref runtime.ModelRef) (StartModelState, error) {
	if rt == nil {
		return StartModelState{}, fmt.Errorf("session missing runtime schema")
	}
	switch ref.Kind {
	case runtime.ModelNone:
		return StartModelState{Kind: runtime.ModelNone}, nil
	case runtime.ModelDFA:
		model, err := dfaByRef(rt, ref)
		if err != nil {
			return StartModelState{}, err
		}
		return StartModelState{Kind: runtime.ModelDFA, DFA: model.Start}, nil
	case runtime.ModelNFA:
		model, err := nfaByRef(rt, ref)
		if err != nil {
			return StartModelState{}, err
		}
		size := int(model.Start.Len)
		return StartModelState{
			Kind:       runtime.ModelNFA,
			NFA:        make([]uint64, size),
			nfaScratch: make([]uint64, size),
		}, nil
	case runtime.ModelAll:
		model, err := allByRef(rt, ref)
		if err != nil {
			return StartModelState{}, err
		}
		size := (len(model.Members) + 63) / 64
		return StartModelState{
			Kind: runtime.ModelAll,
			All:  make([]uint64, size),
		}, nil
	default:
		return StartModelState{}, unknownModelKindError(ref.Kind)
	}
}

func unknownModelKindError(kind runtime.ModelKind) error {
	return fmt.Errorf("unknown model kind %d", kind)
}

// StepStartModel advances one content-model state with one element symbol.
func StepStartModel(rt *runtime.Schema, ref runtime.ModelRef, state *StartModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
	if rt == nil {
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
		model, err := dfaByRef(rt, ref)
		if err != nil {
			return StartMatch{}, err
		}
		return stepDFA(rt, model, state, sym, nsID, nsBytes)
	case runtime.ModelNFA:
		model, err := nfaByRef(rt, ref)
		if err != nil {
			return StartMatch{}, err
		}
		return stepNFA(rt, model, state, sym, nsID, nsBytes)
	case runtime.ModelAll:
		model, err := allByRef(rt, ref)
		if err != nil {
			return StartMatch{}, err
		}
		return stepAll(rt, model, state, sym)
	default:
		return StartMatch{}, fmt.Errorf("unknown model kind %d", ref.Kind)
	}
}

// AcceptStartModel verifies that one content-model state is accepting.
func AcceptStartModel(rt *runtime.Schema, ref runtime.ModelRef, state *StartModelState) error {
	if rt == nil {
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
		model, err := dfaByRef(rt, ref)
		if err != nil {
			return err
		}
		if int(state.DFA) >= len(model.States) {
			return fmt.Errorf("dfa state %d out of range", state.DFA)
		}
		if !model.States[state.DFA].Accept {
			return validationErrorWithDetails(
				xsderrors.ErrContentModelInvalid,
				"content model not accepted",
				"",
				expectedFromDFAState(rt, model, state.DFA),
			)
		}
		return nil
	case runtime.ModelNFA:
		model, err := nfaByRef(rt, ref)
		if err != nil {
			return err
		}
		if bitsetEmpty(state.NFA) {
			if model.Nullable {
				return nil
			}
			return validationErrorWithDetails(
				xsderrors.ErrContentModelInvalid,
				"content model not accepted",
				"",
				expectedFromNFAStart(rt, model),
			)
		}
		accept, ok := bitsetSlice(model.Bitsets, model.Accept)
		if !ok {
			return fmt.Errorf("nfa accept bitset out of range")
		}
		if !bitsetIntersects(state.NFA, accept) {
			return validationErrorWithDetails(
				xsderrors.ErrContentModelInvalid,
				"content model not accepted",
				"",
				expectedFromNFAFollow(rt, model, state.NFA),
			)
		}
		return nil
	case runtime.ModelAll:
		model, err := allByRef(rt, ref)
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
				return validationErrorWithDetails(
					xsderrors.ErrRequiredElementMissing,
					"required element missing from all group",
					"",
					expectedFromAllRemaining(rt, model, state.All, true),
				)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown model kind %d", ref.Kind)
	}
}

func stepDFA(rt *runtime.Schema, model *runtime.DFAModel, state *StartModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
	if int(state.DFA) >= len(model.States) {
		return StartMatch{}, fmt.Errorf("dfa state %d out of range", state.DFA)
	}
	rec := model.States[state.DFA]
	trans, err := sliceDFATransitions(model, rec)
	if err != nil {
		return StartMatch{}, err
	}
	if sym != 0 && len(trans) > 0 {
		idx := sort.Search(len(trans), func(i int) bool { return trans[i].Sym >= sym })
		if idx < len(trans) && trans[idx].Sym == sym {
			state.DFA = trans[idx].Next
			return StartMatch{Kind: StartMatchElem, Elem: trans[idx].Elem}, nil
		}
	}
	wild, err := sliceDFAWildcards(model, rec)
	if err != nil {
		return StartMatch{}, err
	}
	var acc matchAccumulator
	var next uint32
	for _, edge := range wild {
		if !rt.WildcardAccepts(edge.Rule, nsBytes, nsID) {
			continue
		}
		if addErr := acc.add(StartMatch{Kind: StartMatchWildcard, Wildcard: edge.Rule}, func() error {
			return fmt.Errorf("ambiguous wildcard match")
		}); addErr != nil {
			return StartMatch{}, addErr
		}
		next = edge.Next
	}
	if !acc.found {
		return StartMatch{}, validationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			StartActualElementName(rt, sym, nsID),
			expectedFromDFAState(rt, model, state.DFA),
		)
	}
	state.DFA = next
	return acc.match, nil
}

func stepNFA(rt *runtime.Schema, model *runtime.NFAModel, state *StartModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
	if len(state.NFA) != int(model.Start.Len) {
		return StartMatch{}, fmt.Errorf("nfa state size mismatch")
	}
	if len(state.nfaScratch) != len(state.NFA) {
		return StartMatch{}, fmt.Errorf("nfa scratch size mismatch")
	}

	reachable := state.nfaScratch
	bitsetZero(reachable)
	if bitsetEmpty(state.NFA) {
		start, ok := bitsetSlice(model.Bitsets, model.Start)
		if !ok {
			return StartMatch{}, fmt.Errorf("nfa start bitset out of range")
		}
		copy(reachable, start)
	} else {
		if int(model.FollowLen) > len(model.Follow) {
			return StartMatch{}, fmt.Errorf("nfa follow table out of range")
		}
		var followErr error
		forEachBit(state.NFA, len(model.Follow), func(pos int) {
			if followErr != nil {
				return
			}
			ref := model.Follow[pos]
			set, ok := bitsetSlice(model.Bitsets, ref)
			if !ok {
				followErr = fmt.Errorf("nfa follow bitset out of range")
				return
			}
			bitsetOr(reachable, set)
		})
		if followErr != nil {
			return StartMatch{}, followErr
		}
	}

	if bitsetEmpty(reachable) {
		return StartMatch{}, validationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			StartActualElementName(rt, sym, nsID),
			expectedFromNFAStart(rt, model),
		)
	}

	var acc matchAccumulator
	matchPos := -1
	var matchErr error
	forEachBit(reachable, len(model.Matchers), func(pos int) {
		if matchErr != nil {
			return
		}
		m := model.Matchers[pos]
		switch m.Kind {
		case runtime.PosExact:
			if sym == 0 || m.Sym != sym {
				return
			}
			if err := acc.add(StartMatch{Kind: StartMatchElem, Elem: m.Elem}, ambiguousContentModelMatchError); err != nil {
				matchErr = err
				return
			}
			matchPos = pos
		case runtime.PosWildcard:
			if !rt.WildcardAccepts(m.Rule, nsBytes, nsID) {
				return
			}
			if err := acc.add(StartMatch{Kind: StartMatchWildcard, Wildcard: m.Rule}, ambiguousContentModelMatchError); err != nil {
				matchErr = err
				return
			}
			matchPos = pos
		default:
			matchErr = fmt.Errorf("unknown matcher kind %d", m.Kind)
		}
	})
	if matchErr != nil {
		return StartMatch{}, matchErr
	}
	if !acc.found {
		return StartMatch{}, validationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			StartActualElementName(rt, sym, nsID),
			expectedFromNFAMatchers(rt, model, reachable),
		)
	}
	bitsetZero(state.NFA)
	setBit(state.NFA, matchPos)
	return acc.match, nil
}

func stepAll(rt *runtime.Schema, model *runtime.AllModel, state *StartModelState, sym runtime.SymbolID) (StartMatch, error) {
	if sym == 0 {
		return StartMatch{}, validationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"unknown element name",
			"",
			expectedFromAllRemaining(rt, model, state.All, false),
		)
	}
	var acc matchAccumulator
	matchIdx := -1
	for i, member := range model.Members {
		elem, ok := element(rt, member.Elem)
		if !ok {
			continue
		}
		if !member.AllowsSubst {
			if elem.Name != sym {
				continue
			}
			if err := acc.add(StartMatch{Kind: StartMatchElem, Elem: member.Elem}, ambiguousContentModelMatchError); err != nil {
				return StartMatch{}, err
			}
			matchIdx = i
			continue
		}
		actual, ok := LookupStartGlobalElement(rt, sym)
		if !ok {
			continue
		}
		if !allMemberAllowsSubst(rt, member, actual) {
			continue
		}
		if err := acc.add(StartMatch{Kind: StartMatchElem, Elem: actual}, ambiguousContentModelMatchError); err != nil {
			return StartMatch{}, err
		}
		matchIdx = i
	}
	if !acc.found {
		return StartMatch{}, validationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			StartActualElementName(rt, sym, 0),
			expectedFromAllRemaining(rt, model, state.All, false),
		)
	}
	if allHas(state.All, matchIdx) {
		return StartMatch{}, validationError(xsderrors.ErrContentModelInvalid, "duplicate element in all group")
	}
	allSet(state.All, matchIdx)
	state.AllCount++
	return acc.match, nil
}

type matchAccumulator struct {
	match StartMatch
	found bool
}

func (a *matchAccumulator) add(match StartMatch, onAmbiguous func() error) error {
	if !a.found {
		a.match = match
		a.found = true
		return nil
	}
	if onAmbiguous == nil {
		return fmt.Errorf("ambiguous content model match")
	}
	return onAmbiguous()
}

func noContentModelMatchError() error {
	return validationError(xsderrors.ErrUnexpectedElement, "no content model match")
}

func ambiguousContentModelMatchError() error {
	return validationError(xsderrors.ErrContentModelInvalid, "ambiguous content model match")
}

func validationError(code xsderrors.ErrorCode, msg string) error {
	return xsderrors.New(code, msg)
}

func validationErrorWithDetails(code xsderrors.ErrorCode, msg, actual string, expected []string) error {
	return xsderrors.NewWithDetails(code, msg, actual, expected)
}
