package runtime

// PublishSchema audits compiler-owned state, constructs every validation read
// projection, verifies those projections, and seals the result. The build is
// moved on success; callers must not retain or mutate its maps or slices.
func PublishSchema(build SchemaBuild) (*Schema, error) {
	rt := &Schema{build: build}
	if err := validateCompilerPublication(rt); err != nil {
		return nil, err
	}
	rt.reads = newSchemaReads(&rt.build)
	if err := validateRuntimeReadProjections(rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func newSchemaReads(build *SchemaBuild) schemaReads {
	globalReads := NewGlobalReadMapProjection(build.GlobalAttributes, build.GlobalElements, build.GlobalTypes)
	reads := schemaReads{
		GlobalAttributes:          globalReads.Attributes,
		GlobalElements:            globalReads.Elements,
		GlobalTypes:               globalReads.Types,
		Substitutions:             build.Substitutions,
		SubstitutionLookup:        build.SubstitutionLookup,
		Names:                     NewBorrowedNameReadView(&build.Names),
		Notations:                 NewNotationReadMap(&build.Names, build.Notations),
		Attributes:                NewAttributeDeclReadsForDecls(build.Attributes),
		TypeDerivations:           NewTypeDerivationReadForTypes(build.Builtin.AnyType, build.SimpleTypes, build.ComplexTypes),
		SimpleTypePrimitives:      NewSimpleTypePrimitiveReadsForTypes(build.SimpleTypes),
		SimpleTypeIdentities:      NewSimpleTypeIdentityReadsForTypes(build.SimpleTypes),
		SimpleTypeFinals:          NewSimpleTypeFinalReadsForTypes(build.SimpleTypes),
		SimpleValueTypes:          NewSimpleValueTypeReadsForSimpleTypes(build.SimpleTypes),
		ComplexTypeInfos:          NewTypeInfosForComplexTypes(build.ComplexTypes),
		ComplexAttributeUseSetIDs: NewComplexAttributeUseSetIDProjection(build.ComplexTypes),
		ComplexContentModelIDs:    NewComplexContentModelIDProjection(build.ComplexTypes),
		ComplexSimpleContent:      NewSimpleContentTypeReadsForComplexTypes(build.ComplexTypes),
		ComplexChildContent:       NewElementChildContentsForComplexTypes(build.ComplexTypes),
		ComplexTextContent:        NewElementTextContentsForComplexTypes(build.ComplexTypes, false),
		FixedComplexTextContent:   NewElementTextContentsForComplexTypes(build.ComplexTypes, true),
		Wildcards:                 NewWildcardViews(&build.Names, build.Wildcards),
		CompiledModels:            NewBorrowedCompiledModelViews(build.CompiledModels),
		ElementNames:              NewElementNameReadsForDecls(build.Elements),
		ElementStarts:             NewElementStartInfosForElementDecls(build.Elements),
		ElementIdentities:         NewElementIdentityConstraintReadsForDecls(build.Elements),
		Identities:                NewIdentityConstraintReads(build.Identities),
		ElementValueConstraints:   NewElementValueConstraintReadsForDecls(build.Elements),
		SimpleTextContent:         NewElementTextContentForSimpleType(),
	}
	reads.SimpleValueQNameResolverNeeds = NewSimpleValueQNameResolverNeedsForTypeReads(reads.SimpleValueTypes)
	reads.simpleValueCallbacks = NewSimpleValueCallbacksForTypeReadsAndSimpleTypes(
		reads.SimpleValueTypes,
		build.SimpleTypes,
		notationReadLookup(reads.Notations),
		nil,
		nil,
	)
	reads.AttributeUseSets = NewAttributeUseSetReadsForSetsWithTypeReads(&build.Names, build.AttributeUseSets, reads.SimpleValueTypes)
	return reads
}
