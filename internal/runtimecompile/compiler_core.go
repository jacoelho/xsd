package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

type compiledValidators struct {
	elements           defaultFixedSet[schema.ElemID]
	attributes         defaultFixedSet[schema.AttrID]
	attrUses           defaultFixedSet[*types.AttributeDecl]
	TypeValidators     map[schema.TypeID]runtime.ValidatorID
	ValidatorByType    map[types.Type]runtime.ValidatorID
	SimpleContentTypes map[*types.ComplexType]types.Type
	Validators         runtime.ValidatorsBundle
	Enums              runtime.EnumTable
	Facets             []runtime.FacetInstr
	Patterns           []runtime.Pattern
	Values             runtime.ValueBlob
}

// ValidatorForType returns the validator ID for a type when available.
func (c *compiledValidators) ValidatorForType(typ types.Type) (runtime.ValidatorID, bool) {
	if c == nil || typ == nil {
		return 0, false
	}
	if st, ok := types.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
			typ = builtin
		}
	}
	if bt, ok := types.AsBuiltinType(typ); ok {
		typ = bt
	}
	id, ok := c.ValidatorByType[typ]
	return id, ok
}

func (c *compiledValidators) elementDefault(id schema.ElemID) (compiledDefaultFixed, bool) {
	if c == nil {
		return compiledDefaultFixed{}, false
	}
	return c.elements.defaultValue(id)
}

func (c *compiledValidators) elementFixed(id schema.ElemID) (compiledDefaultFixed, bool) {
	if c == nil {
		return compiledDefaultFixed{}, false
	}
	return c.elements.fixedValue(id)
}

func (c *compiledValidators) attributeDefault(id schema.AttrID) (compiledDefaultFixed, bool) {
	if c == nil {
		return compiledDefaultFixed{}, false
	}
	return c.attributes.defaultValue(id)
}

func (c *compiledValidators) attributeFixed(id schema.AttrID) (compiledDefaultFixed, bool) {
	if c == nil {
		return compiledDefaultFixed{}, false
	}
	return c.attributes.fixedValue(id)
}

func (c *compiledValidators) attrUseDefault(attr *types.AttributeDecl) (compiledDefaultFixed, bool) {
	if c == nil {
		return compiledDefaultFixed{}, false
	}
	return c.attrUses.defaultValue(attr)
}

func (c *compiledValidators) attrUseFixed(attr *types.AttributeDecl) (compiledDefaultFixed, bool) {
	if c == nil {
		return compiledDefaultFixed{}, false
	}
	return c.attrUses.fixedValue(attr)
}

type compiler struct {
	facetsCache     map[*types.SimpleType][]types.Facet
	builtinTypeIDs  map[types.TypeName]runtime.TypeID
	simpleContent   map[*types.ComplexType]types.Type
	res             *typeResolver
	runtimeTypeIDs  map[schema.TypeID]runtime.TypeID
	registry        *schema.Registry
	schema          *parser.Schema
	compiling       map[types.Type]bool
	validatorByType map[types.Type]runtime.ValidatorID
	elements        defaultFixedSet[schema.ElemID]
	attributes      defaultFixedSet[schema.AttrID]
	attrUses        defaultFixedSet[*types.AttributeDecl]
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
		validatorByType: make(map[types.Type]runtime.ValidatorID),
		compiling:       make(map[types.Type]bool),
		facetsCache:     make(map[*types.SimpleType][]types.Facet),
		elements: newDefaultFixedSet(
			newDefaultFixedTable[schema.ElemID](),
			newDefaultFixedTable[schema.ElemID](),
		),
		attributes: newDefaultFixedSet(
			newDefaultFixedTable[schema.AttrID](),
			newDefaultFixedTable[schema.AttrID](),
		),
		attrUses: newDefaultFixedSet(
			newDefaultFixedTable[*types.AttributeDecl](),
			newDefaultFixedTable[*types.AttributeDecl](),
		),
		simpleContent: make(map[*types.ComplexType]types.Type),
		bundle: runtime.ValidatorsBundle{
			Meta: make([]runtime.ValidatorMeta, 1),
		},
	}
}
