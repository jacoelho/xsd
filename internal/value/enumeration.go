package value

// enumerationIndexMinLiterals is the smallest group that gets a canonical-text
// index; smaller groups are scanned.
const enumerationIndexMinLiterals = 8

// enumerationGroup is one derivation step's enumeration literals in declaration
// order, which diagnostics list. A value satisfies the group when it equals any
// literal; each group of a derivation chain is checked independently.
//
// When the group has at least enumerationIndexMinLiterals literals and each is
// a non-list value of the one string-family primitive indexKind (string and its
// derived types after whitespace normalization, or anyURI), index maps each
// literal's canonical text to its first position. Equality for those
// primitives is canonical-text equality, so a lookup leaves at most one
// candidate and equalParsed still decides. Other groups have a nil index.
type enumerationGroup struct {
	index     map[string]uint32
	literals  []parsedValue
	indexKind PrimitiveKind
}

func newEnumerationGroup(literals []parsedValue) enumerationGroup {
	group := enumerationGroup{literals: literals}
	if len(literals) < enumerationIndexMinLiterals {
		return group
	}
	kind := literals[0].atom.kind
	if kind != PrimitiveString && kind != PrimitiveAnyURI {
		return group
	}
	for i := range literals {
		if literals[i].isList || literals[i].atom.kind != kind {
			return group
		}
	}
	index := make(map[string]uint32, len(literals))
	for i := range literals {
		key := literals[i].atom.lexical.Canonical
		if _, seen := index[key]; !seen {
			index[key] = uint32(i)
		}
	}
	group.index = index
	group.indexKind = kind
	return group
}

// matches reports whether value equals a literal of the group.
func (g *enumerationGroup) matches(value *parsedValue) bool {
	if g.index != nil {
		if value.isList || value.atom.kind != g.indexKind {
			return false
		}
		i, ok := g.index[value.atom.lexical.Canonical]
		return ok && equalParsed(value, &g.literals[i])
	}
	for i := range g.literals {
		if equalParsed(value, &g.literals[i]) {
			return true
		}
	}
	return false
}

// matchesBytes reports whether lexical is the canonical text of a literal of a
// group admitted by the raw string-enumeration plan, whose admission rule makes
// that text each literal's equality key.
func (g *enumerationGroup) matchesBytes(lexical []byte) bool {
	if g.index != nil {
		_, ok := g.index[string(lexical)]
		return ok
	}
	for i := range g.literals {
		if string(lexical) == g.literals[i].atom.lexical.Canonical {
			return true
		}
	}
	return false
}
