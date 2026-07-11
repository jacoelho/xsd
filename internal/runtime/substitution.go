package runtime

import (
	"errors"
	"maps"
	"slices"
)

// SubstitutionCycleError reports a cyclic substitution group rooted at Element.
type SubstitutionCycleError struct {
	Element ElementID
}

var (
	// ErrSubstitutionMemberTypeNotDerived reports that a substitution member's
	// type is not derived from the substitution head's type.
	ErrSubstitutionMemberTypeNotDerived = errors.New("substitution member type is not derived from head")
	// ErrSubstitutionMemberTypeExcludedDerivation reports that the head's final
	// constraint blocks the member's derivation path.
	ErrSubstitutionMemberTypeExcludedDerivation = errors.New("substitution member type uses excluded derivation")
)

// ForEachSubstitutionMember iterates freeze-published substitution members for
// head until fn returns false.
func ForEachSubstitutionMember(reads map[ElementID][]ElementID, head ElementID, fn func(ElementID) bool) {
	for _, member := range reads[head] {
		if !fn(member) {
			return
		}
	}
}

// SubstitutionMemberByName returns the freeze-published substitution member
// for name under head.
func SubstitutionMemberByName(lookupReads map[ElementID]map[QName]ElementID, head ElementID, name QName) (ElementID, bool) {
	members := lookupReads[head]
	if members == nil {
		return NoElement, false
	}
	member, ok := members[name]
	if !ok {
		return NoElement, false
	}
	return member, ok
}

// Error returns the stable substitution-cycle message.
func (e SubstitutionCycleError) Error() string {
	return "cyclic substitution group"
}

// ValidateSubstitutionMaps validates frozen substitution closure and lookup maps.
func ValidateSubstitutionMaps(
	rt TypeDerivationRuntime,
	names *NameTable,
	elements []ElementDecl,
	globalElements map[QName]ElementID,
	substitutions map[ElementID][]ElementID,
	lookup map[ElementID]map[QName]ElementID,
) error {
	if names == nil {
		return errors.New("substitution maps require name table")
	}
	if !hasSubstitutionHeads(elements) {
		if len(substitutions) != 0 || len(lookup) != 0 {
			return errors.New("substitution maps exist without substitution heads")
		}
		return nil
	}
	expected, err := expectedSubstitutions(rt, elements, globalElements)
	if err != nil {
		return err
	}
	if !EqualSubstitutionMap(substitutions, expected) {
		return errors.New("substitution map does not match element substitution heads")
	}
	for head, members := range substitutions {
		if !validSubstitutionElementID(elements, head) {
			return errors.New("substitution head references invalid element")
		}
		for _, member := range members {
			if !validSubstitutionElementID(elements, member) {
				return errors.New("substitution member references invalid element")
			}
		}
	}
	for head, members := range lookup {
		if !validSubstitutionElementID(elements, head) {
			return errors.New("substitution lookup head references invalid element")
		}
		for name, member := range members {
			if !names.ValidQName(name) || !validSubstitutionElementID(elements, member) {
				return errors.New("substitution lookup references invalid element")
			}
			if elements[member].Name != name {
				return errors.New("substitution lookup name does not match element")
			}
		}
	}
	expectedLookup := BuildSubstitutionLookup(rt, elements, expected)
	if !EqualSubstitutionLookup(lookup, expectedLookup) {
		return errors.New("substitution lookup does not match substitutions")
	}
	return nil
}

func hasSubstitutionHeads(elements []ElementDecl) bool {
	for _, decl := range elements {
		if decl.SubstHead != NoElement {
			return true
		}
	}
	return false
}

