package contentmodel

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// Sub-code suffixes for content model violations.
const (
	// ErrorCodeMissing - required element missing (2.4.b)
	ErrorCodeMissing = "b"
	// ErrorCodeNotExpectedHere - element not expected at this position (2.4.d)
	ErrorCodeNotExpectedHere = "d"
)

// ValidationError describes a content model violation.
type ValidationError struct {
	Index      int // child index where error occurred
	Message    string
	SubCode    string // Sub-code suffix like "b" or "d" for content model violations.
	IsAllGroup bool   // Set to true if this is from an all group
}

// Error returns the formatted validation error.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("child %d: %s", e.Index, e.Message)
}

// FullCode returns the full error code including sub-code.
func (e *ValidationError) FullCode() string {
	base := string(errors.ErrContentModelInvalid)
	if e.SubCode == "" {
		return base
	}
	switch e.SubCode {
	case ErrorCodeMissing:
		return string(errors.ErrRequiredElementMissing)
	case ErrorCodeNotExpectedHere:
		return string(errors.ErrUnexpectedElement)
	default:
		return base + "." + e.SubCode
	}
}

// SymbolMatcher is used by the automaton to check substitution groups.
type SymbolMatcher interface {
	IsSubstitutable(actual, declared types.QName) bool
}

// MatchResult describes what a child element matched in the content model.
type MatchResult struct {
	IsWildcard      bool                  // true if matched a wildcard (xs:any)
	ProcessContents types.ProcessContents // processContents from the wildcard (only valid if IsWildcard)
	MatchedQName    types.QName           // matched declaration name (for non-wildcard matches)
}

// symbolCandidate represents a potential symbol match during content model validation.
type symbolCandidate struct {
	idx        int
	isWildcard bool
}

// Validate checks that children satisfy the content model.
// Returns nil if valid, or a ValidationError describing the first violation.
// Runs in O(n) time with no backtracking.
func (a *Automaton) Validate(children []xml.Element, matcher SymbolMatcher) error {
	_, err := a.ValidateWithMatches(children, matcher, nil)
	return err
}

// handleGroupCounters processes group iteration counting for the current transition.
// Returns an error if maxOccurs is exceeded.
func (a *Automaton) handleGroupCounters(state, next, symIdx, childIdx int, groupCounts, groupRemainders map[int]int) error {
	lastProcessedGroupID := -1
	for _, checkState := range []int{state, next} {
		if c := a.counting[checkState]; c != nil && c.IsGroupCounter {
			if c.GroupID == lastProcessedGroupID {
				continue
			}
			isStartSymbol := slices.Contains(c.GroupStartSymbols, symIdx)
			if isStartSymbol {
				lastProcessedGroupID = c.GroupID
				if c.UnitSize > 0 {
					if c.Max >= 0 && groupCounts[c.GroupID] >= c.Max {
						return &ValidationError{
							Index:   childIdx,
							Message: fmt.Sprintf("group exceeds maxOccurs=%d", c.Max),
							SubCode: ErrorCodeNotExpectedHere,
						}
					}
					groupRemainders[c.GroupID]++
					if groupRemainders[c.GroupID] == c.UnitSize {
						groupCounts[c.GroupID]++
						groupRemainders[c.GroupID] = 0
						if c.Max >= 0 && groupCounts[c.GroupID] > c.Max {
							return &ValidationError{
								Index:   childIdx,
								Message: fmt.Sprintf("group exceeds maxOccurs=%d", c.Max),
								SubCode: ErrorCodeNotExpectedHere,
							}
						}
					}
					continue
				}

				groupCounts[c.GroupID]++
				startCount := groupCounts[c.GroupID]
				minIterations := startCount
				if startCount > 0 && c.FirstPosMaxOccurs == types.UnboundedOccurs {
					minIterations = 1
				} else if c.FirstPosMaxOccurs > 1 {
					minIterations = (startCount + c.FirstPosMaxOccurs - 1) / c.FirstPosMaxOccurs
				}
				if c.Max >= 0 && minIterations > c.Max {
					return &ValidationError{
						Index:   childIdx,
						Message: fmt.Sprintf("group exceeds maxOccurs=%d", c.Max),
						SubCode: ErrorCodeNotExpectedHere,
					}
				}
			}
		}
	}
	return nil
}

