package schema

import (
	"slices"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

// Fixture materializers assemble canonical owner records; expectations remain in callers.
func fixtureSimpleTypes(in []SimpleTypeDerivation) []SimpleType {
	out := make([]SimpleType, len(in))
	for i, typ := range in {
		out[i].ValueSpec = valuepkg.TypeSpec{Base: typ.Base, Variety: typ.Variety, Union: slices.Clone(typ.Union)}
	}
	return out
}
func fixtureComplexTypes(in []ComplexTypeDerivation) []ComplexType {
	out := make([]ComplexType, len(in))
	for i, typ := range in {
		out[i] = ComplexType{Base: typ.Base, Derivation: typ.Kind, Block: typ.Block}
	}
	return out
}
func (f derivationFixture) build() *schemaBuild {
	return &schemaBuild{SimpleTypes: fixtureSimpleTypes(f.simple), ComplexTypes: fixtureComplexTypes(f.complex), AnyType: f.anyType}
}
func fixtureElements(names []QName, restrictions []ParticleRestrictionElement) []ElementDecl {
	out := make([]ElementDecl, max(len(names), len(restrictions)))
	for i, name := range names {
		out[i].Name = name
	}
	for i, r := range restrictions {
		out[i].Type = r.Type
		out[i].Scope = r.Scope
		out[i].Nillable = r.Nillable
		out[i].Block = r.Block
		out[i].Identity = slices.Clone(r.Identities.values)
		if r.Fixed.Present {
			out[i].Fixed = &ValueConstraint{Lexical: r.Fixed.Lexical, Canonical: r.Fixed.Canonical, Value: r.Fixed.Value, ResolvedNames: slices.Clone(r.Fixed.ResolvedNames), qnameContext: r.Fixed.qnameContext}
		}
	}
	return out
}
func fixtureSubstitutions(names []ElementDecl, raw map[ElementID][]ElementID, effective map[ElementID]map[QName]ElementID) SubstitutionTable {
	count := 0
	for id := range raw {
		count = max(count, int(id)+1)
	}
	for id := range effective {
		count = max(count, int(id)+1)
	}
	table := SubstitutionTable{spans: make([]substitutionSpan, count)}
	for head := range count {
		entries := make(map[ElementID]substitutionEntry)
		for _, id := range raw[ElementID(head)] {
			var name QName
			if int(id) < len(names) {
				name = names[id].Name
			}
			entries[id] = substitutionEntry{name: name, member: id}
		}
		for name, id := range effective[ElementID(head)] {
			entries[id] = substitutionEntry{name: name, member: id, effective: true}
		}
		span := substitutionSpan{start: len(table.entries), count: len(entries)}
		ordered := make([]substitutionEntry, 0, len(entries))
		for _, entry := range entries {
			ordered = append(ordered, entry)
		}
		slices.SortFunc(ordered, func(a, b substitutionEntry) int { return compareQName(a.name, b.name) })
		table.entries = append(table.entries, ordered...)
		table.spans[head] = span
	}
	return table
}
func (f particleFixture) build() *schemaBuild {
	b := &schemaBuild{Models: cloneContentModels(f.models), Elements: fixtureElements(f.elements, nil), Wildcards: slices.Clone(f.wildcards)}
	b.Substitutions = fixtureSubstitutions(b.Elements, f.substitutions, f.substitutionLookup)
	return b
}
func (f choiceLimitRestrictionFixture) build() *schemaBuild {
	b := (&derivationFixture{simple: f.simpleDerivations, complex: f.complexDerivations, anyType: f.anyType}).build()
	b.Models = cloneContentModels(f.models)
	b.Elements = fixtureElements(f.elements, f.elementRestrictions)
	return b
}
func fixtureMapSlice[K ~uint32, V any](in map[K]V) []V {
	count := 0
	for id := range in {
		count = max(count, int(id)+1)
	}
	out := make([]V, count)
	for id, v := range in {
		out[id] = v
	}
	return out
}
func (f compiledModelFixture) build() *schemaBuild {
	b := &schemaBuild{Models: fixtureMapSlice(f.models), Wildcards: fixtureMapSlice(f.wildcards), AnyType: f.anyType}
	names := fixtureMapSlice(f.elementNames)
	rs := fixtureMapSlice(f.elementRestrictions)
	b.Elements = fixtureElements(names, rs)
	for id, typ := range f.elementTypes {
		if int(id) >= len(b.Elements) {
			b.Elements = append(b.Elements, make([]ElementDecl, int(id)+1-len(b.Elements))...)
		}
		b.Elements[id].Type = typ
		b.Elements[id].Scope = DeclarationScopeNonGlobal
	}
	b.Substitutions = fixtureSubstitutions(b.Elements, nil, f.substitutions)
	b.SimpleTypes = fixtureSimpleTypes(fixtureMapSlice(f.simpleDerivations))
	for i := range b.SimpleTypes {
		if _, ok := f.simpleDerivations[SimpleTypeID(i)]; !ok {
			b.SimpleTypes[i].Missing = true
		}
	}
	b.ComplexTypes = fixtureComplexTypes(fixtureMapSlice(f.complexDerivations))
	return b
}
func (f dfaRowIndexFixture) build() *schemaBuild {
	b := &schemaBuild{Models: fixtureMapSlice(f.contentModels), Wildcards: fixtureMapSlice(f.wildcards), Elements: fixtureElements(fixtureMapSlice(f.elementNames), nil)}
	b.Substitutions = fixtureSubstitutions(b.Elements, f.substitutionIDs, f.substitutionByName)
	return b
}
func (f complexTypeValidationFixture) build() *schemaBuild {
	b := f.derivationFixture.build()
	p := f.particleFixture.build()
	b.Models = p.Models
	b.Elements = p.Elements
	b.Wildcards = p.Wildcards
	b.Substitutions = p.Substitutions
	if len(f.simpleFinal) > len(b.SimpleTypes) {
		b.SimpleTypes = append(b.SimpleTypes, make([]SimpleType, len(f.simpleFinal)-len(b.SimpleTypes))...)
	}
	for i, final := range f.simpleFinal {
		b.SimpleTypes[i].Final = final
	}
	return b
}
func (f attributeWildcardFixture) build() *schemaBuild {
	return &schemaBuild{Wildcards: fixtureMapSlice(f)}
}
func (f attributeUseSetFixture) build() *schemaBuild {
	b := &schemaBuild{Wildcards: fixtureMapSlice(f.wildcards), SimpleTypes: make([]SimpleType, len(fixtureMapSlice(f.identities)))}
	for id, identity := range f.identities {
		b.SimpleTypes[id].ValueSpec.Identity = identity
	}
	return b
}
func (f attributeUseRestrictionFixture) build() *schemaBuild {
	b := f.derivationFixture.build()
	b.Wildcards = fixtureMapSlice(f.attributeWildcardFixture)
	return b
}
func (f complexExtensionModelFixture) build() *schemaBuild {
	return &schemaBuild{Models: cloneContentModels(f.models)}
}
func (f substitutionMembershipFixture) build() *schemaBuild {
	return (&derivationFixture{simple: fixtureMapSlice(f.simple), complex: fixtureMapSlice(f.complex)}).build()
}
func (emptyTypeDerivationFixture) build() *schemaBuild { return &schemaBuild{} }

func (s *contentModelLoweringFixture) build() *schemaBuild { return &schemaBuild{Models: s.models} }
