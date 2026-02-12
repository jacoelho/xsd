package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) stepNFA(model *runtime.NFAModel, state *ModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
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
		return StartMatch{}, newValidationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			s.actualElementName(sym, nsID),
			s.expectedFromNFAStart(model),
		)
	}

	var acc modelMatchAccumulator
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
			if err := acc.add(StartMatch{Kind: MatchElem, Elem: m.Elem}, ambiguousContentModelMatchError); err != nil {
				matchErr = err
				return
			}
			matchPos = pos
		case runtime.PosWildcard:
			if !s.rt.WildcardAccepts(m.Rule, nsBytes, nsID) {
				return
			}
			if err := acc.add(StartMatch{Kind: MatchWildcard, Wildcard: m.Rule}, ambiguousContentModelMatchError); err != nil {
				matchErr = err
				return
			}
			matchPos = pos
		default:
			matchErr = fmt.Errorf("unknown matcher kind %d", m.Kind)
			return
		}
	})
	if matchErr != nil {
		return StartMatch{}, matchErr
	}
	if !acc.found {
		return StartMatch{}, newValidationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			s.actualElementName(sym, nsID),
			s.expectedFromNFAMatchers(model, reachable),
		)
	}
	match := acc.match
	bitsetZero(state.NFA)
	setBit(state.NFA, matchPos)
	return match, nil
}
