package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) stepAll(model *runtime.AllModel, state *ModelState, sym runtime.SymbolID) (StartMatch, error) {
	if sym == 0 {
		return StartMatch{}, newValidationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"unknown element name",
			"",
			s.expectedFromAllRemaining(model, state.All, false),
		)
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
	if !acc.found {
		return StartMatch{}, newValidationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			s.actualElementName(sym, 0),
			s.expectedFromAllRemaining(model, state.All, false),
		)
	}
	match := acc.match
	if allHas(state.All, matchIdx) {
		return StartMatch{}, newValidationError(xsderrors.ErrContentModelInvalid, "duplicate element in all group")
	}
	allSet(state.All, matchIdx)
	state.AllCount++
	return match, nil
}
