package runtime

import "testing"

func TestRuntimePublicationCloneHelpersDoNotAliasMutableState(t *testing.T) {
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

	valueConstraintIdentity := ValueConstraintIdentity{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item"}},
	}
	clonedValueConstraintIdentity := CloneValueConstraintIdentity(valueConstraintIdentity)
	valueConstraintIdentity.ResolvedNames[0].Lexical = "p:other"
	if clonedValueConstraintIdentity.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("CloneValueConstraintIdentity aliased resolved-name slice: %#v", clonedValueConstraintIdentity.ResolvedNames)
	}
}
