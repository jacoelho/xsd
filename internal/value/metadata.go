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
	Identity       IdentityKind
	HasEnumeration bool
}

// InputRequirements is the immutable admission and evaluation metadata for a
// type. It is deliberately scalar: validation can query the completed plan
// without copying facet, union, or dependency records.
type InputRequirements struct {
	NeedsQName          bool
	Identity            IdentityKind
	UnconstrainedString bool
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
	return InputRequirements{
		NeedsQName: t.needsQName,
		Identity:   t.identity,
		UnconstrainedString: t.variety == Atomic &&
			t.primitive == PrimitiveString &&
			t.whitespace == WhitespacePreserve &&
			t.identity == IdentityNone &&
			t.facets.present == 0,
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
		Identity:       t.identity,
		Base:           t.base,
		ListItem:       t.listItem,
		Union:          append([]TypeID(nil), t.union...),
		HasEnumeration: len(t.facets.enumGroups) != 0,
	}
	return view, true
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
