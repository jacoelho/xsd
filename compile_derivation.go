package xsd

// computeTypeDerivationMask reports whether t validly derives from base and
// the derivation methods accumulated on the path, by walking the type graph.
// It is the authority for derivation: compileTypeAncestors flattens its
// results into each type's Ancestors for validation-time lookup, and
// validateTypeAncestors recomputes them at freeze to detect drift.
func (rt *runtimeSchema) computeTypeDerivationMask(t, base typeID) (derivationMask, bool) {
	if t == base {
		return 0, true
	}
	if base == complexRef(rt.Builtin.AnyType) {
		if id, ok := t.complex(); ok {
			return rt.complexAnyTypeDerivationMask(id)
		}
		return blockRestriction, true
	}
	if tID, ok := t.complex(); ok {
		if baseID, ok := base.simple(); ok {
			return rt.complexSimpleTypeDerivationMask(tID, baseID)
		}
		if baseID, ok := base.complex(); ok {
			return rt.complexTypeDerivationMask(tID, baseID)
		}
		return 0, false
	}
	if tID, ok := t.simple(); ok {
		if baseID, ok := base.simple(); ok {
			return rt.simpleTypeDerivationMask(tID, baseID, make(map[[2]simpleTypeID]bool))
		}
	}
	return 0, false
}

func (rt *runtimeSchema) complexSimpleTypeDerivationMask(t complexTypeID, base simpleTypeID) (derivationMask, bool) {
	ct, ok := rt.complexType(t)
	if !ok || !ct.simpleContent() {
		return 0, false
	}
	var mask derivationMask
	if baseSimple, isSimple := ct.Base.simple(); isSimple {
		mask, ok = rt.simpleTypeDerivationMask(baseSimple, base, make(map[[2]simpleTypeID]bool))
	} else if baseComplex, isComplex := ct.Base.complex(); isComplex {
		mask, ok = rt.complexSimpleTypeDerivationMask(baseComplex, base)
	} else {
		return 0, false
	}
	if !ok {
		return 0, false
	}
	switch ct.Derivation {
	case derivationExtension:
		mask |= blockExtension
	case derivationRestriction:
		mask |= blockRestriction
	case derivationNone:
	}
	return mask, true
}

func (rt *runtimeSchema) complexAnyTypeDerivationMask(t complexTypeID) (derivationMask, bool) {
	var mask derivationMask
	for range len(rt.ComplexTypes) {
		if t == rt.Builtin.AnyType {
			return mask, true
		}
		ct, ok := rt.complexType(t)
		if !ok {
			return 0, false
		}
		switch ct.Derivation {
		case derivationExtension:
			mask |= blockExtension
		case derivationRestriction:
			mask |= blockRestriction
		case derivationNone:
		}
		if ct.Base.Kind == typeSimple {
			return mask | blockRestriction, true
		}
		parent, ok := ct.Base.complex()
		if !ok {
			return 0, false
		}
		t = parent
	}
	return 0, false
}

func (rt *runtimeSchema) simpleTypeDerivationMask(t, base simpleTypeID, seen map[[2]simpleTypeID]bool) (derivationMask, bool) {
	if t == base {
		return 0, true
	}
	st, ok := rt.simpleType(t)
	if !ok {
		return 0, false
	}
	baseType, ok := rt.simpleType(base)
	if !ok {
		return 0, false
	}
	pair := [2]simpleTypeID{t, base}
	if seen[pair] {
		return 0, false
	}
	seen[pair] = true

	if baseType.Variety == varietyUnion {
		for _, member := range baseType.Union {
			if mask, derived := rt.simpleTypeDerivationMask(t, member, seen); derived {
				return mask | blockRestriction, true
			}
		}
	}

	if st.Base == noSimpleType || st.Base == t {
		return 0, false
	}
	mask, ok := rt.simpleTypeDerivationMask(st.Base, base, seen)
	if !ok {
		return 0, false
	}
	return mask | blockRestriction, true
}

