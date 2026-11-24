package schema

import (
	"cmp"
	"errors"
	"math"
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

// substitutionCycleError reports a cyclic substitution group rooted at Element.
type substitutionCycleError struct {
	Element ElementID
}

// substitutionClosureLimitError reports that the aggregate number of raw
// head/member ancestor pairs exceeds the configured bound.
type substitutionClosureLimitError struct {
	Limit int
}

// substitutionMembershipError identifies an invalid direct member/head edge.
type substitutionMembershipError struct {
	Cause  error
	Member ElementID
	Head   ElementID
}

var (
	// errSubstitutionMemberTypeNotDerived reports that a substitution member's
	// type is not derived from the substitution head's type.
	errSubstitutionMemberTypeNotDerived = errors.New("substitution member type is not derived from head")
	// errSubstitutionMemberTypeExcludedDerivation reports that the head's final
	// constraint blocks the member's derivation path.
	errSubstitutionMemberTypeExcludedDerivation = errors.New("substitution member type uses excluded derivation")
)

// substitutionTable is the immutable substitution-group projection.
// Compilation reads it over build element IDs, and publication copies it over
// published element IDs (published). Its slices are intentionally private.
// Each head's span of entries is in name-ID order for lookup; byName holds the
// same span's entry positions in expanded-name order (namespace URI, then
// local name), the order of every check that stops at its first failure and of
// every diagnostic that lists members.
type substitutionTable struct {
	spans   []substitutionSpan
	entries []substitutionEntry
	byName  []uint32
}

type substitutionSpan struct {
	start int
	count int
}

type substitutionEntry struct {
	name      QName
	member    ElementID
	effective bool
}

// buildSubstitutionTable validates the direct substitution forest and builds a
// bounded transitive table. maxClosureEntries counts every raw ancestor pair,
// including entries later excluded from effective name matching.
func buildSubstitutionTable(
	rt *schemaBuild,
	names *nameTable,
	elements []elementDecl,
	globals map[QName]ElementID,
	maxClosureEntries int,
	work func(int) error,
) (substitutionTable, error) {
	if maxClosureEntries < 0 {
		return substitutionTable{}, errors.New("substitution closure entry limit must be non-negative")
	}
	forest, err := validateSubstitutionForest(names, elements, globals)
	if err != nil {
		return substitutionTable{}, err
	}
	if forest.total == 0 {
		return substitutionTable{}, nil
	}
	if limit := substitutionClosureLimit(maxClosureEntries); forest.total > limit {
		return substitutionTable{}, substitutionClosureLimitError{Limit: limit}
	}
	edges, err := validateSubstitutionEdges(rt, elements, forest.parents, work)
	if err != nil {
		return substitutionTable{}, err
	}
	counts := substitutionAncestorCounts(forest.parents)
	spans, entries, next := allocateSubstitutionSpans(counts, forest.total)
	populateSubstitutionEntries(entries, next, elements, forest.parents, edges)
	return newSubstitutionTable(spans, entries, func(a, b QName) int {
		return compareQNameText(names, a, b)
	}), nil
}

func substitutionAncestorCounts(parents []ElementID) []int {
	counts := make([]int, len(parents))
	for member := range parents {
		for head := parents[member]; head != NoElement; head = parents[head] {
			counts[head]++
		}
	}
	return counts
}

func allocateSubstitutionSpans(counts []int, total int) ([]substitutionSpan, []substitutionEntry, []int) {
	spans := make([]substitutionSpan, len(counts))
	next := make([]int, len(counts))
	start := 0
	for head, count := range counts {
		spans[head] = substitutionSpan{start: start, count: count}
		next[head] = start
		start += count
	}
	return spans, make([]substitutionEntry, total), next
}

func populateSubstitutionEntries(
	entries []substitutionEntry,
	next []int,
	elements []elementDecl,
	parents []ElementID,
	edges []substitutionEdge,
) {
	for member := range elements {
		memberID := ElementID(member)
		memberDecl := elements[member]
		var methods, blocks DerivationMask
		for current, head := memberID, parents[member]; head != NoElement; current, head = head, parents[head] {
			methods |= edges[current].methods
			blocks |= edges[current].blocks
			pos := next[head]
			entries[pos] = substitutionEntry{
				name:      memberDecl.Name,
				member:    memberID,
				effective: substitutionEffective(elements[head], memberDecl, methods, blocks),
			}
			next[head]++
		}
	}
}

// newSubstitutionTable is the only constructor of a non-empty table. It takes
// ownership of entries, sorts each span of them by name ID, and records their
// order under compareNames, the expanded-name order, in byName. Sorting is
// bounded by MaxSubstitutionClosureEntries, which buildSubstitutionTable checks
// before any entry exists; it is not charged to derivation work, which is
// bounded by direct edges.
func newSubstitutionTable(spans []substitutionSpan, entries []substitutionEntry, compareNames func(a, b QName) int) substitutionTable {
	ranked, rank := rankSubstitutionMembers(entries, compareNames)
	position := make([]uint32, len(rank))
	byName := make([]uint32, len(entries))
	for i := range byName {
		byName[i] = uint32(i)
	}
	for _, span := range spans {
		spanEntries := entries[span.start : span.start+span.count]
		slices.SortFunc(spanEntries, compareSubstitutionEntry)
		spanByName := byName[span.start : span.start+span.count]
		for i, entry := range spanEntries {
			position[entry.member] = spanByName[i]
			spanByName[i] = rank[entry.member]
		}
		slices.Sort(spanByName)
		for i, r := range spanByName {
			spanByName[i] = position[ranked[r].member]
		}
	}
	return substitutionTable{spans: spans, entries: entries, byName: byName}
}

// rankSubstitutionMembers returns each distinct member of entries once, ordered
// by compareNames and then by member ID, and the rank of each member ID in that
// order, so spans sort by integer rank instead of comparing names again. A
// member is in a span at most once, so its rank orders the span.
func rankSubstitutionMembers(entries []substitutionEntry, compareNames func(a, b QName) int) ([]substitutionEntry, []uint32) {
	var members ElementID
	for _, entry := range entries {
		members = max(members, entry.member+1)
	}
	seen := make([]bool, members)
	var ranked []substitutionEntry
	for _, entry := range entries {
		if !seen[entry.member] {
			seen[entry.member] = true
			ranked = append(ranked, entry)
		}
	}
	slices.SortFunc(ranked, func(a, b substitutionEntry) int {
		return cmp.Or(compareNames(a.name, b.name), cmp.Compare(a.member, b.member))
	})
	rank := make([]uint32, members)
	for i, entry := range ranked {
		rank[entry.member] = uint32(i)
	}
	return ranked, rank
}

// published returns the table over published element IDs, with its own
// spans and entries and sharing byName with t. Every head and member is a
// global declaration, which is reached, so each has a published ID unless the
// build is corrupt. The numbering is monotone, so each span keeps its order.
func (t substitutionTable) published(elements publishedElements) (substitutionTable, error) {
	entries := make([]substitutionEntry, len(t.entries))
	for i, entry := range t.entries {
		id, ok := elements.id(entry.member)
		if !ok {
			return substitutionTable{}, xsderrors.InternalInvariant("substitution member has no published ID")
		}
		entry.member = id
		entries[i] = entry
	}
	if len(t.spans) == 0 {
		return substitutionTable{entries: entries, byName: t.byName}, nil
	}
	if len(t.spans) != elements.declarations {
		return substitutionTable{}, xsderrors.InternalInvariant("substitution table does not cover the element declarations")
	}
	spans := make([]substitutionSpan, elements.count)
	for head, span := range t.spans {
		id, ok := elements.id(ElementID(head)) // #nosec G115 -- spans is an element-declaration table.
		if ok {
			spans[id] = span
		} else if span.count != 0 {
			return substitutionTable{}, xsderrors.InternalInvariant("substitution head has no published ID")
		}
	}
	return substitutionTable{spans: spans, entries: entries, byName: t.byName}, nil
}

// forEachEntryByName iterates head's raw transitive entries in expanded-name
// order until fn returns false.
func (t substitutionTable) forEachEntryByName(head ElementID, fn func(substitutionEntry) bool) {
	span, ok := t.span(head)
	if !ok || span.count == 0 {
		return
	}
	for _, position := range t.byName[span.start : span.start+span.count] {
		if !fn(t.entries[position]) {
			return
		}
	}
}

// ForEachMember iterates all raw transitive members for head in expanded-name
// order until fn returns false. Abstract and blocked members remain visible to
// compilation checks.
func (t substitutionTable) ForEachMember(head ElementID, fn func(ElementID) bool) {
	t.forEachEntryByName(head, func(entry substitutionEntry) bool {
		return fn(entry.member)
	})
}

// ForEachEntry iterates effective name/member entries for head until fn returns
// false.
func (t substitutionTable) ForEachEntry(head ElementID, fn func(QName, ElementID) bool) {
	span, ok := t.span(head)
	if !ok || span.count == 0 {
		return
	}
	for _, entry := range t.entries[span.start : span.start+span.count] {
		if entry.effective && !fn(entry.name, entry.member) {
			return
		}
	}
}

// MemberByName returns the effective substitution member registered under head.
func (t substitutionTable) MemberByName(head ElementID, name QName) (ElementID, bool) {
	span, ok := t.span(head)
	if !ok || span.count == 0 {
		return NoElement, false
	}
	entries := t.entries[span.start : span.start+span.count]
	position, found := slices.BinarySearchFunc(entries, name, func(entry substitutionEntry, target QName) int {
		return compareQName(entry.name, target)
	})
	if !found || !entries[position].effective {
		return NoElement, false
	}
	return entries[position].member, true
}

// HasMembers reports whether head has raw transitive substitution members.
func (t substitutionTable) HasMembers(head ElementID) bool {
	span, ok := t.span(head)
	return ok && span.count != 0
}

func (t substitutionTable) span(head ElementID) (substitutionSpan, bool) {
	if !validElementID(head, len(t.spans)) {
		return substitutionSpan{}, false
	}
	span := t.spans[head]
	if span.start < 0 || span.count < 0 || span.start > len(t.entries) || span.count > len(t.entries)-span.start {
		return substitutionSpan{}, false
	}
	return span, true
}

// substitutionClosureLimit is maxClosureEntries, capped at maxUint32Index, the
// largest uint32 byName position, so the configured limit is the only bound a
// closure meets.
func substitutionClosureLimit(maxClosureEntries int) int {
	return min(maxClosureEntries, maxUint32Index)
}

// validNameOrder reports whether byName holds, for every span, a permutation
// of that span's entry positions. It reads the table only, marking the
// positions each span has seen in one scratch slice over all entries: a
// bitset would chain each mark on the word the previous mark wrote.
func (t substitutionTable) validNameOrder() bool {
	if len(t.byName) != len(t.entries) {
		return false
	}
	seen := make([]bool, len(t.entries))
	for head := range t.spans {
		span, ok := t.span(ElementID(head))
		if !ok || !t.spanNameOrderIsPermutation(span, seen) {
			return false
		}
	}
	return true
}

// spanNameOrderIsPermutation reports whether the byName slots of span hold a
// permutation of its positions, using seen, which is clear on entry and on
// return, to find a repeated position.
func (t substitutionTable) spanNameOrderIsPermutation(span substitutionSpan, seen []bool) bool {
	positions := t.byName[span.start : span.start+span.count]
	permutation := true
	marked := 0
	for _, value := range positions {
		position := int(value)
		if position < span.start || position >= span.start+span.count || seen[position] {
			permutation = false
			break
		}
		seen[position] = true
		marked++
	}
	for _, value := range positions[:marked] {
		seen[value] = false
	}
	return permutation
}

type substitutionForest struct {
	parents []ElementID
	total   int
}

func validateSubstitutionForest(
	names *nameTable,
	elements []elementDecl,
	globals map[QName]ElementID,
) (substitutionForest, error) {
	if names == nil {
		return substitutionForest{}, errors.New("substitution table requires name table")
	}
	parents, hasHeads, err := validateSubstitutionParents(names, elements, globals)
	if err != nil {
		return substitutionForest{}, err
	}
	if !hasHeads {
		return substitutionForest{}, nil
	}
	depth, err := substitutionForestDepths(parents)
	if err != nil {
		return substitutionForest{}, err
	}
	total, err := substitutionClosureSize(depth)
	if err != nil {
		return substitutionForest{}, err
	}
	return substitutionForest{parents: parents, total: total}, nil
}

func validateSubstitutionParents(
	names *nameTable,
	elements []elementDecl,
	globals map[QName]ElementID,
) ([]ElementID, bool, error) {
	parents := make([]ElementID, len(elements))
	for i := range parents {
		parents[i] = NoElement
	}
	hasHeads := false
	for index, member := range elements {
		parent, err := validateSubstitutionParent(names, elements, globals, index, member)
		if err != nil {
			return nil, false, err
		}
		if parent != NoElement {
			hasHeads = true
			parents[index] = parent
		}
	}
	return parents, hasHeads, nil
}

func validateSubstitutionParent(
	names *nameTable,
	elements []elementDecl,
	globals map[QName]ElementID,
	index int,
	member elementDecl,
) (ElementID, error) {
	if member.SubstHead == NoElement {
		return NoElement, nil
	}
	memberID, ok := elementIndexID(index)
	if !ok {
		return NoElement, errors.New("substitution member element ID is invalid")
	}
	if !names.ValidQName(member.Name) {
		return NoElement, errors.New("substitution member name is invalid")
	}
	globalMember, ok := globals[member.Name]
	if !ok || globalMember != memberID {
		return NoElement, errors.New("substitution member is not a global element")
	}
	if !validSubstitutionElementID(elements, member.SubstHead) {
		return NoElement, errors.New("element declaration references invalid substitution head")
	}
	head := elements[member.SubstHead]
	if !names.ValidQName(head.Name) {
		return NoElement, errors.New("substitution head name is invalid")
	}
	globalHead, ok := globals[head.Name]
	if !ok || globalHead != member.SubstHead {
		return NoElement, errors.New("substitution head is not a global element")
	}
	return member.SubstHead, nil
}

func substitutionForestDepths(parents []ElementID) ([]int, error) {
	state := make([]uint8, len(parents))
	depth := make([]int, len(parents))
	path := make([]ElementID, 0, len(parents))
	for start := range parents {
		if state[start] == 2 {
			continue
		}
		var err error
		path, err = traceSubstitutionPath(ElementID(start), parents, state, path[:0])
		if err != nil {
			return nil, err
		}
		completeSubstitutionPath(path, parents, state, depth)
	}
	return depth, nil
}

func traceSubstitutionPath(
	start ElementID,
	parents []ElementID,
	state []uint8,
	path []ElementID,
) ([]ElementID, error) {
	for current := start; current != NoElement && state[current] != 2; current = parents[current] {
		if state[current] == 1 {
			return nil, substitutionCycleError{Element: current}
		}
		state[current] = 1
		path = append(path, current)
	}
	return path, nil
}

func completeSubstitutionPath(path, parents []ElementID, state []uint8, depth []int) {
	for _, current := range slices.Backward(path) {
		if parent := parents[current]; parent != NoElement {
			depth[current] = depth[parent] + 1
		}
		state[current] = 2
	}
}

func substitutionClosureSize(depth []int) (int, error) {
	total := 0
	for _, ancestors := range depth {
		if ancestors > math.MaxInt-total {
			return 0, substitutionClosureLimitError{Limit: math.MaxInt}
		}
		total += ancestors
	}
	return total, nil
}

type substitutionEdge struct {
	methods DerivationMask
	blocks  DerivationMask
}

func validateSubstitutionEdges(
	rt *schemaBuild,
	elements []elementDecl,
	parents []ElementID,
	work func(int) error,
) ([]substitutionEdge, error) {
	edges := make([]substitutionEdge, len(elements))
	var memberWalk unionMemberWalk
	for member, head := range parents {
		if head == NoElement {
			continue
		}
		edge, err := validateSubstitutionEdge(rt, elements, member, head, &memberWalk, work)
		if err != nil {
			return nil, err
		}
		edges[member] = edge
	}
	return edges, nil
}

func validateSubstitutionEdge(
	rt *schemaBuild,
	elements []elementDecl,
	member int,
	head ElementID,
	memberWalk *unionMemberWalk,
	work func(int) error,
) (substitutionEdge, error) {
	memberID, ok := elementIndexID(member)
	if !ok {
		return substitutionEdge{}, errors.New("substitution member index exceeds element ID range")
	}
	derivation, ok, err := deriveSubstitution(rt, elements[member].Type, elements[head].Type, memberWalk, work)
	if err != nil {
		return substitutionEdge{}, err
	}
	if !ok {
		return substitutionEdge{}, substitutionMembershipError{
			Cause:  errSubstitutionMemberTypeNotDerived,
			Member: memberID,
			Head:   head,
		}
	}
	if elements[head].Final&derivation.full != 0 {
		return substitutionEdge{}, substitutionMembershipError{
			Cause:  errSubstitutionMemberTypeExcludedDerivation,
			Member: memberID,
			Head:   head,
		}
	}
	blocks, err := substitutionTypeBlocks(rt, elements[member].Type, elements[head].Type, work)
	if err != nil {
		return substitutionEdge{}, err
	}
	return substitutionEdge{methods: derivation.blocking, blocks: blocks}, nil
}

func compareSubstitutionEntry(a, b substitutionEntry) int {
	if order := compareQName(a.name, b.name); order != 0 {
		return order
	}
	return int64Compare(int64(a.member), int64(b.member))
}

func compareQName(a, b QName) int {
	if a.Namespace < b.Namespace {
		return -1
	}
	if a.Namespace > b.Namespace {
		return 1
	}
	if a.Local < b.Local {
		return -1
	}
	if a.Local > b.Local {
		return 1
	}
	return 0
}

func int64Compare(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Error returns the stable substitution-cycle message.
//
//nolint:revive // The receiver is required by error and carries type identity.
func (e substitutionCycleError) Error() string {
	return "cyclic substitution group"
}

// Error returns the stable substitution closure limit message.
//
//nolint:revive // The receiver is required by error and carries type identity.
func (e substitutionClosureLimitError) Error() string {
	return "substitution closure entry limit exceeded"
}

// Error returns the stable invalid-membership message.
//
//nolint:revive // The receiver is required by error and carries type identity.
func (e substitutionMembershipError) Error() string {
	return "substitution member is not allowed by head"
}

// Unwrap returns the precise direct-membership failure.
func (e substitutionMembershipError) Unwrap() error {
	return e.Cause
}

func substitutionEffective(head, member elementDecl, mask, typeBlocks DerivationMask) bool {
	if member.Abstract {
		return false
	}
	if head.Block&derivationSubstitution != 0 {
		return false
	}
	return mask&head.Block == 0 && mask&typeBlocks == 0
}

func elementIndexID(id int) (ElementID, bool) {
	runtimeID, ok := newRuntimeID(id)
	if !ok {
		return NoElement, false
	}
	return ElementID(runtimeID), true
}

func validSubstitutionElementID(elements []elementDecl, id ElementID) bool {
	return validElementID(id, len(elements))
}
