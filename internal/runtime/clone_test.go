package runtime

import "testing"

func TestRuntimePublicationCloneHelpersDoNotAliasMutableState(t *testing.T) {
	substitutions := map[ElementID][]ElementID{
		1: {2, 3},
	}
	clonedSubstitutions := CloneSubstitutionMap(substitutions)
	substitutions[1][0] = 9
	if clonedSubstitutions[1][0] != 2 {
		t.Fatalf("CloneSubstitutionMap aliased member slice: %#v", clonedSubstitutions[1])
	}

	name := QName{Namespace: 1, Local: 2}
	lookup := map[ElementID]map[QName]ElementID{
		1: {name: 3},
	}
	clonedLookup := CloneSubstitutionLookup(lookup)
	lookup[1][name] = 9
	if clonedLookup[1][name] != 3 {
		t.Fatalf("CloneSubstitutionLookup aliased lookup map: %#v", clonedLookup[1])
	}

	wildcards := []Wildcard{{
		Namespaces: []NamespaceID{1, 2},
		Mode:       WildcardList,
		Process:    ProcessStrict,
	}}
	clonedWildcards := CloneWildcards(wildcards)
	clonedWildcard := CloneWildcard(wildcards[0])
	wildcards[0].Namespaces[0] = 9
	if clonedWildcards[0].Namespaces[0] != 1 {
		t.Fatalf("CloneWildcards aliased namespace slice: %#v", clonedWildcards[0].Namespaces)
	}
	if clonedWildcard.Namespaces[0] != 1 {
		t.Fatalf("CloneWildcard aliased namespace slice: %#v", clonedWildcard.Namespaces)
	}

	occurs := Occurrence{Min: 1, Max: 1}
	models := []ContentModel{{
		Particles:    []Particle{ElementParticle(1, occurs)},
		ChoiceLimits: []uint32{0},
		Occurs:       occurs,
		Kind:         ModelSequence,
	}}
	clonedModels := CloneContentModels(models)
	clonedModel := CloneContentModel(models[0])
	models[0].Particles[0].Element = 9
	models[0].ChoiceLimits[0] = 9
	if clonedModels[0].Particles[0].Element != 1 || clonedModels[0].ChoiceLimits[0] != 0 {
		t.Fatalf("CloneContentModels aliased nested slices: %#v", clonedModels[0])
	}
	if clonedModel.Particles[0].Element != 1 || clonedModel.ChoiceLimits[0] != 0 {
		t.Fatalf("CloneContentModel aliased nested slices: %#v", clonedModel)
	}

	complexTypes := []ComplexType{{Name: name, Content: 1}}
	clonedComplexTypes := CloneComplexTypes(complexTypes)
	complexTypes[0].Content = 9
	if clonedComplexTypes[0].Content != 1 {
		t.Fatalf("CloneComplexTypes aliased complex-type slice: %#v", clonedComplexTypes)
	}

	simpleDerivation := SimpleTypeDerivation{Union: []SimpleTypeID{1, 2}}
	clonedSimpleDerivation := CloneSimpleTypeDerivation(simpleDerivation)
	simpleDerivation.Union[0] = 9
	if clonedSimpleDerivation.Union[0] != 1 {
		t.Fatalf("CloneSimpleTypeDerivation aliased union slice: %#v", clonedSimpleDerivation.Union)
	}

	length := uint32(1)
	minLength := uint32(2)
	minLiteral := CompiledLiteral{Canonical: "1"}
	fastPattern := CompileSimpleStringPattern("[A-Z]")
	if fastPattern == nil {
		t.Fatal("CompileSimpleStringPattern returned nil")
	}
	facets := FacetSet{
		Length:      length,
		MinLength:   minLength,
		Enumeration: []CompiledLiteral{{Canonical: "A"}},
		Patterns: []StringPatternGroup{
			{Patterns: []StringPattern{NewFastStringPattern("[A-Z]", fastPattern)}},
		},
		Present: FacetLength | FacetMinLength | FacetEnumeration | FacetPattern,
	}
	SetBoundFacet(&facets, FacetMinInclusive, minLiteral, false)
	clonedFacets := CloneFacetSet(facets)
	facets.Length = 9
	facets.MinLength = 9
	SetBoundFacet(&facets, FacetMinInclusive, CompiledLiteral{Canonical: "9"}, false)
	facets.Enumeration[0].Canonical = "B"
	facets.Patterns[0].Patterns[0].fast.atoms = nil
	clonedMinInclusive, ok := BoundFacet(clonedFacets, FacetMinInclusive)
	if clonedFacets.Length != 1 ||
		clonedFacets.MinLength != 2 ||
		!ok ||
		clonedMinInclusive.Canonical != "1" ||
		clonedFacets.Enumeration[0].Canonical != "A" ||
		!clonedFacets.Patterns[0].Patterns[0].MatchString("A") {
		t.Fatalf("CloneFacetSet aliased mutable state: %#v", clonedFacets)
	}

	simpleTypes := []SimpleType{{
		Union:  []SimpleTypeID{1},
		Facets: facets,
		Name:   name,
	}}
	clonedSimpleTypes := CloneSimpleTypes(simpleTypes)
	simpleTypes[0].Union[0] = 9
	simpleTypes[0].Facets.Length = 10
	simpleTypes[0].Facets.Enumeration[0].Canonical = "C"
	if clonedSimpleTypes[0].Union[0] != 1 ||
		clonedSimpleTypes[0].Facets.Length != 9 ||
		clonedSimpleTypes[0].Facets.Enumeration[0].Canonical != "B" {
		t.Fatalf("CloneSimpleTypes aliased mutable state: %#v", clonedSimpleTypes[0])
	}

	valueConstraintSimpleType := ValueConstraintSimpleType{Union: []SimpleTypeID{1, 2}}
	clonedValueConstraintSimpleType := CloneValueConstraintSimpleType(valueConstraintSimpleType)
	valueConstraintSimpleType.Union[0] = 9
	if clonedValueConstraintSimpleType.Union[0] != 1 {
		t.Fatalf("CloneValueConstraintSimpleType aliased union slice: %#v", clonedValueConstraintSimpleType.Union)
	}

	valueConstraint := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item"}},
		Lexical:       "p:item",
		Canonical:     "p:item",
	}
	clonedValueConstraint := CloneValueConstraint(valueConstraint)
	valueConstraint.ResolvedNames[0].Lexical = "p:other"
	if clonedValueConstraint.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("CloneValueConstraint aliased resolved-name slice: %#v", clonedValueConstraint.ResolvedNames)
	}
	if cloned := CloneValueConstraint(nil); cloned != nil {
		t.Fatalf("CloneValueConstraint(nil) = %+v, want nil", cloned)
	}

	useConstraint := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "u:item"}},
		Lexical:       "u:item",
		Canonical:     "u:item",
	}
	attributeUses := []AttributeUse{{
		Default: useConstraint,
		Fixed:   useConstraint,
		Name:    name,
		Type:    1,
	}}
	clonedAttributeUses := CloneAttributeUses(attributeUses)
	attributeUses[0].Default.ResolvedNames[0].Lexical = "u:changed"
	if clonedAttributeUses[0].Default.ResolvedNames[0].Lexical != "u:item" {
		t.Fatalf("CloneAttributeUses aliased value constraint: %#v", clonedAttributeUses[0].Default.ResolvedNames)
	}

	setConstraint := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "s:item"}},
		Lexical:       "s:item",
		Canonical:     "s:item",
	}
	attributeUseSets := []AttributeUseSet{{
		Index:            map[QName]uint32{name: 0},
		Uses:             []AttributeUse{{Default: setConstraint, Name: name, Type: 1}},
		Required:         []uint32{0},
		ValueConstraints: []uint32{0},
		Wildcard:         1,
	}}
	clonedAttributeUseSets := CloneAttributeUseSets(attributeUseSets)
	attributeUseSets[0].Index[name] = 9
	attributeUseSets[0].Uses[0].Type = 9
	attributeUseSets[0].Uses[0].Default.ResolvedNames[0].Lexical = "s:changed"
	attributeUseSets[0].Required[0] = 9
	attributeUseSets[0].ValueConstraints[0] = 9
	if clonedAttributeUseSets[0].Index[name] != 0 ||
		clonedAttributeUseSets[0].Uses[0].Type != 1 ||
		clonedAttributeUseSets[0].Uses[0].Default.ResolvedNames[0].Lexical != "s:item" ||
		clonedAttributeUseSets[0].Required[0] != 0 ||
		clonedAttributeUseSets[0].ValueConstraints[0] != 0 {
		t.Fatalf("CloneAttributeUseSets aliased mutable state: %#v", clonedAttributeUseSets[0])
	}

	attributeDecls := []AttributeDecl{{
		Default: valueConstraint,
		Fixed:   valueConstraint,
		Name:    name,
		Type:    1,
	}}
	clonedAttributeDecls := CloneAttributeDecls(attributeDecls)
	attributeDecls[0].Default.ResolvedNames[0].Lexical = "p:changed"
	if clonedAttributeDecls[0].Default.ResolvedNames[0].Lexical != "p:other" {
		t.Fatalf("CloneAttributeDecls aliased value constraint: %#v", clonedAttributeDecls[0].Default.ResolvedNames)
	}

	elementDecls := []ElementDecl{{
		Default:  valueConstraint,
		Fixed:    valueConstraint,
		Identity: []IdentityConstraintID{1},
		Name:     name,
		Type:     SimpleRef(1),
	}}
	clonedElementDecls := CloneElementDecls(elementDecls)
	elementDecls[0].Identity[0] = 9
	elementDecls[0].Default.ResolvedNames[0].Lexical = "p:again"
	if clonedElementDecls[0].Identity[0] != 1 ||
		clonedElementDecls[0].Default.ResolvedNames[0].Lexical != "p:changed" {
		t.Fatalf("CloneElementDecls aliased mutable state: %#v", clonedElementDecls[0])
	}

	simpleValidation := SimpleTypeValidation{Union: []SimpleTypeID{1, 2}}
	clonedSimpleValidation := CloneSimpleTypeValidation(simpleValidation)
	simpleValidation.Union[0] = 9
	if clonedSimpleValidation.Union[0] != 1 {
		t.Fatalf("CloneSimpleTypeValidation aliased union slice: %#v", clonedSimpleValidation.Union)
	}

	simpleRestriction := SimpleTypeRestrictionValidation{Union: []SimpleTypeID{1, 2}}
	clonedSimpleRestriction := CloneSimpleTypeRestrictionValidation(simpleRestriction)
	simpleRestriction.Union[0] = 9
	if clonedSimpleRestriction.Union[0] != 1 {
		t.Fatalf("CloneSimpleTypeRestrictionValidation aliased union slice: %#v", clonedSimpleRestriction.Union)
	}

	graphNode := SimpleTypeGraphNode{Union: []SimpleTypeID{1, 2}}
	clonedGraphNode := CloneSimpleTypeGraphNode(graphNode)
	graphNode.Union[0] = 9
	if clonedGraphNode.Union[0] != 1 {
		t.Fatalf("CloneSimpleTypeGraphNode aliased union slice: %#v", clonedGraphNode.Union)
	}

	valueConstraintIdentity := ValueConstraintIdentity{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item"}},
	}
	clonedValueConstraintIdentity := CloneValueConstraintIdentity(valueConstraintIdentity)
	valueConstraintIdentity.ResolvedNames[0].Lexical = "p:other"
	if clonedValueConstraintIdentity.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("CloneValueConstraintIdentity aliased resolved-name slice: %#v", clonedValueConstraintIdentity.ResolvedNames)
	}

	runtimeGlobals := RuntimeGlobals{
		GlobalAttributes: map[QName]AttributeID{{Local: 1}: 1},
		GlobalElements:   map[QName]ElementID{{Local: 2}: 2},
		GlobalTypes:      map[QName]TypeID{{Local: 3}: SimpleRef(3)},
		GlobalIdentities: map[QName]IdentityConstraintID{{Local: 4}: 4},
		Notations:        map[QName]bool{{Local: 5}: true},
		AttributeNames:   []QName{{Local: 1}},
		ElementNames:     []QName{{Local: 2}},
		SimpleTypeNames:  []QName{{Local: 3}},
		ComplexTypeNames: []QName{{Local: 4}},
		IdentityNames:    []QName{{Local: 5}},
	}
	clonedGlobals := CloneRuntimeGlobals(runtimeGlobals)
	runtimeGlobals.GlobalAttributes[QName{Local: 1}] = 9
	runtimeGlobals.GlobalElements[QName{Local: 2}] = 9
	runtimeGlobals.GlobalTypes[QName{Local: 3}] = ComplexRef(9)
	runtimeGlobals.GlobalIdentities[QName{Local: 4}] = 9
	runtimeGlobals.Notations[QName{Local: 5}] = false
	runtimeGlobals.AttributeNames[0].Local = 9
	runtimeGlobals.ElementNames[0].Local = 9
	runtimeGlobals.SimpleTypeNames[0].Local = 9
	runtimeGlobals.ComplexTypeNames[0].Local = 9
	runtimeGlobals.IdentityNames[0].Local = 9
	if clonedGlobals.GlobalAttributes[QName{Local: 1}] != 1 ||
		clonedGlobals.GlobalElements[QName{Local: 2}] != 2 ||
		clonedGlobals.GlobalTypes[QName{Local: 3}] != SimpleRef(3) ||
		clonedGlobals.GlobalIdentities[QName{Local: 4}] != 4 ||
		!clonedGlobals.Notations[QName{Local: 5}] ||
		clonedGlobals.AttributeNames[0].Local != 1 ||
		clonedGlobals.ElementNames[0].Local != 2 ||
		clonedGlobals.SimpleTypeNames[0].Local != 3 ||
		clonedGlobals.ComplexTypeNames[0].Local != 4 ||
		clonedGlobals.IdentityNames[0].Local != 5 {
		t.Fatalf("CloneRuntimeGlobals aliased mutable projection state: %#v", clonedGlobals)
	}

	attributeUseSet := AttributeUseSetValidation{
		Index:            map[QName]uint32{{Local: 1}: 0},
		Uses:             []AttributeUseValidation{{Name: QName{Local: 1}, Type: 1}},
		Required:         []uint32{0},
		ValueConstraints: []uint32{0},
		Wildcard:         NoAttributeWildcardState(),
	}
	clonedAttributeUseSet := CloneAttributeUseSetValidation(attributeUseSet)
	attributeUseSet.Index[QName{Local: 1}] = 9
	attributeUseSet.Uses[0].Type = 9
	attributeUseSet.Required[0] = 9
	attributeUseSet.ValueConstraints[0] = 9
	if clonedAttributeUseSet.Index[QName{Local: 1}] != 0 ||
		clonedAttributeUseSet.Uses[0].Type != 1 ||
		clonedAttributeUseSet.Required[0] != 0 ||
		clonedAttributeUseSet.ValueConstraints[0] != 0 {
		t.Fatalf("CloneAttributeUseSetValidation aliased mutable projection state: %#v", clonedAttributeUseSet)
	}

	elementDecl := ElementDeclValidation{Identity: []IdentityConstraintID{1}, Name: QName{Local: 1}}
	clonedElementDecl := CloneElementDeclValidation(elementDecl)
	elementDecl.Identity[0] = 9
	if clonedElementDecl.Identity[0] != 1 {
		t.Fatalf("CloneElementDeclValidation aliased identity slice: %#v", clonedElementDecl.Identity)
	}

	compiled := []CompiledModel{{
		Rows: []CompiledModelRow{{
			Edges: []CompiledModelEdge{{Particle: ElementParticle(1, occurs), To: 1}},
			Index: DFARowIndex{
				NameToEdge:    map[QName]uint32{name: 0},
				WildcardEdges: []uint32{1},
				Enabled:       true,
			},
			Accept: true,
		}},
		All:    []CompiledAllTerm{{Particle: ElementParticle(1, occurs), Required: true}},
		Source: 1,
		Kind:   CompiledModelDFA,
	}}
	clonedCompiled := CloneCompiledModels(compiled)
	compiled[0].Rows[0].Edges[0].To = 9
	compiled[0].Rows[0].Index.NameToEdge[name] = 9
	compiled[0].Rows[0].Index.WildcardEdges[0] = 9
	compiled[0].All[0].Required = false
	if clonedCompiled[0].Rows[0].Edges[0].To != 1 ||
		clonedCompiled[0].Rows[0].Index.NameToEdge[name] != 0 ||
		clonedCompiled[0].Rows[0].Index.WildcardEdges[0] != 1 ||
		!clonedCompiled[0].All[0].Required {
		t.Fatalf("CloneCompiledModels aliased nested state: %#v", clonedCompiled[0])
	}
}

func TestRuntimePublicationCloneHelpersPreserveNil(t *testing.T) {
	if got := CloneSubstitutionMap(nil); got != nil {
		t.Fatalf("CloneSubstitutionMap(nil) = %#v, want nil", got)
	}
	if got := CloneSubstitutionLookup(nil); got != nil {
		t.Fatalf("CloneSubstitutionLookup(nil) = %#v, want nil", got)
	}
}
