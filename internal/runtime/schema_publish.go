package runtime

import "errors"

// PublishSchema audits compiler-owned state, constructs every validation read
// projection, verifies those projections, and seals the result. The build is
// moved on success; callers must not retain or mutate its maps or slices.
func PublishSchema(build *SchemaBuild) (*Schema, error) {
	if build == nil {
		return nil, errors.New("nil schema build")
	}
	candidate, err := newAuditedSchema(build)
	if err != nil {
		return nil, err
	}
	*build = SchemaBuild{}
	return candidate, nil
}

func newAuditedSchema(build *SchemaBuild) (*Schema, error) {
	candidate := &Schema{runtime: newSchemaRuntime(build)}
	audit := schemaAudit{Schema: *candidate, build: *build}
	if err := validateSchema(&audit); err != nil {
		return nil, err
	}
	if err := validateRuntimeReadProjections(&audit); err != nil {
		return nil, err
	}
	return candidate, nil
}

func newSchemaRuntime(build *SchemaBuild) schemaRuntime {
	globalReads := NewGlobalReadMapProjection(build.GlobalAttributes, build.GlobalElements, build.GlobalTypes)
	simpleValueRoutes := newSimpleValueRouteReadsForSimpleTypes(build.SimpleTypes)
	simpleValueCold := newSimpleValueColdReadTable(build.SimpleTypes)
	reads := schemaRuntime{
		Builtin:                 build.Builtin,
		GlobalAttributes:        globalReads.Attributes,
		GlobalElements:          globalReads.Elements,
		GlobalTypes:             globalReads.Types,
		Substitutions:           build.Substitutions,
		SubstitutionLookup:      build.SubstitutionLookup,
		Names:                   NewBorrowedNameReadView(&build.Names),
		Notations:               NewNotationReadMap(&build.Names, build.Notations),
		Attributes:              NewAttributeDeclReadsForDecls(build.Attributes),
		TypeDerivations:         NewBorrowedTypeDerivationReadForTypes(build.Builtin.AnyType, build.SimpleTypes, build.ComplexTypes),
		SimpleTypePrimitives:    NewSimpleTypePrimitiveReadsForTypes(build.SimpleTypes),
		SimpleTypeIdentities:    NewSimpleTypeIdentityReadsForTypes(build.SimpleTypes),
		SimpleTypeFinals:        NewSimpleTypeFinalReadsForTypes(build.SimpleTypes),
		SimpleValueRoutes:       simpleValueRoutes,
		SimpleValueCold:         simpleValueCold,
		ComplexTypes:            newComplexTypeReads(build.ComplexTypes),
		Wildcards:               NewWildcardViews(&build.Names, build.Wildcards),
		CompiledModels:          NewBorrowedCompiledModelViews(build.CompiledModels),
		ElementNames:            NewElementNameReadsForDecls(build.Elements),
		ElementStarts:           NewElementStartInfosForElementDecls(build.Elements),
		ElementIdentities:       moveElementIdentityConstraintReads(build.Elements),
		Identities:              moveIdentityConstraintReads(build.Identities),
		ElementValueConstraints: NewElementValueConstraintReadsForDecls(build.Elements),
		SimpleTextContent:       NewElementTextContentForSimpleType(),
	}
	reads.SimpleValueQNameNeeds = NewSimpleValueQNameResolverNeedsForSimpleTypes(build.SimpleTypes)
	reads.AttributeUseSets = moveAttributeUseSetReads(&build.Names, build.AttributeUseSets, build.SimpleTypes)
	return reads
}
