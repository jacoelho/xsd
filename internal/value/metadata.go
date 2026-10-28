package value

// TypeView is completed construction metadata for schema callers. Slices in a
// returned view are caller-owned copies; the program owns the type records.
type TypeView struct {
	Union          []TypeID
	Base           TypeID
	ListItem       TypeID
	Variety        Variety
	Primitive      PrimitiveKind
	Whitespace     WhitespaceMode
	Builtin        BuiltinKind
	HasEnumeration bool
}

// InputRequirements is the immutable admission and evaluation metadata for a
// type. It is deliberately scalar: validation can query the completed plan
// without copying facet, union, or dependency records.
type InputRequirements struct {
	NeedsQName bool
	// DocumentIdentity reports that values can project xs:ID or xs:IDREF
	// through the type, a list item, or a union member.
	DocumentIdentity bool
	// CarriesID is the single ID-ness predicate for schema value-constraint
	// and ID-use rules and for instance ID-attribute accounting (I-016).
	CarriesID bool
	// UnconstrainedString reports an atomic xs:string value space with
	// preserved whitespace, no ID/IDREF carriers, and no effective facets,
	// including inherited ones: every lexical form is valid and equals its
	// canonical value, so callers may compare raw text directly.
	UnconstrainedString bool
	// LexicalCanonical reports an atomic xs:string value space with preserved
	// whitespace and no ID/IDREF carriers: an accepted lexical form is its own
	// canonical value, so after validation callers may compare raw text with a
	// canonical constraint. Facets may still reject values.
	LexicalCanonical bool
}

// InputRequirements returns the scalar value-admission requirements for a
// sealed program type. The returned value contains no program-owned storage.
func (p *Program) InputRequirements(id TypeID) (InputRequirements, bool) {
	if p == nil || !p.sealed {
		return InputRequirements{}, false
	}
	t, ok := p.typeDef(id)
	if !ok {
		return InputRequirements{}, false
	}
	return inputRequirements(t), true
}

// InputRequirements returns the scalar requirements for an available builder
// type, including provisional metadata during resolver callbacks.
func (b *Builder) InputRequirements(id TypeID) (InputRequirements, bool) {
	if b == nil || b.program == nil || b.program.sealed {
		return InputRequirements{}, false
	}
	t, ok := b.program.typeDef(id)
	if !ok {
		return InputRequirements{}, false
	}
	return inputRequirements(t), true
}

func inputRequirements(t *typeDef) InputRequirements {
	lexicalCanonical := t.variety == Atomic &&
		t.primitive == PrimitiveString &&
		t.whitespace == WhitespacePreserve &&
		t.carriers == 0
	return InputRequirements{
		NeedsQName:          t.needsQName,
		DocumentIdentity:    t.carriers != 0,
		CarriesID:           t.carriers&carriesID != 0,
		LexicalCanonical:    lexicalCanonical,
		UnconstrainedString: lexicalCanonical && t.facets.present == 0,
	}
}

// TypeView returns metadata for an available incremental type, including
// provisional metadata during resolver callbacks.
func (b *Builder) TypeView(id TypeID) (TypeView, bool) {
	if b == nil || b.program == nil || b.program.sealed {
		return TypeView{}, false
	}
	t, ok := b.program.typeDef(id)
	if !ok {
		return TypeView{}, false
	}
	view := TypeView{
		Variety:        t.variety,
		Primitive:      t.primitive,
		Whitespace:     t.whitespace,
		Builtin:        t.builtin,
		Base:           t.base,
		ListItem:       t.listItem,
		Union:          append([]TypeID(nil), t.union...),
		HasEnumeration: len(t.facets.enumGroups) != 0,
	}
	return view, true
}

// NotationWithoutEnumeration reports whether a completed type is atomic
// NOTATION without any effective enumeration. Restriction bases may have this
// property; the schema owner decides which references are actual type uses.
func (b *Builder) NotationWithoutEnumeration(id TypeID) (bare, ok bool) {
	t, ok := b.completedTypeDef(id)
	if !ok {
		return false, false
	}
	return t.variety == Atomic && t.primitive == PrimitiveNotation && len(t.facets.enumGroups) == 0, true
}

