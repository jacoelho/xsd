package runtime

import "testing"

func TestRuntimePublicationCloneHelpersDoNotAliasMutableState(t *testing.T) {
	name := QName{Namespace: 1, Local: 2}
	wildcards := []Wildcard{{
		Namespaces: []NamespaceID{1, 2},
		Mode:       WildcardList,
		Process:    ProcessStrict,
	}}
	clonedWildcard := CloneWildcard(wildcards[0])
	wildcards[0].Namespaces[0] = 9
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
	clonedModel := CloneContentModel(models[0])
	models[0].Particles[0].Element = 9
	models[0].ChoiceLimits[0] = 9
	if clonedModel.Particles[0].Element != 1 || clonedModel.ChoiceLimits[0] != 0 {
		t.Fatalf("CloneContentModel aliased nested slices: %#v", clonedModel)
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

	valueConstraintSimpleType := ValueConstraintSimpleType{Union: []SimpleTypeID{1, 2}}
	clonedValueConstraintSimpleType := CloneValueConstraintSimpleType(valueConstraintSimpleType)
	valueConstraintSimpleType.Union[0] = 9
	if clonedValueConstraintSimpleType.Union[0] != 1 {
		t.Fatalf("CloneValueConstraintSimpleType aliased union slice: %#v", clonedValueConstraintSimpleType.Union)
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
	clonedCompiled := CloneCompiledModel(compiled[0])
	compiled[0].Rows[0].Edges[0].To = 9
	compiled[0].Rows[0].Index.NameToEdge[name] = 9
	compiled[0].Rows[0].Index.WildcardEdges[0] = 9
	compiled[0].All[0].Required = false
	if clonedCompiled.Rows[0].Edges[0].To != 1 ||
		clonedCompiled.Rows[0].Index.NameToEdge[name] != 0 ||
		clonedCompiled.Rows[0].Index.WildcardEdges[0] != 1 ||
		!clonedCompiled.All[0].Required {
		t.Fatalf("CloneCompiledModel aliased nested state: %#v", clonedCompiled)
	}
}
