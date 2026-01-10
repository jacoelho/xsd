package grammar

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
	// child index where error occurred
	Index   int
	Message string
	// Sub-code suffix like "b" or "d" for content model violations.
	SubCode string
	// Set to true if this is from an all group
	IsAllGroup bool
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
	// true if matched a wildcard (xs:any)
	IsWildcard bool
	// processContents from the wildcard (only valid if IsWildcard)
	ProcessContents types.ProcessContents
	// matched declaration name (for non-wildcard matches)
	MatchedQName types.QName
	// compiled element pointer for the matched symbol (if available)
	MatchedElement *CompiledElement
}

// symbolCandidate represents a potential symbol match during content model schemacheck.
type symbolCandidate struct {
	idx        int
	isWildcard bool
}

func inBounds(idx, length int) bool {
	return idx >= 0 && idx < length
}

func symbolCountExceeded(symIdx int, counts []int, max int) bool {
	return max >= 0 && inBounds(symIdx, len(counts)) && counts[symIdx] > max
}

func symbolCount(counts []int, symIdx int) int {
	if !inBounds(symIdx, len(counts)) {
		return 0
	}
	return counts[symIdx]
}

func hasGroupCounters(a *Automaton, groups *groupCounterState) bool {
	return a != nil && a.groupCount > 0 && groups != nil
}

// Validate checks that children satisfy the content model.
// Returns nil if valid, or a ValidationError describing the first violation.
// Runs in O(n) time with no backtracking.
func (a *Automaton) Validate(doc *xsdxml.Document, children []xsdxml.NodeID, matcher SymbolMatcher) error {
	_, err := a.ValidateWithMatches(doc, children, matcher, nil)
	return err
}

// handleGroupCounters processes group iteration counting for the current transition.
// Returns an error if maxOccurs is exceeded.
func (a *Automaton) handleGroupCounters(state, next, symIdx, childIdx int, groups *groupCounterState) error {
	if a == nil || a.groupCount == 0 || groups == nil {
		return nil
	}
	lastProcessedGroupID := -1
	for _, checkState := range []int{state, next} {
		c := a.counting[checkState]
		if c == nil || !c.IsGroupCounter || c.GroupID == lastProcessedGroupID {
			continue
		}
		if !slices.Contains(c.GroupStartSymbols, symIdx) {
			continue
		}
		idx, ok := a.groupIndexByID[c.GroupID]
		if !ok {
			continue
		}
		lastProcessedGroupID = c.GroupID
		if err := a.incrementGroupCounter(c, idx, childIdx, groups); err != nil {
			return err
		}
	}
	return nil
}

func (a *Automaton) incrementGroupCounter(c *Counter, idx, childIdx int, groups *groupCounterState) error {
	groups.seen[idx] = true
	if c.UnitSize > 0 {
		return a.incrementGroupCounterUnit(c, idx, childIdx, groups)
	}
	groups.counts[idx]++
	minIterations := minGroupIterations(groups.counts[idx], c.FirstPosMaxOccurs)
	if c.Max >= 0 && minIterations > c.Max {
		return groupMaxOccursError(childIdx, c.Max)
	}
	return nil
}

func (a *Automaton) incrementGroupCounterUnit(c *Counter, idx, childIdx int, groups *groupCounterState) error {
	if c.Max >= 0 && groups.counts[idx] >= c.Max {
		return groupMaxOccursError(childIdx, c.Max)
	}
	groups.remainders[idx]++
	if groups.remainders[idx] != c.UnitSize {
		return nil
	}
	groups.counts[idx]++
	groups.remainders[idx] = 0
	if c.Max >= 0 && groups.counts[idx] > c.Max {
		return groupMaxOccursError(childIdx, c.Max)
	}
	return nil
}

func groupMaxOccursError(childIdx, max int) error {
	return &ValidationError{
		Index:   childIdx,
		Message: fmt.Sprintf("group exceeds maxOccurs=%d", max),
		SubCode: ErrorCodeNotExpectedHere,
	}
}

func minGroupIterations(startCount, firstPosMaxOccurs int) int {
	if startCount > 0 && firstPosMaxOccurs == types.UnboundedOccurs {
		return 1
	}
	if firstPosMaxOccurs > 1 {
		return (startCount + firstPosMaxOccurs - 1) / firstPosMaxOccurs
	}
	return startCount
}

// handleElementCounter processes element occurrence counting for the current match.
// Returns an error if maxOccurs is exceeded.
func (a *Automaton) handleElementCounter(state, next, symIdx, childIdx int, symbolCounts []int, childName string) error {
	if inBounds(symIdx, len(symbolCounts)) {
		symbolCounts[symIdx]++
	}
	max := types.UnboundedOccurs
	if inBounds(symIdx, len(a.symbolMax)) {
		max = a.symbolMax[symIdx]
	}
	if symbolCountExceeded(symIdx, symbolCounts, max) {
		return &ValidationError{
			Index:   childIdx,
			Message: fmt.Sprintf("element %q exceeds maxOccurs=%d", childName, max),
			SubCode: ErrorCodeNotExpectedHere,
		}
	}
	return nil
}

// validateFinalCounts checks all counters satisfy minOccurs at end of schemacheck.
func (a *Automaton) validateFinalCounts(symbolCounts []int, groups *groupCounterState, childCount int) error {
	if err := a.validateGroupCounts(groups, childCount); err != nil {
		return err
	}
	return a.validateSymbolCounts(symbolCounts, childCount)
}

