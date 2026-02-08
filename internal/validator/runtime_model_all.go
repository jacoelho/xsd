package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) stepAll(model *runtime.AllModel, state *ModelState, sym runtime.SymbolID) (StartMatch, error) {
	if sym == 0 {
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "unknown element name")
	}
	matchIdx := -1
	matchElem := runtime.ElemID(0)
	for i, member := range model.Members {
		elem, ok := s.element(member.Elem)
		if !ok {
			continue
		}
		if !member.AllowsSubst {
			if elem.Name != sym {
				continue
			}
			if matchIdx != -1 {
				return StartMatch{}, newValidationError(xsderrors.ErrContentModelInvalid, "ambiguous content model match")
			}
			matchIdx = i
			matchElem = member.Elem
			continue
		}
		actual, ok := s.globalElementBySymbol(sym)
		if !ok {
			continue
		}
		if !s.allMemberAllowsSubst(member, actual) {
			continue
		}
		if matchIdx != -1 {
			return StartMatch{}, newValidationError(xsderrors.ErrContentModelInvalid, "ambiguous content model match")
		}
		matchIdx = i
		matchElem = actual
	}
	if matchIdx == -1 {
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
	}
	if allHas(state.All, matchIdx) {
		return StartMatch{}, newValidationError(xsderrors.ErrContentModelInvalid, "duplicate element in all group")
	}
	allSet(state.All, matchIdx)
	state.AllCount++
	return StartMatch{Kind: MatchElem, Elem: matchElem}, nil
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
