package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) stepAll(model *runtime.AllModel, state *ModelState, sym runtime.SymbolID) (StartMatch, error) {
	if sym == 0 {
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "unknown element name")
	}
	var acc modelMatchAccumulator
	matchIdx := -1
	for i, member := range model.Members {
		elem, ok := s.element(member.Elem)
		if !ok {
			continue
		}
		if !member.AllowsSubst {
			if elem.Name != sym {
				continue
			}
			if err := acc.add(StartMatch{Kind: MatchElem, Elem: member.Elem}, ambiguousContentModelMatchError); err != nil {
				return StartMatch{}, err
			}
			matchIdx = i
			continue
		}
		actual, ok := s.globalElementBySymbol(sym)
		if !ok {
			continue
		}
		if !s.allMemberAllowsSubst(member, actual) {
			continue
		}
		if err := acc.add(StartMatch{Kind: MatchElem, Elem: actual}, ambiguousContentModelMatchError); err != nil {
			return StartMatch{}, err
		}
		matchIdx = i
	}
	match, err := acc.result()
	if err != nil {
		return StartMatch{}, err
	}
	if allHas(state.All, matchIdx) {
		return StartMatch{}, newValidationError(xsderrors.ErrContentModelInvalid, "duplicate element in all group")
	}
	allSet(state.All, matchIdx)
	state.AllCount++
	return match, nil
}

func allHas(words []uint64, idx int) bool {
	if idx < 0 {
		return false
	}
	word := idx / 64
	bit := uint(idx % 64)
	if word >= len(words) {
		return false
	}
	return words[word]&(1<<bit) != 0
}

func allSet(words []uint64, idx int) {
	if idx < 0 {
		return
	}
	word := idx / 64
	bit := uint(idx % 64)
	if word >= len(words) {
		return
	}
	words[word] |= 1 << bit
}