func (a *Automaton) validateGroupCounts(groups *groupCounterState, childCount int) error {
	if !hasGroupCounters(a, groups) {
		return nil
	}
	clear(groups.checked)
	for stateID, c := range a.counting {
		if c == nil || !c.IsGroupCounter {
			continue
		}
		idx, ok := a.groupIndexByID[c.GroupID]
		if !ok || groups.checked[idx] {
			continue
		}
		groups.checked[idx] = true
		if c.UnitSize > 0 && groups.remainders[idx] != 0 {
			return &ValidationError{
				Index:   childCount,
				Message: fmt.Sprintf("group incomplete: expected %d occurrences per iteration", c.UnitSize),
				SubCode: ErrorCodeMissing,
			}
		}
		if groups.seen[idx] && groups.counts[idx] < c.Min {
			return &ValidationError{
				Index:   childCount,
				Message: fmt.Sprintf("minOccurs=%d not satisfied (state=%d, group=%d, count=%d)", c.Min, stateID, c.GroupID, groups.counts[idx]),
				SubCode: ErrorCodeMissing,
			}
		}
	}
	return nil
}

func (a *Automaton) validateSymbolCounts(symbolCounts []int, childCount int) error {
	for symIdx, min := range a.symbolMin {
		if min <= 0 {
			continue
		}
		count := symbolCount(symbolCounts, symIdx)
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
func (a *Automaton) ValidateWithMatches(doc *xsdxml.Document, children []xsdxml.NodeID, matcher SymbolMatcher, wildcards []*types.AnyElement) ([]MatchResult, error) {
	matches := make([]MatchResult, len(children))

	if len(children) == 0 {
		if a.emptyOK {
			return matches, nil
		}
		return nil, &ValidationError{Index: 0, Message: "content required but none found", SubCode: ErrorCodeMissing}
	}

	state := 0
	symbolCounts := make([]int, len(a.symbols))
	var groupState groupCounterState
	groupState.reset(a.groupCount)

	for i, child := range children {
		qname := types.QName{
			Namespace: types.NamespaceURI(doc.NamespaceURI(child)),
			Local:     doc.LocalName(child),
		}

		symIdx, isWildcard, next := a.findBestMatchQName(qname, state, matcher)

		if symIdx < 0 {
			// element is not part of the content model at all - not allowed (.d)
			return nil, &ValidationError{
				Index:   i,
				Message: fmt.Sprintf("element %q not allowed", doc.LocalName(child)),
				SubCode: ErrorCodeNotExpectedHere,
			}
		}

		matches[i].IsWildcard = isWildcard
		if isWildcard && len(wildcards) > 0 {
			matches[i].ProcessContents = a.findWildcardProcessContentsQName(qname, wildcards)
		} else if !isWildcard && inBounds(symIdx, len(a.symbols)) {
			matches[i].MatchedQName = a.symbols[symIdx].QName
			matches[i].MatchedElement = a.matchedElement(state, symIdx)
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
					Message: fmt.Sprintf("element %q not expected here", doc.LocalName(child)),
					SubCode: ErrorCodeNotExpectedHere,
				}
			}
			return nil, &ValidationError{
				Index:   i,
				Message: fmt.Sprintf("element %q not expected here", doc.LocalName(child)),
				SubCode: ErrorCodeMissing,
			}
		}

		if err := a.handleGroupCounters(state, next, symIdx, i, &groupState); err != nil {
			return nil, err
		}

		if err := a.handleElementCounter(state, next, symIdx, i, symbolCounts, doc.LocalName(child)); err != nil {
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

	if err := a.validateFinalCounts(symbolCounts, &groupState, len(children)); err != nil {
		return nil, err
	}

	return matches, nil
}

// findBestMatchQName finds the best matching symbol for an element at the given state.
// It returns the symbol index, whether it's a wildcard match, and the next state.
// If an element can match multiple symbols, it prefers the one with a valid transition.
func (a *Automaton) findBestMatchQName(qname types.QName, state int, matcher SymbolMatcher) (symIdx int, isWildcard bool, next int) {
	var candidatesBuf [8]symbolCandidate
	candidates := candidatesBuf[:0]

	for i, sym := range a.symbols {
		if sym.Kind == KindElement && sym.QName.Equal(qname) {
			candidates = append(candidates, symbolCandidate{i, false})
		}
	}

	if matcher != nil {
		for i, sym := range a.symbols {
			if sym.Kind == KindElement && sym.AllowSubstitution && matcher.IsSubstitutable(qname, sym.QName) {
				candidates = append(candidates, symbolCandidate{i, false})
			}
		}
	}

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
			if slices.Contains(sym.NSList, qname.Namespace) {
				candidates = append(candidates, symbolCandidate{i, true})
			}
		}
	}

	if len(candidates) == 0 {
		return -1, false, -1
	}

	// try to find a candidate with a valid transition
	for _, c := range candidates {
		nextState := a.transition(state, c.idx)
		if nextState >= 0 {
			return c.idx, c.isWildcard, nextState
		}
	}

	// no valid transition, return the first candidate (for error reporting)
	return candidates[0].idx, candidates[0].isWildcard, a.transition(state, candidates[0].idx)
}

func (a *Automaton) matchedElement(state, symIdx int) *CompiledElement {
	if a == nil || state < 0 || symIdx < 0 {
		return nil
	}
	if !inBounds(state, len(a.stateSymbolPos)) {
		return nil
	}
	row := a.stateSymbolPos[state]
	if !inBounds(symIdx, len(row)) {
		return nil
	}
	pos := row[symIdx]
	if !inBounds(pos, len(a.posElements)) {
		return nil
	}
	return a.posElements[pos]
}

// findWildcardProcessContentsQName finds the processContents for a wildcard match.
func (a *Automaton) findWildcardProcessContentsQName(qname types.QName, wildcards []*types.AnyElement) types.ProcessContents {
	if len(wildcards) == 0 {
		return types.Strict // default to strict if no wildcards available
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
