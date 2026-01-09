package grammar

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

// AutomatonStreamValidator validates content models incrementally.
type AutomatonStreamValidator struct {
	automaton    *Automaton
	state        int
	childIndex   int
	matcher      SymbolMatcher
	wildcards    []*types.AnyElement
	symbolCounts []int
	groupState   groupCounterState
}

// NewStreamValidator creates a streaming validator for the automaton.
func (a *Automaton) NewStreamValidator(matcher SymbolMatcher, wildcards []*types.AnyElement) *AutomatonStreamValidator {
	validator := &AutomatonStreamValidator{}
	validator.Reset(a, matcher, wildcards)
	return validator
}

// Reset reinitializes the validator for reuse.
func (v *AutomatonStreamValidator) Reset(a *Automaton, matcher SymbolMatcher, wildcards []*types.AnyElement) {
	v.automaton = a
	v.matcher = matcher
	v.wildcards = wildcards
	v.state = 0
	v.childIndex = 0
	if a == nil {
		v.symbolCounts = v.symbolCounts[:0]
		v.groupState.reset(0)
		return
	}
	v.symbolCounts = ensureIntSlice(v.symbolCounts, len(a.symbols))
	clear(v.symbolCounts)
	v.groupState.reset(a.groupCount)
}

// Release clears references before returning the validator to a pool.
func (v *AutomatonStreamValidator) Release() {
	if v == nil {
		return
	}
	v.automaton = nil
	v.matcher = nil
	v.wildcards = nil
	v.state = 0
	v.childIndex = 0
}

// Feed validates a single child element and advances the automaton state.
func (v *AutomatonStreamValidator) Feed(child types.QName) (MatchResult, error) {
	var result MatchResult
	childIdx := v.childIndex
	v.childIndex++

	symIdx, isWildcard, next := v.automaton.findBestMatchQName(child, v.state, v.matcher)
	if symIdx < 0 {
		return result, &ValidationError{
			Index:   childIdx,
			Message: fmt.Sprintf("element %q not allowed", child.Local),
			SubCode: ErrorCodeNotExpectedHere,
		}
	}

	result.IsWildcard = isWildcard
	if isWildcard && len(v.wildcards) > 0 {
		result.ProcessContents = v.automaton.findWildcardProcessContentsQName(child, v.wildcards)
	} else if !isWildcard && symIdx >= 0 && symIdx < len(v.automaton.symbols) {
		result.MatchedQName = v.automaton.symbols[symIdx].QName
		result.MatchedElement = v.automaton.matchedElement(v.state, symIdx)
	}

	if next < 0 {
		isAccepting := (v.state < len(v.automaton.accepting) && v.automaton.accepting[v.state]) ||
			(v.state == 0 && v.automaton.emptyOK)
		if isAccepting {
			return result, &ValidationError{
				Index:   childIdx,
				Message: fmt.Sprintf("element %q not expected here", child.Local),
				SubCode: ErrorCodeNotExpectedHere,
			}
		}
		return result, &ValidationError{
			Index:   childIdx,
			Message: fmt.Sprintf("element %q not expected here", child.Local),
			SubCode: ErrorCodeMissing,
		}
	}

	if err := v.automaton.handleGroupCounters(v.state, next, symIdx, childIdx, &v.groupState); err != nil {
		return result, err
	}
	if err := v.automaton.handleElementCounter(v.state, next, symIdx, childIdx, v.symbolCounts, child.Local); err != nil {
		return result, err
	}

	v.state = next
	return result, nil
}

// Close validates final state and counters after the last child.
func (v *AutomatonStreamValidator) Close() error {
	if v.childIndex == 0 {
		if v.automaton.emptyOK {
			return nil
		}
		return &ValidationError{
			Index:   0,
			Message: "content required but none found",
			SubCode: ErrorCodeMissing,
		}
	}

	if v.state >= len(v.automaton.accepting) || !v.automaton.accepting[v.state] {
		return &ValidationError{
			Index:   v.childIndex,
			Message: "content incomplete: required elements missing",
			SubCode: ErrorCodeMissing,
		}
	}

	if err := v.automaton.validateFinalCounts(v.symbolCounts, &v.groupState, v.childIndex); err != nil {
		return err
	}

	return nil
}

// AllGroupStreamValidator validates all-group content models incrementally.
type AllGroupStreamValidator struct {
	validator       *AllGroupValidator
	matcher         SymbolMatcher
	elementSeen     []bool
	numRequiredSeen int
	childIndex      int
}

// NewStreamValidator creates a streaming validator for an all group.
func (v *AllGroupValidator) NewStreamValidator(matcher SymbolMatcher) *AllGroupStreamValidator {
	return &AllGroupStreamValidator{
		validator:   v,
		matcher:     matcher,
		elementSeen: make([]bool, len(v.elements)),
	}
}

// Feed validates a single child element against the all group.
func (v *AllGroupStreamValidator) Feed(child types.QName) (MatchResult, error) {
	var result MatchResult
	childIdx := v.childIndex
	v.childIndex++

	if len(v.validator.elements) == 0 {
		return result, &ValidationError{
			Index:   childIdx,
			Message: fmt.Sprintf("element %q not allowed", child.Local),
			SubCode: ErrorCodeNotExpectedHere,
		}
	}

	for i, elem := range v.validator.elements {
		elemQName := elem.ElementQName()
		if elemQName.Equal(child) {
			if v.elementSeen[i] {
				return result, &ValidationError{
					Index:   childIdx,
					Message: fmt.Sprintf("element %q appears more than once in all group", child.Local),
					SubCode: ErrorCodeNotExpectedHere,
				}
			}
			v.elementSeen[i] = true
			if !elem.IsOptional() {
				v.numRequiredSeen++
			}
			result.MatchedQName = elemQName
			result.MatchedElement = elem.ElementDecl()
			return result, nil
		}

		if v.matcher != nil && elem.AllowsSubstitution() && v.matcher.IsSubstitutable(child, elemQName) {
			if v.elementSeen[i] {
				return result, &ValidationError{
					Index:   childIdx,
					Message: fmt.Sprintf("element %q (substituting for %q) appears more than once in all group", child.Local, elemQName.Local),
					SubCode: ErrorCodeNotExpectedHere,
				}
			}
			v.elementSeen[i] = true
			if !elem.IsOptional() {
				v.numRequiredSeen++
			}
			result.MatchedQName = elemQName
			result.MatchedElement = elem.ElementDecl()
			return result, nil
		}
	}

	return result, &ValidationError{
		Index:   childIdx,
		Message: fmt.Sprintf("element %q not allowed in all group", child.Local),
		SubCode: ErrorCodeNotExpectedHere,
	}
}

// Close validates required elements after the last child.
func (v *AllGroupStreamValidator) Close() error {
	if v.childIndex == 0 && v.validator.minOccurs == 0 {
		return nil
	}
	if v.numRequiredSeen == v.validator.numRequired {
		return nil
	}
	for i, elem := range v.validator.elements {
		if !elem.IsOptional() && !v.elementSeen[i] {
			return &ValidationError{
				Index:   v.childIndex,
				Message: fmt.Sprintf("required element %q missing from all group", elem.ElementQName().Local),
				SubCode: ErrorCodeMissing,
			}
		}
	}
	return &ValidationError{
		Index:   v.childIndex,
		Message: "required elements missing from all group",
		SubCode: ErrorCodeMissing,
	}
}
