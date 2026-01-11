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
	Message    string
	SubCode    string
	Index      int
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
	MatchedElement  *CompiledElement
	MatchedQName    types.QName
	ProcessContents types.ProcessContents
	IsWildcard      bool
}

// symbolCandidate represents a potential symbol match during content model schemacheck.
type symbolCandidate struct {
	symbolIndex int
	isWildcard  bool
}

func inBounds(idx, length int) bool {
	return idx >= 0 && idx < length
}

func symbolCountExceeded(symbolIndex int, counts []int, max int) bool {
	return max >= 0 && inBounds(symbolIndex, len(counts)) && counts[symbolIndex] > max
}

func symbolCount(counts []int, symbolIndex int) int {
	if !inBounds(symbolIndex, len(counts)) {
		return 0
	}
	return counts[symbolIndex]
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
func (a *Automaton) handleGroupCounters(state, next, symbolIndex, childIdx int, groups *groupCounterState) error {
	if a == nil || a.groupCount == 0 || groups == nil {
		return nil
	}
	lastProcessedGroupID := -1
	// avoid double-counting when both states reference the same group counter
	for _, checkState := range []int{state, next} {
		c := a.counting[checkState]
		if c == nil || !c.IsGroupCounter || c.GroupID == lastProcessedGroupID {
			continue
		}
		if !slices.Contains(c.GroupStartSymbols, symbolIndex) {
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
		// ceil(startCount / firstPosMaxOccurs) using integer math
		return (startCount + firstPosMaxOccurs - 1) / firstPosMaxOccurs
	}
	return startCount
}

// handleElementCounter processes element occurrence counting for the current match.
// Returns an error if maxOccurs is exceeded.
func (a *Automaton) handleElementCounter(state, next, symbolIndex, childIdx int, symbolCounts []int, childName string) error {
	if inBounds(symbolIndex, len(symbolCounts)) {
		symbolCounts[symbolIndex]++
	}
	max := types.UnboundedOccurs
	if inBounds(symbolIndex, len(a.symbolMax)) {
		max = a.symbolMax[symbolIndex]
	}
	if symbolCountExceeded(symbolIndex, symbolCounts, max) {
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
	for symbolIndex, min := range a.symbolMin {
		if min <= 0 {
			continue
		}
		count := symbolCount(symbolCounts, symbolIndex)
		if count < min {
			return &ValidationError{
				Index:   childCount,
				Message: fmt.Sprintf("minOccurs=%d not satisfied (symbol=%d, count=%d)", min, symbolIndex, count),
				SubCode: ErrorCodeMissing,
			}
		}
	}
	return nil
}

type validationState struct {
	groups       groupCounterState
	symbolCounts []int
	currentState int
}

// ValidateWithMatches validates children and returns match results for each child.
// This allows the caller to determine how each child was matched (element vs wildcard).
func (a *Automaton) ValidateWithMatches(doc *xsdxml.Document, children []xsdxml.NodeID, matcher SymbolMatcher, wildcards []*types.AnyElement) ([]MatchResult, error) {
	matches := make([]MatchResult, len(children))

	if done, err := a.validateEmptyContent(len(children)); done {
		if err != nil {
			return nil, err
		}
		return matches, nil
	}

	state := a.initValidationState()

	for i, child := range children {
		if err := a.processChild(doc, child, i, matcher, wildcards, state, &matches[i]); err != nil {
			return nil, err
		}
	}

	if err := a.validateEndState(state, len(children)); err != nil {
		return nil, err
	}
	return matches, nil
}

func (a *Automaton) validateEmptyContent(childCount int) (bool, error) {
	if childCount > 0 {
		return false, nil
	}
	if a.emptyOK {
		return true, nil
	}
	return true, &ValidationError{Index: 0, Message: "content required but none found", SubCode: ErrorCodeMissing}
}

func (a *Automaton) initValidationState() *validationState {
	state := &validationState{
		currentState: 0,
		symbolCounts: make([]int, len(a.symbols)),
	}
	state.groups.reset(a.groupCount)
	return state
}

func (a *Automaton) processChild(doc *xsdxml.Document, child xsdxml.NodeID, childIdx int, matcher SymbolMatcher, wildcards []*types.AnyElement, state *validationState, match *MatchResult) error {
	qname := types.QName{
		Namespace: types.NamespaceURI(doc.NamespaceURI(child)),
		Local:     doc.LocalName(child),
	}

	symbolIndex, isWildcard, nextState := a.findBestMatchQName(qname, state.currentState, matcher)
	if symbolIndex < 0 {
		return a.elementNotAllowedError(doc.LocalName(child), childIdx)
	}

	a.recordMatch(match, symbolIndex, isWildcard, qname, state.currentState, wildcards)

	if nextState < 0 {
		return a.noValidTransitionError(doc.LocalName(child), childIdx, state.currentState)
	}

	if err := a.handleGroupCounters(state.currentState, nextState, symbolIndex, childIdx, &state.groups); err != nil {
		return err
	}
	if err := a.handleElementCounter(state.currentState, nextState, symbolIndex, childIdx, state.symbolCounts, doc.LocalName(child)); err != nil {
		return err
	}
	state.currentState = nextState
	return nil
}

func (a *Automaton) recordMatch(match *MatchResult, symbolIndex int, isWildcard bool, qname types.QName, state int, wildcards []*types.AnyElement) {
	match.IsWildcard = isWildcard
	if isWildcard && len(wildcards) > 0 {
		match.ProcessContents = a.findWildcardProcessContentsQName(qname, wildcards)
		return
	}
	if !isWildcard && inBounds(symbolIndex, len(a.symbols)) {
		match.MatchedQName = a.symbols[symbolIndex].QName
		match.MatchedElement = a.matchedElement(state, symbolIndex)
	}
}

func (a *Automaton) elementNotAllowedError(localName string, childIdx int) error {
	return &ValidationError{
		Index:   childIdx,
		Message: fmt.Sprintf("element %q not allowed", localName),
		SubCode: ErrorCodeNotExpectedHere,
	}
}

func (a *Automaton) noValidTransitionError(localName string, childIdx int, state int) error {
	isAccepting := (state < len(a.accepting) && a.accepting[state]) ||
		(state == 0 && a.emptyOK)
	if isAccepting {
		return &ValidationError{
			Index:   childIdx,
			Message: fmt.Sprintf("element %q not expected here", localName),
			SubCode: ErrorCodeNotExpectedHere,
		}
	}
	return &ValidationError{
		Index:   childIdx,
		Message: fmt.Sprintf("element %q not expected here", localName),
		SubCode: ErrorCodeMissing,
	}
}

func (a *Automaton) validateEndState(state *validationState, childCount int) error {
	if !a.accepting[state.currentState] {
		return &ValidationError{
			Index:   childCount,
			Message: "content incomplete: required elements missing",
			SubCode: ErrorCodeMissing,
		}
	}
	return a.validateFinalCounts(state.symbolCounts, &state.groups, childCount)
}

// findBestMatchQName finds the best matching symbol for an element at the given state.
// It returns the symbol index, whether it's a wildcard match, and the next state.
// If an element can match multiple symbols, it prefers the one with a valid transition.
func (a *Automaton) findBestMatchQName(qname types.QName, state int, matcher SymbolMatcher) (symbolIndex int, isWildcard bool, next int) {
	var candidatesBuf [8]symbolCandidate
	candidates := candidatesBuf[:0]

	candidates = a.collectExactMatches(qname, candidates)
	candidates = a.collectSubstitutionMatches(qname, matcher, candidates)
	candidates = a.collectWildcardMatches(qname, candidates)

	if len(candidates) == 0 {
		return -1, false, -1
	}

	return a.selectBestCandidate(candidates, state)
}

func (a *Automaton) collectExactMatches(qname types.QName, candidates []symbolCandidate) []symbolCandidate {
	for i, sym := range a.symbols {
		if sym.Kind == KindElement && sym.QName.Equal(qname) {
			candidates = append(candidates, symbolCandidate{i, false})
		}
	}
	return candidates
}

func (a *Automaton) collectSubstitutionMatches(qname types.QName, matcher SymbolMatcher, candidates []symbolCandidate) []symbolCandidate {
	if matcher == nil {
		return candidates
	}
	for i, sym := range a.symbols {
		if sym.Kind == KindElement && sym.AllowSubstitution && matcher.IsSubstitutable(qname, sym.QName) {
			candidates = append(candidates, symbolCandidate{i, false})
		}
	}
	return candidates
}

func (a *Automaton) collectWildcardMatches(qname types.QName, candidates []symbolCandidate) []symbolCandidate {
	for i, sym := range a.symbols {
		if a.wildcardMatches(sym, qname) {
			candidates = append(candidates, symbolCandidate{i, true})
		}
	}
	return candidates
}

func (a *Automaton) wildcardMatches(sym Symbol, qname types.QName) bool {
	switch sym.Kind {
	case KindAny:
		return true
	case KindAnyNS:
		return string(qname.Namespace) == sym.NS
	case KindAnyOther:
		// ##other matches any namespace other than target namespace AND not empty
		elemNS := string(qname.Namespace)
		return elemNS != sym.NS && elemNS != ""
	case KindAnyNSList:
		return slices.Contains(sym.NSList, qname.Namespace)
	default:
		return false
	}
}

func (a *Automaton) selectBestCandidate(candidates []symbolCandidate, state int) (symbolIndex int, isWildcard bool, next int) {
	for _, c := range candidates {
		nextState := a.transition(state, c.symbolIndex)
		if nextState >= 0 {
			return c.symbolIndex, c.isWildcard, nextState
		}
	}
	first := candidates[0]
	return first.symbolIndex, first.isWildcard, a.transition(state, first.symbolIndex)
}

func (a *Automaton) matchedElement(state, symbolIndex int) *CompiledElement {
	if a == nil || state < 0 || symbolIndex < 0 {
		return nil
	}
	if !inBounds(state, len(a.stateSymbolPos)) {
		return nil
	}
	row := a.stateSymbolPos[state]
	if !inBounds(symbolIndex, len(row)) {
		return nil
	}
	pos := row[symbolIndex]
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
