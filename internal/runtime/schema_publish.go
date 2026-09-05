package runtime

import (
	"errors"
	"maps"

	"github.com/jacoelho/xsd/xsderrors"
)

// PublishSchema audits compiler-owned state, constructs every validation read
// projection, verifies those projections, and seals the result. The caller must
// provide exclusive access to build for the duration of the call. On success,
// build is cleared and the returned schema owns all validation-facing storage;
// previously retained aliases may be mutated without affecting the schema.
func PublishSchema(build *SchemaBuild, work ContentModelWork) (*Schema, error) {
	if build == nil {
		return nil, errors.New("nil schema build")
	}
	if err := requireContentModelWork(work); err != nil {
		return nil, err
	}
	candidate, err := newAuditedSchema(build, work)
	if err != nil {
		return nil, err
	}
	*build = SchemaBuild{}
	return candidate, nil
}

func newAuditedSchema(build *SchemaBuild, work ContentModelWork) (*Schema, error) {
	if err := validateSchemaBuildIDDomain(build); err != nil {
		return nil, err
	}
	if err := validateStringPatternSourcesForSimpleTypes(build.SimpleTypes); err != nil {
		return nil, xsderrors.InternalInvariant(err.Error())
	}
	if err := validateSchemaBuildOwnership(build); err != nil {
		return nil, err
	}
	runtime, err := newSchemaRuntime(build, work)
	if err != nil {
		return nil, contentModelAuditError(err)
	}
	candidate := &Schema{runtime: runtime}
	audit := schemaAudit{Schema: *candidate, build: *build, contentModelWork: work}
	audit.contentAnalysis, err = NewContentModelAnalysis(&audit.build, work)
	if err != nil {
		return nil, err
	}
	if err := validateSchema(&audit); err != nil {
		return nil, err
	}
	if err := validateRuntimeReadProjections(&audit); err != nil {
		return nil, err
	}
	return candidate, nil
}

func validateSchemaBuildIDDomain(build *SchemaBuild) error {
	tables := [...]struct {
		name   string
		length int
	}{
		{name: "namespace", length: len(build.Names.namespaces)},
		{name: "local-name", length: len(build.Names.locals)},
		{name: "simple-type", length: len(build.SimpleTypes)},
		{name: "complex-type", length: len(build.ComplexTypes)},
		{name: "element-declaration", length: len(build.Elements)},
		{name: "attribute-declaration", length: len(build.Attributes)},
		{name: "content-model", length: len(build.Models)},
		{name: "compiled-content-model", length: len(build.CompiledModels)},
		{name: "attribute-use-set", length: len(build.AttributeUseSets)},
		{name: "wildcard-definition", length: len(build.Wildcards)},
		{name: "identity-constraint", length: len(build.Identities)},
	}
	for _, table := range tables {
		if !validRuntimeIDTableLength(table.length) {
			return xsderrors.InternalInvariant("schema " + table.name + " table exceeds runtime ID domain")
		}
	}
	return nil
}

func newSchemaRuntime(build *SchemaBuild, work ContentModelWork) (schemaRuntime, error) {
	compiledModels, err := newCompiledModelReads(build.CompiledModels, work)
	if err != nil {
		return schemaRuntime{}, err
	}
	simpleValueRoutes := newSimpleValueRouteReadsForSimpleTypes(build.SimpleTypes)
	simpleTypeCold := newSimpleTypeColdReadTable(build.SimpleTypes)
	typeDerivations, err := newTypeDerivationReadForTypes(
		build.Builtin.AnyType,
		build.SimpleTypes,
		build.ComplexTypes,
		simpleTypeCold,
	)
	if err != nil {
		return schemaRuntime{}, err
	}
	reads := schemaRuntime{
		GlobalAttributes:  maps.Clone(build.GlobalAttributes),
		GlobalElements:    maps.Clone(build.GlobalElements),
		GlobalTypes:       maps.Clone(build.GlobalTypes),
		Substitutions:     build.Substitutions,
		Names:             newNameReadView(&build.Names),
		Notations:         newNotationReadMap(&build.Names, build.Notations),
		Attributes:        newAttributeDeclReadsForDecls(build.Attributes),
		TypeDerivations:   typeDerivations,
		SimpleValueRoutes: simpleValueRoutes,
		SimpleTypeCold:    simpleTypeCold,
		ComplexTypes:      newComplexTypeReads(build.ComplexTypes),
		Wildcards:         newWildcardViews(&build.Names, build.Wildcards),
		CompiledModels:    compiledModels,
		Elements:          newElementReadTable(build.Elements, build.ComplexTypes),
		Identities:        newIdentityConstraintReads(build.Identities),
	}
	reads.SimpleValueQNameNeeds = newSimpleValueQNameResolverNeedsForSimpleTypes(build.SimpleTypes)
	reads.AttributeUseSets = newAttributeUseSetReads(&build.Names, build.AttributeUseSets, build.SimpleTypes)
	return reads, nil
}
