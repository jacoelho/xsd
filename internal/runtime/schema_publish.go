package runtime

import (
	"context"
	"errors"
	"maps"

	"github.com/jacoelho/xsd/xsderrors"
)

// PublishSchema audits compiler-owned state, constructs every validation read
// projection, verifies those projections, and seals the result. The caller must
// provide exclusive access to build for the duration of the call. On success,
// build is cleared and the returned schema owns all validation-facing storage;
// previously retained aliases may be mutated without affecting the schema.
func PublishSchema(ctx context.Context, build *SchemaBuild) (*Schema, error) {
	if build == nil {
		return nil, errors.New("nil schema build")
	}
	if err := publishContextError(ctx); err != nil {
		return nil, err
	}
	candidate, err := newAuditedSchema(ctx, build)
	if err != nil {
		return nil, err
	}
	// This is the publication linearization point. Once the final cancellation
	// check passes, consuming build and returning candidate is one commit.
	if err := publishContextError(ctx); err != nil {
		return nil, err
	}
	*build = SchemaBuild{}
	return candidate, nil
}

func newAuditedSchema(ctx context.Context, build *SchemaBuild) (*Schema, error) {
	if err := validateSchemaBuildIDDomain(build); err != nil {
		return nil, err
	}
	if err := publishContextError(ctx); err != nil {
		return nil, err
	}
	if err := validateStringPatternSourcesForSimpleTypes(build.SimpleTypes); err != nil {
		return nil, xsderrors.InternalInvariant(err.Error())
	}
	if err := validateSchemaBuildOwnership(build); err != nil {
		return nil, err
	}
	if err := publishContextError(ctx); err != nil {
		return nil, err
	}
	runtime, err := newSchemaRuntime(build)
	if err != nil {
		return nil, xsderrors.InternalInvariant(err.Error())
	}
	candidate := &Schema{runtime: runtime}
	audit := schemaAudit{Schema: *candidate, build: *build}
	if err := publishContextError(ctx); err != nil {
		return nil, err
	}
	if err := validateSchema(&audit); err != nil {
		return nil, err
	}
	if err := publishContextError(ctx); err != nil {
		return nil, err
	}
	if err := validateRuntimeReadProjections(&audit); err != nil {
		return nil, err
	}
	return candidate, nil
}

func publishContextError(ctx context.Context) error {
	if ctx == nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "context is nil")
	}
	if cause := context.Cause(ctx); cause != nil {
		return xsderrors.Canceled(xsderrors.CodeCompileCanceled, "schema publication canceled", cause)
	}
	return nil
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

func newSchemaRuntime(build *SchemaBuild) (schemaRuntime, error) {
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
		Names:             NewNameReadView(&build.Names),
		Notations:         NewNotationReadMap(&build.Names, build.Notations),
		Attributes:        NewAttributeDeclReadsForDecls(build.Attributes),
		TypeDerivations:   typeDerivations,
		SimpleValueRoutes: simpleValueRoutes,
		SimpleTypeCold:    simpleTypeCold,
		ComplexTypes:      newComplexTypeReads(build.ComplexTypes),
		Wildcards:         NewWildcardViews(&build.Names, build.Wildcards),
		CompiledModels:    newCompiledModelReads(build.CompiledModels),
		Elements:          newElementReadTable(build.Elements, build.ComplexTypes),
		Identities:        newIdentityConstraintReads(build.Identities),
	}
	reads.SimpleValueQNameNeeds = newSimpleValueQNameResolverNeedsForSimpleTypes(build.SimpleTypes)
	reads.AttributeUseSets = newAttributeUseSetReads(&build.Names, build.AttributeUseSets, build.SimpleTypes)
	return reads, nil
}