// handleElementCounter processes element occurrence counting for the current match.
// Returns an error if maxOccurs is exceeded.
func (a *Automaton) handleElementCounter(state, next, symIdx, childIdx int, symbolCounts map[int]int, childName string) error {
	symbolCounts[symIdx]++
	max := types.UnboundedOccurs
	if symIdx >= 0 && symIdx < len(a.symbolMax) {
		max = a.symbolMax[symIdx]
	}
	if max >= 0 && symbolCounts[symIdx] > max {
		return &ValidationError{
			Index:   childIdx,
			Message: fmt.Sprintf("element %q exceeds maxOccurs=%d", childName, max),
			SubCode: ErrorCodeNotExpectedHere,
		}
	}
	return nil
}

// validateFinalCounts checks all counters satisfy minOccurs at end of validation.
func (a *Automaton) validateFinalCounts(symbolCounts, groupCounts, groupRemainders map[int]int, childCount int) error {
	checkedGroupIDs := make(map[int]bool)
	for stateID, c := range a.counting {
		if c == nil {
			continue
		}
		if c.IsGroupCounter {
			if checkedGroupIDs[c.GroupID] {
				continue
			}
			checkedGroupIDs[c.GroupID] = true
			if c.UnitSize > 0 && groupRemainders[c.GroupID] != 0 {
				return &ValidationError{
					Index:   childCount,
					Message: fmt.Sprintf("group incomplete: expected %d occurrences per iteration", c.UnitSize),
					SubCode: ErrorCodeMissing,
				}
			}
			if count, hasCount := groupCounts[c.GroupID]; hasCount && count < c.Min {
				return &ValidationError{
					Index:   childCount,
					Message: fmt.Sprintf("minOccurs=%d not satisfied (state=%d, group=%d, count=%d)", c.Min, stateID, c.GroupID, count),
					SubCode: ErrorCodeMissing,
				}
			}
		}
	}
	for symIdx, min := range a.symbolMin {
		if min <= 0 {
			continue
		}
		count := symbolCounts[symIdx]
		if count < min {
			return &ValidationError{
				Index:   childCount,
				Message: fmt.Sprintf("minOccurs=%d not satisfied (symbol=%d, count=%d)", min, symIdx, count),
				SubCode: ErrorCodeMissing,
			}
		}
	}
	return nil
}

// ValidateWithMatches validates children and returns match results for each child.
// This allows the caller to determine how each child was matched (element vs wildcard).
func (a *Automaton) ValidateWithMatches(children []xml.Element, matcher SymbolMatcher, wildcards []*types.AnyElement) ([]MatchResult, error) {
	matches := make([]MatchResult, len(children))

	if len(children) == 0 {
		if a.emptyOK {
			return matches, nil
		}
		return nil, &ValidationError{Index: 0, Message: "content required but none found", SubCode: ErrorCodeMissing}
	}

	state := 0
	symbolCounts := make(map[int]int)
	groupCounts := make(map[int]int)
	groupRemainders := make(map[int]int)

	for i, child := range children {
		// find the best matching symbol - one that has a valid transition
		symIdx, isWildcard, next := a.findBestMatch(child, state, matcher)

		if symIdx < 0 {
			// element is not part of the content model at all - not allowed (.d)
			return nil, &ValidationError{
				Index:   i,
				Message: fmt.Sprintf("element %q not allowed", child.LocalName()),
				SubCode: ErrorCodeNotExpectedHere,
			}
		}

		matches[i].IsWildcard = isWildcard
		if isWildcard && len(wildcards) > 0 {
			matches[i].ProcessContents = a.findWildcardProcessContents(child, wildcards)
		} else if !isWildcard && symIdx >= 0 && symIdx < len(a.symbols) {
			matches[i].MatchedQName = a.symbols[symIdx].QName
		}

		if next < 0 {
			// element IS in the model but no valid transition exists
			// if we're in an accepting state, this is extra content (.d)
			// otherwise, a required element is missing before this one (.b)
			isAccepting := (state < len(a.accepting) && a.accepting[state]) ||
				(state == 0 && a.emptyOK)
			if isAccepting {
				return nil, &ValidationError{
					Index:   i,
					Message: fmt.Sprintf("element %q not expected here", child.LocalName()),
					SubCode: ErrorCodeNotExpectedHere,
				}
			}
			return nil, &ValidationError{
				Index:   i,
				Message: fmt.Sprintf("element %q not expected here", child.LocalName()),
				SubCode: ErrorCodeMissing,
			}
		}

		if err := a.handleGroupCounters(state, next, symIdx, i, groupCounts, groupRemainders); err != nil {
			return nil, err
		}

		if err := a.handleElementCounter(state, next, symIdx, i, symbolCounts, child.LocalName()); err != nil {
			return nil, err
		}

		state = next
	}

	if !a.accepting[state] {
		return nil, &ValidationError{
			Index:   len(children),
			Message: "content incomplete: required elements missing",
			SubCode: ErrorCodeMissing,
		}
	}

	if err := a.validateFinalCounts(symbolCounts, groupCounts, groupRemainders, len(children)); err != nil {
		return nil, err
	}

	return matches, nil
}

