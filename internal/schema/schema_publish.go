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
// validation-facing storage. The schema takes the compiled content tables
// and the substitution entries without copying them and rewrites the element
// IDs they hold to published IDs; other previously retained aliases may be
// mutated without affecting the schema.
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
	program, rewrite, err := newSchemaProgram(build, work)
	if err != nil {
		return schemaProgram{}, schemaBuildValidationError(err)
	}
	if build.valueBuilder == nil {
		return schemaProgram{}, xsderrors.InternalInvariant("schema build has no value program builder")
	}
	valueProgram, err := build.valueBuilder.Seal()
	if err != nil {
		return schemaProgram{}, schemaValueError(Limits{}, xsderrors.CodeSchemaFacet, "simple type", err)
	}
	if valueProgram == nil {
		return schemaProgram{}, xsderrors.InternalInvariant("value builder returned nil program")
	}
	program.Value = valueProgram
	// The element rewrite and dense conversion change storage that build
	// still shares, so they follow the last step that can reject the build.
	rewrite.apply(&program)
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
			return xsderrors.InternalInvariant("schema " + table.name + " table exceeds runtime ID domain")
		}
	}
	if err := validateCompiledContent(build); err != nil {
		return err
	}
	if !build.Substitutions.validNameOrder() {
		return xsderrors.InternalInvariant("schema substitution table has no expanded-name order")
	}
	return nil
}

// validateCompiledContent proves that the compiled table names ascending
// source models, one compiled model each.
func validateCompiledContent(build *schemaBuild) error {
	content := &build.CompiledContent
	if len(content.Sources) != len(content.Models) || len(content.Runs) != len(content.Models) {
		return xsderrors.InternalInvariant("schema compiled content-model table does not match its sources")
	}
	for i, id := range content.Sources {
		if !validContentModelID(id, len(build.Models)) || (i > 0 && content.Sources[i-1] >= id) {
			return xsderrors.InternalInvariant("schema compiled content-model sources are not ascending source models")
		}
	}
	return nil
}

// elementRewrite is the part of publication that changes storage build
// still shares, so it follows the last step that can reject the build: the
// element particles of the published content tables and the substitution
// members, from build to published IDs. runs is parallel to
// schemaProgram.CompiledModels. It cannot fail, because newSchemaProgram
// proved that every ID it rewrites has a published ID, and it visits each
// table and entry once.
type elementRewrite struct {
	runs     [][]compiledModelEdgeRead
	elements publishedElements
}

func publishedContentRuns(runs [][]compiledModelEdgeRead, entered []int) [][]compiledModelEdgeRead {
	published := make([][]compiledModelEdgeRead, len(entered))
	for i, root := range entered {
		published[i] = runs[root]
	}
	return published
}

func (r elementRewrite) apply(program *schemaProgram) {
	r.elements.rewriteContent(program.CompiledModels, r.runs)
	program.Substitutions.rewriteMembers(r.elements)
}

func newSchemaProgram(build *schemaBuild, work contentModelWork) (schemaProgram, elementRewrite, error) {
	content, err := enterContentModels(build.ComplexTypes, &build.CompiledContent)
	if err != nil {
		return schemaProgram{}, elementRewrite{}, err
	}
	elements, err := newPublishedElements(build, content.Models)
	if err != nil {
		return schemaProgram{}, elementRewrite{}, err
	}
	globalElements, substitutions, err := publishedElementMaps(build, elements)
	if err != nil {
		return schemaProgram{}, elementRewrite{}, err
	}
	typeDerivations, err := newTypeDerivationReadForTypes(
		build.AnyType,
		build.SimpleTypes,
		build.ComplexTypes,
	)
	if err != nil {
		return schemaProgram{}, elementRewrite{}, err
	}
	for _, count := range []int{len(build.Names.locals), len(build.Identities), len(build.SimpleTypes), len(build.ComplexTypes)} {
		if workErr := spendContentModelWorkN(work, count); workErr != nil {
			return schemaProgram{}, elementRewrite{}, workErr
		}
	}
	names := newNameReadView(&build.Names)
	reads := schemaProgram{
		DiagnosticNames:       newDiagnosticNames(build),
		GlobalAttributes:      maps.Clone(build.GlobalAttributes),
		GlobalElements:        globalElements,
		GlobalTypes:           maps.Clone(build.GlobalTypes),
		Substitutions:         substitutions,
		Names:                 names,
		Notations:             newNotationReadMap(&build.Names, build.Notations),
		Attributes:            newAttributeDeclReadsForDecls(build.Attributes),
		TypeDerivations:       typeDerivations,
		SimpleTypeUnavailable: append([]bool(nil), build.simpleTypeUnavailable...),
		Wildcards:             newWildcardViews(&build.Names, build.Wildcards),
		CompiledModels:        publishedContentModels(build.CompiledContent.Models, content.Models),
		Elements:              newElementReadTable(build.Elements, elements),
		IdentityDispatch:      newIdentityDispatchRead(build.Identities, names),
	}
	if err = validatePublishedIdentityConstraints(&reads.Elements, build.Identities); err != nil {
		return schemaProgram{}, elementRewrite{}, err
	}
	var sets []attributeUseSet
	reads.ComplexTypes, sets, err = newComplexTypeReads(build.ComplexTypes, content.Published, build.AttributeUseSets)
	if err != nil {
		return schemaProgram{}, elementRewrite{}, err
	}
	reads.AttributeUseSets, err = newAttributeUseSetReads(&build.Names, sets, build.valueBuilder)
	if err != nil {
		return schemaProgram{}, elementRewrite{}, err
	}
	return reads, elementRewrite{runs: publishedContentRuns(build.CompiledContent.Runs, content.Models), elements: elements}, nil
}
