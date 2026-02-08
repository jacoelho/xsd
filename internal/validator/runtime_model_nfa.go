package validator

import (
	"fmt"
	"math/bits"

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
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
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
				matchErr = newValidationError(xsderrors.ErrContentModelInvalid, "ambiguous content model match")
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
				matchErr = newValidationError(xsderrors.ErrContentModelInvalid, "ambiguous content model match")
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
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
	}
	return match, nil
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
