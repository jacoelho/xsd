package validatorgen

import (
	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
)

type compiledValidators struct {
	elements           defaultFixedSet[schema.ElemID]
	attributes         defaultFixedSet[schema.AttrID]
	attrUses           defaultFixedSet[*model.AttributeDecl]
	TypeValidators     map[schema.TypeID]runtime.ValidatorID
	ValidatorByType    map[model.Type]runtime.ValidatorID
	SimpleContentTypes map[*model.ComplexType]model.Type
	ComplexTypes       *complextypeplan.Plan
	Validators         runtime.ValidatorsBundle
	Enums              runtime.EnumTable
	Facets             []runtime.FacetInstr
	Patterns           []runtime.Pattern
	Values             runtime.ValueBlob
}

// ValidatorForType returns the validator ID for a type when available.
func (c *compiledValidators) ValidatorForType(typ model.Type) (runtime.ValidatorID, bool) {
	if c == nil || typ == nil {
		return 0, false
	}
	if st, ok := model.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := builtins.Get(builtins.TypeName(st.Name().Local)); builtin != nil {
			typ = builtin
		}
	}
	if bt, ok := model.AsBuiltinType(typ); ok {
		typ = bt
	}
	id, ok := c.ValidatorByType[typ]
	return id, ok
}

type compiler struct {
	facetsCache     map[*model.SimpleType][]model.Facet
	builtinTypeIDs  map[model.TypeName]runtime.TypeID
	complexTypes    *complextypeplan.Plan
	simpleContent   map[*model.ComplexType]model.Type
	res             *typeResolver
	runtimeTypeIDs  map[schema.TypeID]runtime.TypeID
	registry        *schema.Registry
	schema          *parser.Schema
	compiling       map[model.Type]bool
	validatorByType map[model.Type]runtime.ValidatorID
	elements        defaultFixedSet[schema.ElemID]
	attributes      defaultFixedSet[schema.AttrID]
	attrUses        defaultFixedSet[*model.AttributeDecl]
	bundle          runtime.ValidatorsBundle
	enums           enumBuilder
	values          valueBuilder
	patterns        []runtime.Pattern
	facets          []runtime.FacetInstr
}

func newCompiler(sch *parser.Schema) *compiler {
	return &compiler{
		schema:          sch,
		res:             newTypeResolver(sch),
		validatorByType: make(map[model.Type]runtime.ValidatorID),
		compiling:       make(map[model.Type]bool),
		facetsCache:     make(map[*model.SimpleType][]model.Facet),
		elements: newDefaultFixedSet(
			newDefaultFixedTable[schema.ElemID](),
			newDefaultFixedTable[schema.ElemID](),
		),
		attributes: newDefaultFixedSet(
			newDefaultFixedTable[schema.AttrID](),
			newDefaultFixedTable[schema.AttrID](),
		),
		attrUses: newDefaultFixedSet(
			newDefaultFixedTable[*model.AttributeDecl](),
			newDefaultFixedTable[*model.AttributeDecl](),
		),
		simpleContent: make(map[*model.ComplexType]model.Type),
		bundle: runtime.ValidatorsBundle{
			Meta: make([]runtime.ValidatorMeta, 1),
		},
	}
}
