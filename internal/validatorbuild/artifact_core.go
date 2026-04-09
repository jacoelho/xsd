package validatorbuild

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ValidatorArtifacts contains all runtime validator artifacts generated from a schema.
type ValidatorArtifacts struct {
	elements        defaultFixedSet[analysis.ElemID]
	attributes      defaultFixedSet[analysis.AttrID]
	attrUses        defaultFixedSet[*model.AttributeDecl]
	TypeValidators  map[analysis.TypeID]runtime.ValidatorID
	ValidatorByType map[model.Type]runtime.ValidatorID
	ComplexTypes    *complexplan.ComplexTypes
	Validators      runtime.ValidatorsBundle
	Enums           runtime.EnumTable
	Facets          []runtime.FacetInstr
	Patterns        []runtime.Pattern
	Values          runtime.ValueBlob
}

// ValidatorForType returns the validator ID for a type when available.
func (c *ValidatorArtifacts) ValidatorForType(typ model.Type) (runtime.ValidatorID, bool) {
	if c == nil || typ == nil {
		return 0, false
	}
	if st, ok := model.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := model.GetBuiltin(model.TypeName(st.Name().Local)); builtin != nil {
			typ = builtin
		}
	}
	if bt, ok := model.AsBuiltinType(typ); ok {
		typ = bt
	}
	id, ok := c.ValidatorByType[typ]
	return id, ok
}

type artifactCompiler struct {
	facetsCache     map[*model.SimpleType][]model.Facet
	builtinTypeIDs  map[model.TypeName]runtime.TypeID
	complexTypes    *complexplan.ComplexTypes
	res             *typeResolver
	runtimeTypeIDs  map[analysis.TypeID]runtime.TypeID
	registry        *analysis.Registry
	schema          *parser.Schema
	compiling       map[model.Type]bool
	validatorByType map[model.Type]runtime.ValidatorID
	elements        defaultFixedSet[analysis.ElemID]
	attributes      defaultFixedSet[analysis.AttrID]
	attrUses        defaultFixedSet[*model.AttributeDecl]
	bundle          runtime.ValidatorsBundle
	enums           enumBuilder
	values          valueBuilder
	patterns        []runtime.Pattern
	facets          []runtime.FacetInstr
}

func newArtifactCompiler(sch *parser.Schema) *artifactCompiler {
	return &artifactCompiler{
		schema:          sch,
		res:             newTypeResolver(sch),
		validatorByType: make(map[model.Type]runtime.ValidatorID),
		compiling:       make(map[model.Type]bool),
		facetsCache:     make(map[*model.SimpleType][]model.Facet),
		elements: newDefaultFixedSet(
			newDefaultFixedTable[analysis.ElemID](),
			newDefaultFixedTable[analysis.ElemID](),
		),
		attributes: newDefaultFixedSet(
			newDefaultFixedTable[analysis.AttrID](),
			newDefaultFixedTable[analysis.AttrID](),
		),
		attrUses: newDefaultFixedSet(
			newDefaultFixedTable[*model.AttributeDecl](),
			newDefaultFixedTable[*model.AttributeDecl](),
		),
		bundle: runtime.ValidatorsBundle{
			Meta: make([]runtime.ValidatorMeta, 1),
		},
	}
}
