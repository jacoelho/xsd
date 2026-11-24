package schema

import (
	"errors"
	"maps"

	"github.com/jacoelho/xsd/xsderrors"
)

// PublishSchema validates compiler-owned state, constructs one immutable
// validation program, and seals the result. The caller must provide exclusive
// access to build for the duration of the call. A rejected build is left
// unchanged. On success, build is cleared and the returned schema owns all
// validation-facing storage. The schema takes the compiled content tables,
// already over published element IDs, without copying them, and copies the
// substitution table over published element IDs while sharing its name index
// with build; other previously retained aliases may be mutated without
// affecting the schema.
func publishSchema(build *schemaBuild, work contentModelWork) (*Schema, error) {
	if build == nil {
		return nil, errors.New("nil schema build")
	}
	if err := requireContentModelWork(work); err != nil {
		return nil, err
	}
	program, err := sealSchemaProgram(build, work)
	if err != nil {
		return nil, err
	}
	*build = schemaBuild{}
	return &Schema{program: program}, nil
}

func sealSchemaProgram(build *schemaBuild, work contentModelWork) (schemaProgram, error) {
	if err := validateSchemaBuildIDDomain(build); err != nil {
		return schemaProgram{}, err
	}
	if err := validateSchemaBuildOwnership(build); err != nil {
		return schemaProgram{}, err
	}
	contentAnalysis, err := newContentModelAnalysis(build, work)
	if err != nil {
		return schemaProgram{}, err
	}
	validation := schemaValidation{
		build:            build,
		contentModelWork: work,
		contentAnalysis:  contentAnalysis,
	}
	if err = validateSchema(&validation); err != nil {
		return schemaProgram{}, err
	}
	program, err := newSchemaProgram(build, work)
	if err != nil {
		return schemaProgram{}, schemaBuildValidationError(err)
	}
	if build.valueBuilder == nil {
		return schemaProgram{}, xsderrors.InternalInvariant("schema build has no value program builder", nil)
	}
	if err = validatePublishedSimpleTypes(&program, build); err != nil {
		return schemaProgram{}, err
	}
	valueProgram, err := build.valueBuilder.SealKeeping(build.ReachableSimpleTypes)
	if err != nil {
		return schemaProgram{}, xsderrors.InternalInvariant("sealing the value program", err)
	}
	if valueProgram == nil {
		return schemaProgram{}, xsderrors.InternalInvariant("value builder returned nil program", nil)
	}
	program.Value = valueProgram
	// Dense conversion rewrites rows that build still shares, so it follows
	// the last step that can reject the build.
	dense := defaultDenseIndexBudget()
	addDenseDFAIndexes(program.CompiledModels, &dense)
	return program, nil
}

func validateSchemaBuildIDDomain(build *schemaBuild) error {
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
		{name: "compiled-content-model", length: len(build.CompiledContent.Models)},
		{name: "attribute-use-set", length: len(build.AttributeUseSets)},
		{name: "wildcard-definition", length: len(build.Wildcards)},
		{name: "identity-constraint", length: len(build.Identities)},
	}
	for _, table := range tables {
		if !validRuntimeIDTableLength(table.length) {
			return xsderrors.InternalInvariant("schema "+table.name+" table exceeds runtime ID domain", nil)
		}
	}
	if err := validateCompiledContent(build); err != nil {
		return err
	}
	if !build.Substitutions.validNameOrder() {
		return xsderrors.InternalInvariant("schema substitution table has no expanded-name order", nil)
	}
	return nil
}

// validateCompiledContent proves that the compiled table names ascending
// source models, one compiled model each.
func validateCompiledContent(build *schemaBuild) error {
	content := &build.CompiledContent
	if len(content.Sources) != len(content.Models) || len(content.Runs) != len(content.Models) {
		return xsderrors.InternalInvariant("schema compiled content-model table does not match its sources", nil)
	}
	for i, id := range content.Sources {
		if !validContentModelID(id, len(build.Models)) || (i > 0 && content.Sources[i-1] >= id) {
			return xsderrors.InternalInvariant("schema compiled content-model sources are not ascending source models", nil)
		}
	}
	return nil
}

// publishedElementReads are the published holders of element IDs, built over
// published IDs (publishedElements).
type publishedElementReads struct {
	globals       map[QName]ElementID
	models        []compiledModelRead
	substitutions substitutionTable
	elements      elementReadTable
}

func newPublishedElementReads(build *schemaBuild, entered []int) (publishedElementReads, error) {
	elements, err := compiledNumbering(build)
	if err != nil {
		return publishedElementReads{}, err
	}
	if err = validateReachableHolders(build); err != nil {
		return publishedElementReads{}, err
	}
	models, err := publishedContentModels(&build.CompiledContent, entered, elements.count)
	if err != nil {
		return publishedElementReads{}, err
	}
	globals, substitutions, err := publishedElementMaps(build, elements)
	if err != nil {
		return publishedElementReads{}, err
	}
	return publishedElementReads{
		globals:       globals,
		models:        models,
		substitutions: substitutions,
		elements:      newElementReadTable(build.Elements, elements),
	}, nil
}

func newSchemaProgram(build *schemaBuild, work contentModelWork) (schemaProgram, error) {
	content, err := enterContentModels(build.ComplexTypes, &build.CompiledContent)
	if err != nil {
		return schemaProgram{}, err
	}
	elementReads, err := newPublishedElementReads(build, content.Models)
	if err != nil {
		return schemaProgram{}, err
	}
	typeDerivations, err := newTypeDerivationReadForTypes(
		build.AnyType,
		build.SimpleTypes,
		build.ComplexTypes,
	)
	if err != nil {
		return schemaProgram{}, err
	}
	for _, count := range []int{len(build.Names.locals), len(build.Identities), len(build.SimpleTypes) + build.droppedSimpleTypes.types, len(build.ComplexTypes)} {
		if workErr := spendContentModelWorkN(work, count); workErr != nil {
			return schemaProgram{}, workErr
		}
	}
	names := newNameReadView(&build.Names)
	reads := schemaProgram{
		DiagnosticNames:       newDiagnosticNames(build),
		GlobalAttributes:      maps.Clone(build.GlobalAttributes),
		GlobalElements:        elementReads.globals,
		GlobalTypes:           maps.Clone(build.GlobalTypes),
		Substitutions:         elementReads.substitutions,
		Names:                 names,
		Notations:             newNotationReadMap(&build.Names, build.Notations),
		Attributes:            newAttributeDeclReadsForDecls(build.Attributes),
		TypeDerivations:       typeDerivations,
		SimpleTypeUnavailable: append([]bool(nil), build.simpleTypeUnavailable...),
		Wildcards:             newWildcardViews(&build.Names, build.Wildcards),
		CompiledModels:        elementReads.models,
		Elements:              elementReads.elements,
		IdentityDispatch:      newIdentityDispatchRead(build.Identities, names),
	}
	if err = validatePublishedIdentityConstraints(&reads.Elements, build.Identities); err != nil {
		return schemaProgram{}, err
	}
	var sets []attributeUseSet
	reads.ComplexTypes, sets, err = newComplexTypeReads(build.ComplexTypes, content.Published, build.AttributeUseSets)
	if err != nil {
		return schemaProgram{}, err
	}
	reads.AttributeUseSets, err = newAttributeUseSetReads(&build.Names, sets, build.valueBuilder)
	if err != nil {
		return schemaProgram{}, err
	}
	return reads, nil
}