// findBestMatch finds the best matching symbol for an element at the given state.
// It returns the symbol index, whether it's a wildcard match, and the next state.
// If an element can match multiple symbols, it prefers the one with a valid transition.
func (a *Automaton) findBestMatch(elem xml.Element, state int, matcher SymbolMatcher) (symIdx int, isWildcard bool, next int) {
	qname := types.QName{
		Namespace: types.NamespaceURI(elem.NamespaceURI()),
		Local:     elem.LocalName(),
	}

	var candidates []symbolCandidate

	// exact element match
	for i, sym := range a.symbols {
		if sym.Kind == KindElement && sym.QName.Equal(qname) {
			candidates = append(candidates, symbolCandidate{i, false})
		}
	}

	// substitution group match
	if matcher != nil {
		for i, sym := range a.symbols {
			if sym.Kind == KindElement && sym.AllowSubstitution && matcher.IsSubstitutable(qname, sym.QName) {
				candidates = append(candidates, symbolCandidate{i, false})
			}
		}
	}

	// wildcard matches
	for i, sym := range a.symbols {
		switch sym.Kind {
		case KindAny:
			candidates = append(candidates, symbolCandidate{i, true})
		case KindAnyNS:
			if string(qname.Namespace) == sym.NS {
				candidates = append(candidates, symbolCandidate{i, true})
			}
		case KindAnyOther:
			// ##other matches any namespace other than target namespace AND not empty
			elemNS := string(qname.Namespace)
			if elemNS != sym.NS && elemNS != "" {
				candidates = append(candidates, symbolCandidate{i, true})
			}
		case KindAnyNSList:
			elemNS := string(qname.Namespace)
			if slices.Contains(sym.NSList, elemNS) {
				candidates = append(candidates, symbolCandidate{i, true})
			}
		}
	}

	if len(candidates) == 0 {
		return -1, false, -1
	}

	// try to find a candidate with a valid transition
	for _, c := range candidates {
		nextState := a.trans[state][c.idx]
		if nextState >= 0 {
			return c.idx, c.isWildcard, nextState
		}
	}

	// no valid transition, return the first candidate (for error reporting)
	return candidates[0].idx, candidates[0].isWildcard, a.trans[state][candidates[0].idx]
}

// findWildcardProcessContents finds the processContents for a wildcard match.
func (a *Automaton) findWildcardProcessContents(elem xml.Element, wildcards []*types.AnyElement) types.ProcessContents {
	if len(wildcards) == 0 {
		return types.Strict // default to strict if no wildcards available
	}

	qname := types.QName{
		Namespace: types.NamespaceURI(elem.NamespaceURI()),
		Local:     elem.LocalName(),
	}

	for _, w := range wildcards {
		if a.matchesWildcard(qname, w) {
			return w.ProcessContents
		}
	}

	return types.Strict // default
}

// matchesWildcard checks if a QName matches a wildcard.
func (a *Automaton) matchesWildcard(qname types.QName, w *types.AnyElement) bool {
	switch w.Namespace {
	case types.NSCAny:
		return true
	case types.NSCOther:
		return !qname.Namespace.IsEmpty() && qname.Namespace != w.TargetNamespace
	case types.NSCTargetNamespace:
		return qname.Namespace == w.TargetNamespace
	case types.NSCLocal:
		return qname.Namespace.IsEmpty()
	case types.NSCList:
		return slices.Contains(w.NamespaceList, qname.Namespace)
	}
	return false
}