// UnionHasFacets reports whether a completed union has effective constraints.
// Inherited facets count: member derivation cannot bypass a restriction.
func (b *Builder) UnionHasFacets(id TypeID) (constrained, ok bool) {
	t, ok := b.completedTypeDef(id)
	if !ok {
		return false, false
	}
	return t.variety == Union && t.facets.present != 0, true
}

// ConstructorUses is an immutable view of immediate list-item or union-member
// type uses. Atomic restriction bases are excluded. Its zero value is empty.
// A returned view remains valid across further construction and sealing.
type ConstructorUses struct {
	members []TypeID
	item    TypeID
	list    bool
}

// Len returns the number of immediate constructor uses.
func (u ConstructorUses) Len() int {
	if u.list {
		return 1
	}
	return len(u.members)
}

// At returns the type at index, or false when index is outside the view.
func (u ConstructorUses) At(index int) (TypeID, bool) {
	if index < 0 || index >= u.Len() {
		return NoType, false
	}
	if u.list {
		return u.item, true
	}
	return u.members[index], true
}

// ConstructorUses borrows immutable dependencies from a completed definition.
// Invalid, reserved or sealed builder queries return false rather than an empty
// successful view. Committed definitions cannot be completed again.
func (b *Builder) ConstructorUses(id TypeID) (ConstructorUses, bool) {
	t, ok := b.completedTypeDef(id)
	if !ok {
		return ConstructorUses{}, false
	}
	switch t.variety {
	case Atomic:
		return ConstructorUses{}, true
	case List:
		return ConstructorUses{item: t.listItem, list: true}, true
	case Union:
		return ConstructorUses{members: t.union}, true
	default:
		return ConstructorUses{}, false
	}
}

func (b *Builder) completedTypeDef(id TypeID) (*typeDef, bool) {
	if b == nil || b.program == nil || b.program.sealed {
		return nil, false
	}
	if id >= BuiltinTypeCount {
		i := id - BuiltinTypeCount
		if uint64(i) >= uint64(len(b.program.complete)) || b.program.complete[i] != completionCommitted {
			return nil, false
		}
	}
	return b.program.typeDef(id)
}

func (p *Program) typeNeedsQName(id TypeID, active map[TypeID]struct{}, depth int) bool {
	t, ok := p.typeDef(id)
	if !ok {
		return false
	}
	if depth >= int(p.maxDepth) {
		return false
	}
	if t.needsQNameKnown {
		return t.needsQName
	}
	if t.primitive == PrimitiveQName || t.primitive == PrimitiveNotation {
		return true
	}
	if needs, known := p.typeNeedsQNameKnownChildren(*t, depth); known {
		return needs
	}
	if active == nil {
		active = make(map[TypeID]struct{})
	}
	if _, seen := active[id]; seen {
		return false
	}
	active[id] = struct{}{}
	defer delete(active, id)
	return p.typeNeedsQNameChildren(*t, active, depth)
}

func (p *Program) typeNeedsQNameChildren(t typeDef, active map[TypeID]struct{}, depth int) bool {
	if t.variety == List {
		return p.typeNeedsQName(t.listItem, active, depth+1)
	}
	if t.variety == Union {
		for _, member := range t.union {
			if p.typeNeedsQName(member, active, depth+1) {
				return true
			}
		}
	}
	return false
}

func (p *Program) typeNeedsQNameKnownChildren(t typeDef, depth int) (needs bool, known bool) {
	if depth+1 >= int(p.maxDepth) && (t.variety == List || t.variety == Union) {
		return false, true
	}
	switch t.variety {
	case Atomic:
		return false, true
	case List:
		return p.knownQNameChild(t.listItem)
	case Union:
		return p.knownQNameUnion(t.union)
	default:
		return false, false
	}
}

func (p *Program) knownQNameChild(id TypeID) (needs bool, known bool) {
	child, ok := p.typeDef(id)
	if !ok || !child.needsQNameKnown {
		return false, false
	}
	return child.needsQName, true
}

func (p *Program) knownQNameUnion(union []TypeID) (needs bool, known bool) {
	for _, member := range union {
		childNeeds, childKnown := p.knownQNameChild(member)
		if !childKnown {
			return false, false
		}
		if childNeeds {
			return true, true
		}
	}
	return false, true
}