func expectedSubstitutions(
	rt TypeDerivationRuntime,
	elements []ElementDecl,
	globalElements map[QName]ElementID,
) (map[ElementID][]ElementID, error) {
	direct := make(map[ElementID][]ElementID)
	for id, decl := range elements {
		if decl.SubstHead == NoElement {
			continue
		}
		member, ok := elementIndexID(id)
		if !ok {
			return nil, errors.New("substitution member element ID is invalid")
		}
		if decl.SubstHead == member {
			return nil, errors.New("element substitution head references itself")
		}
		if globalElements[decl.Name] != member {
			return nil, errors.New("substitution member is not a global element")
		}
		if !validSubstitutionElementID(elements, decl.SubstHead) {
			return nil, errors.New("element declaration references invalid substitution head")
		}
		head := elements[decl.SubstHead]
		if globalElements[head.Name] != decl.SubstHead {
			return nil, errors.New("substitution head is not a global element")
		}
		direct[decl.SubstHead] = append(direct[decl.SubstHead], member)
	}
	if err := checkSubstitutionMembership(rt, elements, direct); err != nil {
		return nil, err
	}
	return BuildSubstitutionClosure(direct)
}

func checkSubstitutionMembership(
	rt TypeDerivationRuntime,
	elements []ElementDecl,
	direct map[ElementID][]ElementID,
) error {
	for id := range direct {
		for _, member := range direct[id] {
			if err := ValidateSubstitutionMembership(rt, elements[id], elements[member]); err != nil {
				return errors.New("substitution member is not allowed by head")
			}
		}
	}
	return nil
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

// BuildSubstitutionClosure builds transitive substitution members for each
// direct substitution head.
func BuildSubstitutionClosure(direct map[ElementID][]ElementID) (map[ElementID][]ElementID, error) {
	out := make(map[ElementID][]ElementID, len(direct))
	heads := make([]ElementID, 0, len(direct))
	for head := range direct {
		heads = append(heads, head)
	}
	slices.Sort(heads)
	for _, head := range heads {
		visiting := make(map[ElementID]bool)
		seen := make(map[ElementID]bool)
		var members []ElementID
		var walk func(ElementID) error
		walk = func(id ElementID) error {
			if visiting[id] {
				return SubstitutionCycleError{Element: id}
			}
			visiting[id] = true
			for _, member := range direct[id] {
				if !seen[member] {
					seen[member] = true
					members = append(members, member)
				}
				if err := walk(member); err != nil {
					return err
				}
			}
			delete(visiting, id)
			return nil
		}
		if err := walk(head); err != nil {
			return nil, err
		}
		slices.Sort(members)
		out[head] = members
	}
	return out, nil
}

// BuildSubstitutionLookup builds the name-indexed substitution lookup map from
// a substitution closure.
func BuildSubstitutionLookup(
	rt TypeDerivationRuntime,
	elements []ElementDecl,
	substitutions map[ElementID][]ElementID,
) map[ElementID]map[QName]ElementID {
	out := make(map[ElementID]map[QName]ElementID, len(substitutions))
	for head, members := range substitutions {
		for _, member := range members {
			if !substitutionAllowed(rt, elements[head], elements[member]) {
				continue
			}
			byName := out[head]
			if byName == nil {
				byName = make(map[QName]ElementID, len(members))
				out[head] = byName
			}
			byName[elements[member].Name] = member
		}
	}
	return out
}

func substitutionAllowed(rt TypeDerivationRuntime, head, member ElementDecl) bool {
	if head.Block&DerivationSubstitution != 0 {
		return false
	}
	return SubstitutionDerivationAllowed(rt, member.Type, head.Type, head.Block)
}

// EqualSubstitutionMap reports whether two substitution closure maps expose
// the same head-to-members projection.
func EqualSubstitutionMap(a, b map[ElementID][]ElementID) bool {
	return maps.EqualFunc(a, b, slices.Equal[[]ElementID, ElementID])
}

// EqualSubstitutionLookup reports whether two substitution lookup maps expose
// the same head/name-to-member projection.
func EqualSubstitutionLookup(a, b map[ElementID]map[QName]ElementID) bool {
	if len(a) != len(b) {
		return false
	}
	for head, byName := range a {
		if !maps.Equal(byName, b[head]) {
			return false
		}
	}
	return true
}

func elementIndexID(id int) (ElementID, bool) {
	if id < 0 || uint64(id) > uint64(invalidID) {
		return NoElement, false
	}
	return ElementID(uint32(id)), true
}

func validSubstitutionElementID(elements []ElementDecl, id ElementID) bool {
	return ValidUint32Index(uint32(id), len(elements))
}
