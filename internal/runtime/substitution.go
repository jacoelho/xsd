package runtime

import (
	"errors"
	"slices"
)

// SubstitutionCycleError reports a cyclic substitution group rooted at Element.
type SubstitutionCycleError struct {
	Element ElementID
}

// SubstitutionClosureLimitError reports that the aggregate number of raw
// head/member ancestor pairs exceeds the configured bound.
type SubstitutionClosureLimitError struct {
	Limit int
}

// SubstitutionMembershipError identifies an invalid direct member/head edge.
type SubstitutionMembershipError struct {
	Cause  error
	Member ElementID
	Head   ElementID
}

var (
	// ErrSubstitutionMemberTypeNotDerived reports that a substitution member's
	// type is not derived from the substitution head's type.
	ErrSubstitutionMemberTypeNotDerived = errors.New("substitution member type is not derived from head")
	// ErrSubstitutionMemberTypeExcludedDerivation reports that the head's final
	// constraint blocks the member's derivation path.
	ErrSubstitutionMemberTypeExcludedDerivation = errors.New("substitution member type uses excluded derivation")
)

// SubstitutionTable is the immutable substitution-group projection shared by
// compilation and published validation. Its slices are intentionally private.
type SubstitutionTable struct {
	spans   []substitutionSpan
	entries []substitutionEntry
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

// BuildSubstitutionTable validates the direct substitution forest and builds a
// bounded transitive table. maxClosureEntries counts every raw ancestor pair,
// including entries later excluded from effective name matching.
func BuildSubstitutionTable(
	rt TypeDerivationRuntime,
	names *NameTable,
	elements []ElementDecl,
	globals map[QName]ElementID,
	maxClosureEntries int,
) (SubstitutionTable, error) {
	if maxClosureEntries < 0 {
		return SubstitutionTable{}, errors.New("substitution closure entry limit must be non-negative")
	}
	forest, err := validateSubstitutionForest(names, elements, globals)
	if err != nil {
		return SubstitutionTable{}, err
	}
	if forest.total == 0 {
		return SubstitutionTable{}, nil
	}
	if forest.total > maxClosureEntries {
		return SubstitutionTable{}, SubstitutionClosureLimitError{Limit: maxClosureEntries}
	}
	edges, err := validateSubstitutionEdges(rt, elements, forest.parents)
	if err != nil {
		return SubstitutionTable{}, err
	}

	counts := make([]int, len(elements))
	for member := range elements {
		for head := forest.parents[member]; head != NoElement; head = forest.parents[head] {
			counts[head]++
		}
	}
	table := SubstitutionTable{
		spans:   make([]substitutionSpan, len(elements)),
		entries: make([]substitutionEntry, forest.total),
	}
	next := make([]int, len(elements))
	start := 0
	for head, count := range counts {
		table.spans[head] = substitutionSpan{start: start, count: count}
		next[head] = start
		start += count
	}
	for member := range elements {
		memberID := ElementID(member)
		memberDecl, _ := ElementDeclByID(elements, memberID)
		var mask, blocks DerivationMask
		for current, head := memberID, forest.parents[member]; head != NoElement; current, head = head, forest.parents[head] {
			mask |= edges[current].mask
			blocks |= edges[current].blocks
			headDecl, _ := ElementDeclByID(elements, head)
			pos := next[head]
			table.entries[pos] = substitutionEntry{
				name:      memberDecl.Name,
				member:    memberID,
				effective: substitutionEffective(*headDecl, *memberDecl, mask, blocks),
			}
			next[head]++
		}
	}
	for _, span := range table.spans {
		entries := table.entries[span.start : span.start+span.count]
		slices.SortFunc(entries, compareSubstitutionEntry)
	}
	return table, nil
}

// ValidateSubstitutionTable independently audits a constructed table without
// allocating another closure-sized representation.
func ValidateSubstitutionTable(
	rt TypeDerivationRuntime,
	names *NameTable,
	elements []ElementDecl,
	globals map[QName]ElementID,
	table SubstitutionTable,
) error {
	forest, err := validateSubstitutionForest(names, elements, globals)
	if err != nil {
		return err
	}
	if forest.total == 0 {
		if table.spans != nil || table.entries != nil {
			return errors.New("substitution table is non-zero without substitution heads")
		}
		return nil
	}
	if len(table.spans) != len(elements) || len(table.entries) != forest.total {
		return errors.New("substitution table shape does not match substitution forest")
	}
	edges, err := validateSubstitutionEdges(rt, elements, forest.parents)
	if err != nil {
		return err
	}

	orderedMembers := make([]ElementID, 0, len(elements))
	for i, parent := range forest.parents {
		if parent != NoElement {
			orderedMembers = append(orderedMembers, ElementID(i))
		}
	}
	slices.SortFunc(orderedMembers, func(a, b ElementID) int {
		return compareQName(elements[a].Name, elements[b].Name)
	})

	cursors := make([]int, len(elements))
	start := 0
	for head := range elements {
		span := table.spans[head]
		if span.start != start || span.count < 0 || span.count > len(table.entries)-start {
			return errors.New("substitution table has invalid span")
		}
		cursors[head] = start
		start += span.count
	}
	if start != len(table.entries) {
		return errors.New("substitution table spans do not cover entries")
	}
	for _, member := range orderedMembers {
		var mask, blocks DerivationMask
		for current, head := member, forest.parents[member]; head != NoElement; current, head = head, forest.parents[head] {
			mask |= edges[current].mask
			blocks |= edges[current].blocks
			span := table.spans[head]
			position := cursors[head]
			if position >= span.start+span.count {
				return errors.New("substitution table is missing ancestor pair")
			}
			expected := substitutionEntry{
				name:      elements[member].Name,
				member:    member,
				effective: substitutionEffective(elements[head], elements[member], mask, blocks),
			}
			if table.entries[position] != expected {
				return errors.New("substitution table entry does not match substitution forest")
			}
			cursors[head]++
		}
	}
	for head, position := range cursors {
		span := table.spans[head]
		if position != span.start+span.count {
			return errors.New("substitution table has unexpected ancestor pair")
		}
	}
	return nil
}

// ForEachMember iterates all raw transitive members for head until fn returns
// false. Abstract and blocked members remain visible to compilation checks.
func (t SubstitutionTable) ForEachMember(head ElementID, fn func(ElementID) bool) {
	span, ok := t.span(head)
	if !ok {
		return
	}
	for _, entry := range t.entries[span.start : span.start+span.count] {
		if !fn(entry.member) {
			return
		}
	}
}

// ForEachEntry iterates effective name/member entries for head until fn returns
// false.
func (t SubstitutionTable) ForEachEntry(head ElementID, fn func(QName, ElementID) bool) {
	span, ok := t.span(head)
	if !ok {
		return
	}
	for _, entry := range t.entries[span.start : span.start+span.count] {
		if entry.effective && !fn(entry.name, entry.member) {
			return
		}
	}
}

// MemberByName returns the effective substitution member registered under head.
func (t SubstitutionTable) MemberByName(head ElementID, name QName) (ElementID, bool) {
	span, ok := t.span(head)
	if !ok {
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
func (t SubstitutionTable) HasMembers(head ElementID) bool {
	span, ok := t.span(head)
	return ok && span.count != 0
}

func (t SubstitutionTable) span(head ElementID) (substitutionSpan, bool) {
	if !ValidElementID(head, len(t.spans)) {
		return substitutionSpan{}, false
	}
	span := t.spans[head]
	if span.start < 0 || span.count < 0 || span.start > len(t.entries) || span.count > len(t.entries)-span.start {
		return substitutionSpan{}, false
	}
	return span, true
}

type substitutionForest struct {
	parents []ElementID
	total   int
}

func validateSubstitutionForest(
	names *NameTable,
	elements []ElementDecl,
	globals map[QName]ElementID,
) (substitutionForest, error) {
	if names == nil {
		return substitutionForest{}, errors.New("substitution table requires name table")
	}
	parents := make([]ElementID, len(elements))
	for i := range parents {
		parents[i] = NoElement
	}
	hasHeads := false
	for index, member := range elements {
		if member.SubstHead == NoElement {
			continue
		}
		hasHeads = true
		memberID, ok := elementIndexID(index)
		if !ok {
			return substitutionForest{}, errors.New("substitution member element ID is invalid")
		}
		if !names.ValidQName(member.Name) {
			return substitutionForest{}, errors.New("substitution member name is invalid")
		}
		globalMember, ok := globals[member.Name]
		if !ok || globalMember != memberID {
			return substitutionForest{}, errors.New("substitution member is not a global element")
		}
		if !validSubstitutionElementID(elements, member.SubstHead) {
			return substitutionForest{}, errors.New("element declaration references invalid substitution head")
		}
		head := elements[member.SubstHead]
		if !names.ValidQName(head.Name) {
			return substitutionForest{}, errors.New("substitution head name is invalid")
		}
		globalHead, ok := globals[head.Name]
		if !ok || globalHead != member.SubstHead {
			return substitutionForest{}, errors.New("substitution head is not a global element")
		}
		parents[index] = member.SubstHead
	}
	if !hasHeads {
		return substitutionForest{}, nil
	}

	state := make([]uint8, len(elements))
	depth := make([]int, len(elements))
	path := make([]ElementID, 0, len(elements))
	for start := range elements {
		if state[start] == 2 {
			continue
		}
		path = path[:0]
		for current := ElementID(start); current != NoElement && state[current] != 2; current = parents[current] {
			if state[current] == 1 {
				return substitutionForest{}, SubstitutionCycleError{Element: current}
			}
			state[current] = 1
			path = append(path, current)
		}
		for _, current := range slices.Backward(path) {
			if parent := parents[current]; parent != NoElement {
				depth[current] = depth[parent] + 1
			}
			state[current] = 2
		}
	}
	total := 0
	for _, ancestors := range depth {
		if ancestors > int(^uint(0)>>1)-total {
			return substitutionForest{}, SubstitutionClosureLimitError{Limit: int(^uint(0) >> 1)}
		}
		total += ancestors
	}
	return substitutionForest{parents: parents, total: total}, nil
}

type substitutionEdge struct {
	mask   DerivationMask
	blocks DerivationMask
}

func validateSubstitutionEdges(
	rt TypeDerivationRuntime,
	elements []ElementDecl,
	parents []ElementID,
) ([]substitutionEdge, error) {
	edges := make([]substitutionEdge, len(elements))
	for member, head := range parents {
		if head == NoElement {
			continue
		}
		mask, ok := TypeDerivationMask(rt, elements[member].Type, elements[head].Type)
		if !ok {
			return nil, SubstitutionMembershipError{
				Cause:  ErrSubstitutionMemberTypeNotDerived,
				Member: ElementID(member),
				Head:   head,
			}
		}
		if elements[head].Final&mask != 0 {
			return nil, SubstitutionMembershipError{
				Cause:  ErrSubstitutionMemberTypeExcludedDerivation,
				Member: ElementID(member),
				Head:   head,
			}
		}
		edges[member] = substitutionEdge{
			mask:   mask,
			blocks: substitutionTypeBlocks(rt, elements[member].Type, elements[head].Type),
		}
	}
	return edges, nil
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
func (e SubstitutionCycleError) Error() string {
	return "cyclic substitution group"
}

// Error returns the stable substitution closure limit message.
func (e SubstitutionClosureLimitError) Error() string {
	return "substitution closure entry limit exceeded"
}

// Error returns the stable invalid-membership message.
func (e SubstitutionMembershipError) Error() string {
	return "substitution member is not allowed by head"
}

// Unwrap returns the precise direct-membership failure.
func (e SubstitutionMembershipError) Unwrap() error {
	return e.Cause
}

// ValidateSubstitutionMembership validates that member can substitute for head
// by type derivation and the head's final constraints.
func ValidateSubstitutionMembership(rt TypeDerivationRuntime, head, member ElementDecl) error {
	mask, ok := TypeDerivationMask(rt, member.Type, head.Type)
	if !ok {
		return ErrSubstitutionMemberTypeNotDerived
	}
	if head.Final&mask != 0 {
		return ErrSubstitutionMemberTypeExcludedDerivation
	}
	return nil
}

func substitutionEffective(head, member ElementDecl, mask, typeBlocks DerivationMask) bool {
	if member.Abstract {
		return false
	}
	if head.Block&DerivationSubstitution != 0 {
		return false
	}
	return mask&head.Block == 0 && mask&typeBlocks == 0
}

func elementIndexID(id int) (ElementID, bool) {
	if id < 0 || uint64(id) >= uint64(invalidID) {
		return NoElement, false
	}
	return ElementID(uint32(id)), true
}

func validSubstitutionElementID(elements []ElementDecl, id ElementID) bool {
	return ValidElementID(id, len(elements))
}
