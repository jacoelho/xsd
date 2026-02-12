package validator

import (
	"fmt"
	"sort"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) stepDFA(model *runtime.DFAModel, state *ModelState, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (StartMatch, error) {
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
			return StartMatch{Kind: MatchElem, Elem: trans[idx].Elem}, nil
		}
	}
	wild, err := sliceDFAWildcards(model, rec)
	if err != nil {
		return StartMatch{}, err
	}
	var acc modelMatchAccumulator
	var next uint32
	for _, edge := range wild {
		if !s.rt.WildcardAccepts(edge.Rule, nsBytes, nsID) {
			continue
		}
		if addErr := acc.add(StartMatch{Kind: MatchWildcard, Wildcard: edge.Rule}, func() error {
			return fmt.Errorf("ambiguous wildcard match")
		}); addErr != nil {
			return StartMatch{}, addErr
		}
		next = edge.Next
	}
	if !acc.found {
		return StartMatch{}, newValidationErrorWithDetails(
			xsderrors.ErrUnexpectedElement,
			"no content model match",
			s.actualElementName(sym, nsID),
			s.expectedFromDFAState(model, state.DFA),
		)
	}
	matched := acc.match
	state.DFA = next
	return matched, nil
}
