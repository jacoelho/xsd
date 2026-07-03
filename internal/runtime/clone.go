package runtime

import (
	"maps"
	"slices"
)

// CloneSubstitutionMap deep-clones substitution membership metadata for frozen
// runtime publication.
func CloneSubstitutionMap(in map[ElementID][]ElementID) map[ElementID][]ElementID {
	if in == nil {
		return nil
	}
	out := make(map[ElementID][]ElementID, len(in))
	for head, members := range in {
		out[head] = slices.Clone(members)
	}
	return out
}

// CloneSubstitutionLookup deep-clones substitution name lookup metadata for
// frozen runtime publication.
func CloneSubstitutionLookup(in map[ElementID]map[QName]ElementID) map[ElementID]map[QName]ElementID {
	if in == nil {
		return nil
	}
	out := make(map[ElementID]map[QName]ElementID, len(in))
	for head, byName := range in {
		out[head] = maps.Clone(byName)
	}
	return out
}

// CloneWildcard deep-clones wildcard metadata.
func CloneWildcard(in Wildcard) Wildcard {
	in.Namespaces = slices.Clone(in.Namespaces)
	return in
}

// CloneContentModel deep-clones content-model metadata.
func CloneContentModel(in ContentModel) ContentModel {
	in.Particles = slices.Clone(in.Particles)
	in.ChoiceLimits = slices.Clone(in.ChoiceLimits)
	return in
}

// CloneSimpleTypeDerivation deep-clones simple-type derivation projection
// metadata.
func CloneSimpleTypeDerivation(in SimpleTypeDerivation) SimpleTypeDerivation {
	in.Union = slices.Clone(in.Union)
	return in
}

// CloneFacetSet deep-clones compiled facet storage.
func CloneFacetSet(in FacetSet) FacetSet {
	in.bounds = cloneFacetBounds(in.bounds)
	in.Enumeration = slices.Clone(in.Enumeration)
	in.Patterns = CloneStringPatternGroups(in.Patterns)
	return in
}

func cloneFacetBounds(in facetBounds) facetBounds {
	for i, lit := range in {
		if lit != nil {
			cloned := *lit
			in[i] = &cloned
		}
	}
	return in
}

// CloneSimpleTypeDerivations deep-clones simple-type derivation projection
// metadata.
func CloneSimpleTypeDerivations(in []SimpleTypeDerivation) []SimpleTypeDerivation {
	out := slices.Clone(in)
	for i := range out {
		out[i] = CloneSimpleTypeDerivation(in[i])
	}
	return out
}

// CloneValueConstraintSimpleType deep-clones value-constraint simple-type
// projection metadata.
func CloneValueConstraintSimpleType(in ValueConstraintSimpleType) ValueConstraintSimpleType {
	in.Union = slices.Clone(in.Union)
	return in
}

// CloneSimpleTypeValidation deep-clones simple-type validation projection
// metadata.
func CloneSimpleTypeValidation(in SimpleTypeValidation) SimpleTypeValidation {
	in.Union = slices.Clone(in.Union)
	return in
}

// CloneSimpleTypeRestrictionValidation deep-clones simple-type restriction
// validation projection metadata.
func CloneSimpleTypeRestrictionValidation(in SimpleTypeRestrictionValidation) SimpleTypeRestrictionValidation {
	in.Union = slices.Clone(in.Union)
	return in
}

// CloneSimpleTypeGraphNode deep-clones simple-type graph projection metadata.
func CloneSimpleTypeGraphNode(in SimpleTypeGraphNode) SimpleTypeGraphNode {
	in.Union = slices.Clone(in.Union)
	return in
}

// CloneValueConstraintIdentity deep-clones value-constraint identity
// projection metadata.
func CloneValueConstraintIdentity(in ValueConstraintIdentity) ValueConstraintIdentity {
	in.ResolvedNames = slices.Clone(in.ResolvedNames)
	return in
}

// CloneRuntimeGlobals deep-clones runtime global declaration-map validation
// metadata.
func CloneRuntimeGlobals(in RuntimeGlobals) RuntimeGlobals {
	return RuntimeGlobals{
		GlobalAttributes: maps.Clone(in.GlobalAttributes),
		GlobalElements:   maps.Clone(in.GlobalElements),
		GlobalTypes:      maps.Clone(in.GlobalTypes),
		GlobalIdentities: maps.Clone(in.GlobalIdentities),
		Notations:        maps.Clone(in.Notations),
		AttributeNames:   slices.Clone(in.AttributeNames),
		ElementNames:     slices.Clone(in.ElementNames),
		SimpleTypeNames:  slices.Clone(in.SimpleTypeNames),
		ComplexTypeNames: slices.Clone(in.ComplexTypeNames),
		IdentityNames:    slices.Clone(in.IdentityNames),
	}
}

// CloneAttributeUseSetValidation deep-clones attribute-use-set validation
// projection metadata.
func CloneAttributeUseSetValidation(in AttributeUseSetValidation) AttributeUseSetValidation {
	return AttributeUseSetValidation{
		Index:            maps.Clone(in.Index),
		Uses:             slices.Clone(in.Uses),
		Required:         slices.Clone(in.Required),
		ValueConstraints: slices.Clone(in.ValueConstraints),
		Wildcard:         in.Wildcard,
	}
}

// CloneElementDeclValidation deep-clones element-declaration validation
// projection metadata.
func CloneElementDeclValidation(in ElementDeclValidation) ElementDeclValidation {
	in.Identity = slices.Clone(in.Identity)
	return in
}

// CloneCompiledModel deep-clones compiled content-model metadata for frozen
// runtime publication.
func CloneCompiledModel(in CompiledModel) CompiledModel {
	in.Rows = cloneCompiledModelRows(in.Rows)
	in.All = slices.Clone(in.All)
	return in
}

func cloneCompiledModelRows(in []CompiledModelRow) []CompiledModelRow {
	out := slices.Clone(in)
	for i, row := range in {
		out[i].Index = cloneDFARowIndex(row.Index)
		out[i].Edges = slices.Clone(row.Edges)
	}
	return out
}

func cloneDFARowIndex(in DFARowIndex) DFARowIndex {
	return DFARowIndex{
		NameToEdge:    maps.Clone(in.NameToEdge),
		WildcardEdges: slices.Clone(in.WildcardEdges),
		Enabled:       in.Enabled,
	}
}
