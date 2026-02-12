package validatorgen

import (
	schema "github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	"github.com/jacoelho/xsd/internal/ids"
	model "github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

// CompiledValidators defines an exported type.
type CompiledValidators struct {
	elements           defaultFixedSet[ids.ElemID]
	attributes         defaultFixedSet[ids.AttrID]
	attrUses           defaultFixedSet[*model.AttributeDecl]
	TypeValidators     map[ids.TypeID]runtime.ValidatorID
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
func (c *CompiledValidators) ValidatorForType(typ model.Type) (runtime.ValidatorID, bool) {
	if c == nil || typ == nil {
		return 0, false
	}
	if st, ok := model.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := builtins.Get(model.TypeName(st.Name().Local)); builtin != nil {
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
	runtimeTypeIDs  map[ids.TypeID]runtime.TypeID
	registry        *schema.Registry
	schema          *parser.Schema
	compiling       map[model.Type]bool
	validatorByType map[model.Type]runtime.ValidatorID
	elements        defaultFixedSet[ids.ElemID]
	attributes      defaultFixedSet[ids.AttrID]
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
			newDefaultFixedTable[ids.ElemID](),
			newDefaultFixedTable[ids.ElemID](),
		),
		attributes: newDefaultFixedSet(
			newDefaultFixedTable[ids.AttrID](),
			newDefaultFixedTable[ids.AttrID](),
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
