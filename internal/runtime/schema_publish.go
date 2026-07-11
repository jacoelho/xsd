package runtime

import (
	"errors"

	"github.com/jacoelho/xsd/xsderrors"
)

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
	if err := validateStringPatternSourcesForSimpleTypes(build.SimpleTypes); err != nil {
		return nil, xsderrors.InternalInvariant(err.Error())
	}
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
	simpleValueRoutes := newSimpleValueRouteReadsForSimpleTypes(build.SimpleTypes)
	simpleValueCold := newSimpleValueColdReadTable(build.SimpleTypes)
	reads := schemaRuntime{
		GlobalAttributes:        build.GlobalAttributes,
		GlobalElements:          build.GlobalElements,
		GlobalTypes:             build.GlobalTypes,
		SubstitutionLookup:      build.SubstitutionLookup,
		Names:                   NewBorrowedNameReadView(&build.Names),
		Notations:               NewNotationReadMap(&build.Names, build.Notations),
		Attributes:              NewAttributeDeclReadsForDecls(build.Attributes),
		TypeDerivations:         NewBorrowedTypeDerivationReadForTypes(build.Builtin.AnyType, build.SimpleTypes, build.ComplexTypes),
		SimpleValueRoutes:       simpleValueRoutes,
		SimpleValueCold:         simpleValueCold,
		ComplexTypes:            newComplexTypeReads(build.ComplexTypes),
		Wildcards:               NewWildcardViews(&build.Names, build.Wildcards),
		CompiledModels:          newCompiledModelReads(build.CompiledModels),
		ElementNames:            NewElementNameReadsForDecls(build.Elements),
		ElementStarts:           NewElementStartInfosForElementDecls(build.Elements),
		ElementIdentities:       moveElementIdentityConstraintReads(build.Elements),
		Identities:              moveIdentityConstraintReads(build.Identities),
		ElementValueConstraints: NewElementValueConstraintReadsForDecls(build.Elements),
	}
	reads.SimpleValueQNameNeeds = newSimpleValueQNameResolverNeedsForSimpleTypes(build.SimpleTypes)
	reads.AttributeUseSets = moveAttributeUseSetReads(&build.Names, build.AttributeUseSets, build.SimpleTypes)
	return reads
}
