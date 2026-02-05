package validator

import (
	"fmt"
	"math/bits"
	"slices"
	"sort"

	xsdErrors "github.com/jacoelho/xsd/errors"
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
		return StartMatch{}, newValidationError(xsdErrors.ErrUnexpectedElement, "no content model match")
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
			return newValidationError(xsdErrors.ErrContentModelInvalid, "content model not accepted")
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
			return newValidationError(xsdErrors.ErrContentModelInvalid, "content model not accepted")
		}
		accept, ok := bitsetSlice(model.Bitsets, model.Accept)
		if !ok {
			return fmt.Errorf("nfa accept bitset out of range")
		}
		if !bitsetIntersects(state.NFA, accept) {
			return newValidationError(xsdErrors.ErrContentModelInvalid, "content model not accepted")
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
				return newValidationError(xsdErrors.ErrRequiredElementMissing, "required element missing from all group")
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown model kind %d", ref.Kind)
	}
}

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
	var matched StartMatch
	var next uint32
	found := false
	for _, edge := range wild {
		if s.rt.WildcardAccepts(edge.Rule, nsBytes, nsID) {
			if found {
				return StartMatch{}, fmt.Errorf("ambiguous wildcard match")
			}
			found = true
			next = edge.Next
			matched = StartMatch{Kind: MatchWildcard, Wildcard: edge.Rule}
		}
	}
	if !found {
		return StartMatch{}, newValidationError(xsdErrors.ErrUnexpectedElement, "no content model match")
	}
	state.DFA = next
	return matched, nil
}

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
		return StartMatch{}, newValidationError(xsdErrors.ErrUnexpectedElement, "no content model match")
	}

	bitsetZero(state.NFA)
	matchCount := 0
	var match StartMatch
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
			matchCount++
			if matchCount > 1 {
				matchErr = newValidationError(xsdErrors.ErrContentModelInvalid, "ambiguous content model match")
				return
			}
			match = StartMatch{Kind: MatchElem, Elem: m.Elem}
			setBit(state.NFA, pos)
		case runtime.PosWildcard:
			if !s.rt.WildcardAccepts(m.Rule, nsBytes, nsID) {
				return
			}
			matchCount++
			if matchCount > 1 {
				matchErr = newValidationError(xsdErrors.ErrContentModelInvalid, "ambiguous content model match")
				return
			}
			match = StartMatch{Kind: MatchWildcard, Wildcard: m.Rule}
			setBit(state.NFA, pos)
		default:
			matchErr = fmt.Errorf("unknown matcher kind %d", m.Kind)
			return
		}
	})
	if matchErr != nil {
		return StartMatch{}, matchErr
	}
	if matchCount == 0 {
		return StartMatch{}, newValidationError(xsdErrors.ErrUnexpectedElement, "no content model match")
	}
	return match, nil
}

func (s *Session) stepAll(model *runtime.AllModel, state *ModelState, sym runtime.SymbolID) (StartMatch, error) {
	if sym == 0 {
		return StartMatch{}, newValidationError(xsdErrors.ErrUnexpectedElement, "unknown element name")
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
				return StartMatch{}, newValidationError(xsdErrors.ErrContentModelInvalid, "ambiguous content model match")
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
			return StartMatch{}, newValidationError(xsdErrors.ErrContentModelInvalid, "ambiguous content model match")
		}
		matchIdx = i
		matchElem = actual
	}
	if matchIdx == -1 {
		return StartMatch{}, newValidationError(xsdErrors.ErrUnexpectedElement, "no content model match")
	}
	if allHas(state.All, matchIdx) {
		return StartMatch{}, newValidationError(xsdErrors.ErrContentModelInvalid, "duplicate element in all group")
	}
	allSet(state.All, matchIdx)
	state.AllCount++
	return StartMatch{Kind: MatchElem, Elem: matchElem}, nil
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

func bitsetSlice(blob runtime.BitsetBlob, ref runtime.BitsetRef) ([]uint64, bool) {
	if ref.Len == 0 {
		return nil, true
	}
	off := int(ref.Off)
	end := off + int(ref.Len)
	if off < 0 || end < 0 || end > len(blob.Words) {
		return nil, false
	}
	return blob.Words[off:end], true
}

func bitsetZero(words []uint64) {
	for i := range words {
		words[i] = 0
	}
}

func bitsetOr(dst, src []uint64) {
	for i := range dst {
		if i < len(src) {
			dst[i] |= src[i]
		}
	}
}

func bitsetEmpty(words []uint64) bool {
	for _, w := range words {
		if w != 0 {
			return false
		}
	}
	return true
}

func bitsetIntersects(a, b []uint64) bool {
	limit := min(len(b), len(a))
	for i := range limit {
		if a[i]&b[i] != 0 {
			return true
		}
	}
	return false
}

func forEachBit(words []uint64, limit int, fn func(int)) {
	for wi, w := range words {
		for w != 0 {
			bit := bits.TrailingZeros64(w)
			pos := wi*64 + bit
			if pos >= limit {
				return
			}
			fn(pos)
			w &^= 1 << bit
		}
	}
}

func setBit(words []uint64, pos int) {
	if pos < 0 {
		return
	}
	word := pos / 64
	bit := uint(pos % 64)
	if word >= len(words) {
		return
	}
	words[word] |= 1 << bit
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
