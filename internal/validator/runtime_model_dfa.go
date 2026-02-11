package validator

import (
	"fmt"
	"sort"

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
		if err := acc.add(StartMatch{Kind: MatchWildcard, Wildcard: edge.Rule}, func() error {
			return fmt.Errorf("ambiguous wildcard match")
		}); err != nil {
			return StartMatch{}, err
		}
		next = edge.Next
	}
	matched, err := acc.result()
	if err != nil {
		return StartMatch{}, err
	}
	state.DFA = next
	return matched, nil
}

func sliceDFATransitions(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFATransition, error) {
	off := rec.TransOff
	end := off + rec.TransLen
	if int(off) > len(model.Transitions) || int(end) > len(model.Transitions) {
		return nil, fmt.Errorf("dfa transitions out of range")
	}
	return model.Transitions[off:end], nil
}

func sliceDFAWildcards(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFAWildcardEdge, error) {
	off := rec.WildOff
	end := off + rec.WildLen
	if int(off) > len(model.Wildcards) || int(end) > len(model.Wildcards) {
		return nil, fmt.Errorf("dfa wildcard edges out of range")
	}
	return model.Wildcards[off:end], nil
}