func (rt *runtimeSchema) complexTypeDerivationMask(t, base complexTypeID) (derivationMask, bool) {
	var mask derivationMask
	for range len(rt.ComplexTypes) {
		ct, ok := rt.complexType(t)
		if !ok {
			return 0, false
		}
		parent, ok := ct.Base.complex()
		if !ok {
			return 0, false
		}
		switch ct.Derivation {
		case derivationExtension:
			mask |= blockExtension
		case derivationRestriction:
			mask |= blockRestriction
		case derivationNone:
		}
		if parent == base {
			return mask, true
		}
		t = parent
	}
	return 0, false
}

// compileTypeAncestors flattens computeTypeDerivationMask into each type's
// Ancestors slice. It must run after every type's Base, Union, and Derivation
// are final.
func (c *compiler) compileTypeAncestors() {
	unions := c.rt.unionMembership()
	for id := range c.rt.SimpleTypes {
		c.rt.SimpleTypes[id].Ancestors = c.rt.typeAncestors(simpleRef(simpleTypeID(id)), unions)
	}
	for id := range c.rt.ComplexTypes {
		c.rt.ComplexTypes[id].Ancestors = c.rt.typeAncestors(complexRef(complexTypeID(id)), unions)
	}
}

// unionMembership inverts simpleType.Union: member -> union types that list
// it, in type-index order so ancestor construction is deterministic.
func (rt *runtimeSchema) unionMembership() map[simpleTypeID][]simpleTypeID {
	m := make(map[simpleTypeID][]simpleTypeID)
	for id := range rt.SimpleTypes {
		for _, member := range rt.SimpleTypes[id].Union {
			m[member] = append(m[member], simpleTypeID(id))
		}
	}
	return m
}

// typeAncestors enumerates every base t validly derives from, with its mask.
// Candidates are t's Base chain (crossing from complex into simple content),
// xs:anyType, and the closure of unions reachable through membership of t or
// any candidate; computeTypeDerivationMask then supplies the authoritative
// answer per candidate, so this function never re-derives mask semantics.
func (rt *runtimeSchema) typeAncestors(t typeID, unions map[simpleTypeID][]simpleTypeID) []ancestorMask {
	var candidates []typeID
	seen := map[typeID]bool{t: true}
	add := func(id typeID) {
		if !seen[id] {
			seen[id] = true
			candidates = append(candidates, id)
		}
	}
	cur := t
	for range len(rt.SimpleTypes) + len(rt.ComplexTypes) {
		next, ok := rt.typeBase(cur)
		if !ok || seen[next] {
			break
		}
		add(next)
		cur = next
	}
	add(complexRef(rt.Builtin.AnyType))
	if id, ok := t.simple(); ok {
		for _, u := range unions[id] {
			add(simpleRef(u))
		}
	}
	for i := 0; i < len(candidates); i++ { //nolint:intrange // candidates grows during iteration; len must be re-evaluated.
		if id, ok := candidates[i].simple(); ok {
			for _, u := range unions[id] {
				add(simpleRef(u))
			}
		}
	}
	var out []ancestorMask
	for _, base := range candidates {
		if mask, ok := rt.computeTypeDerivationMask(t, base); ok {
			out = append(out, ancestorMask{Ancestor: base, Mask: mask})
		}
	}
	return out
}

func (rt *runtimeSchema) typeBase(t typeID) (typeID, bool) {
	if id, ok := t.complex(); ok {
		ct, valid := rt.complexType(id)
		if !valid || ct.Base.Kind == typeNone {
			return typeID{}, false
		}
		return ct.Base, true
	}
	if id, ok := t.simple(); ok {
		st, valid := rt.simpleType(id)
		if !valid || st.Base == noSimpleType || st.Base == id {
			return typeID{}, false
		}
		return simpleRef(st.Base), true
	}
	return typeID{}, false
}
