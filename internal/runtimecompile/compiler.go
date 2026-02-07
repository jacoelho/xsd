package runtimecompile

import (
	"fmt"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
	"maps"
)

type CompiledValidators struct {
	AttributeDefaults       map[schema.AttrID]runtime.ValueRef
	AttributeDefaultKeys    map[schema.AttrID]runtime.ValueKeyRef
	TypeValidators          map[schema.TypeID]runtime.ValidatorID
	ValidatorByType         map[types.Type]runtime.ValidatorID
	SimpleContentTypes      map[*types.ComplexType]types.Type
	ElementDefaults         map[schema.ElemID]runtime.ValueRef
	ElementDefaultKeys      map[schema.ElemID]runtime.ValueKeyRef
	ElementFixed            map[schema.ElemID]runtime.ValueRef
	ElementFixedKeys        map[schema.ElemID]runtime.ValueKeyRef
	AttributeFixed          map[schema.AttrID]runtime.ValueRef
	AttributeFixedKeys      map[schema.AttrID]runtime.ValueKeyRef
	AttrUseDefaults         map[*types.AttributeDecl]runtime.ValueRef
	AttrUseDefaultKeys      map[*types.AttributeDecl]runtime.ValueKeyRef
	AttrUseFixed            map[*types.AttributeDecl]runtime.ValueRef
	AttrUseFixedKeys        map[*types.AttributeDecl]runtime.ValueKeyRef
	ElementDefaultMembers   map[schema.ElemID]runtime.ValidatorID
	ElementFixedMembers     map[schema.ElemID]runtime.ValidatorID
	AttributeDefaultMembers map[schema.AttrID]runtime.ValidatorID
	AttributeFixedMembers   map[schema.AttrID]runtime.ValidatorID
	AttrUseDefaultMembers   map[*types.AttributeDecl]runtime.ValidatorID
	AttrUseFixedMembers     map[*types.AttributeDecl]runtime.ValidatorID
	Validators              runtime.ValidatorsBundle
	Enums                   runtime.EnumTable
	Facets                  []runtime.FacetInstr
	Patterns                []runtime.Pattern
	Values                  runtime.ValueBlob
}

func CompileValidators(sch *parser.Schema, registry *schema.Registry) (*CompiledValidators, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}

	comp := newCompiler(sch)
	comp.registry = registry
	comp.initRuntimeTypeIDs(registry)
	if err := comp.compileRegistry(registry); err != nil {
		return nil, err
	}
	if err := comp.compileDefaults(registry); err != nil {
		return nil, err
	}
	if err := comp.compileAttributeUses(registry); err != nil {
		return nil, err
	}
	return comp.result(registry), nil
}

// ValidatorForType returns the validator ID for a type when available.
func (c *CompiledValidators) ValidatorForType(typ types.Type) (runtime.ValidatorID, bool) {
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

type compiler struct {
	facetsCache           map[*types.SimpleType][]types.Facet
	attrDefaultMembers    map[schema.AttrID]runtime.ValidatorID
	attrDefaultKeys       map[schema.AttrID]runtime.ValueKeyRef
	builtinTypeIDs        map[types.TypeName]runtime.TypeID
	attrUseFixedMembers   map[*types.AttributeDecl]runtime.ValidatorID
	attrDefaults          map[schema.AttrID]runtime.ValueRef
	attrUseDefaultKeys    map[*types.AttributeDecl]runtime.ValueKeyRef
	attrUseFixedKeys      map[*types.AttributeDecl]runtime.ValueKeyRef
	elemFixed             map[schema.ElemID]runtime.ValueRef
	elemFixedKeys         map[schema.ElemID]runtime.ValueKeyRef
	simpleContent         map[*types.ComplexType]types.Type
	attrUseFixed          map[*types.AttributeDecl]runtime.ValueRef
	attrUseDefaults       map[*types.AttributeDecl]runtime.ValueRef
	attrFixed             map[schema.AttrID]runtime.ValueRef
	attrFixedKeys         map[schema.AttrID]runtime.ValueKeyRef
	res                   *typeResolver
	elemDefaults          map[schema.ElemID]runtime.ValueRef
	elemDefaultKeys       map[schema.ElemID]runtime.ValueKeyRef
	runtimeTypeIDs        map[schema.TypeID]runtime.TypeID
	registry              *schema.Registry
	attrUseDefaultMembers map[*types.AttributeDecl]runtime.ValidatorID
	schema                *parser.Schema
	attrFixedMembers      map[schema.AttrID]runtime.ValidatorID
	compiling             map[types.Type]bool
	validatorByType       map[types.Type]runtime.ValidatorID
	elemFixedMembers      map[schema.ElemID]runtime.ValidatorID
	elemDefaultMembers    map[schema.ElemID]runtime.ValidatorID
	bundle                runtime.ValidatorsBundle
	enums                 enumBuilder
	values                valueBuilder
	patterns              []runtime.Pattern
	facets                []runtime.FacetInstr
}

func newCompiler(sch *parser.Schema) *compiler {
	return &compiler{
		schema:                sch,
		res:                   newTypeResolver(sch),
		validatorByType:       make(map[types.Type]runtime.ValidatorID),
		compiling:             make(map[types.Type]bool),
		facetsCache:           make(map[*types.SimpleType][]types.Facet),
		elemDefaults:          make(map[schema.ElemID]runtime.ValueRef),
		elemDefaultKeys:       make(map[schema.ElemID]runtime.ValueKeyRef),
		elemFixed:             make(map[schema.ElemID]runtime.ValueRef),
		elemFixedKeys:         make(map[schema.ElemID]runtime.ValueKeyRef),
		attrDefaults:          make(map[schema.AttrID]runtime.ValueRef),
		attrDefaultKeys:       make(map[schema.AttrID]runtime.ValueKeyRef),
		attrFixed:             make(map[schema.AttrID]runtime.ValueRef),
		attrFixedKeys:         make(map[schema.AttrID]runtime.ValueKeyRef),
		attrUseDefaults:       make(map[*types.AttributeDecl]runtime.ValueRef),
		attrUseDefaultKeys:    make(map[*types.AttributeDecl]runtime.ValueKeyRef),
		attrUseFixed:          make(map[*types.AttributeDecl]runtime.ValueRef),
		attrUseFixedKeys:      make(map[*types.AttributeDecl]runtime.ValueKeyRef),
		elemDefaultMembers:    make(map[schema.ElemID]runtime.ValidatorID),
		elemFixedMembers:      make(map[schema.ElemID]runtime.ValidatorID),
		attrDefaultMembers:    make(map[schema.AttrID]runtime.ValidatorID),
		attrFixedMembers:      make(map[schema.AttrID]runtime.ValidatorID),
		attrUseDefaultMembers: make(map[*types.AttributeDecl]runtime.ValidatorID),
		attrUseFixedMembers:   make(map[*types.AttributeDecl]runtime.ValidatorID),
		simpleContent:         make(map[*types.ComplexType]types.Type),
		bundle: runtime.ValidatorsBundle{
			Meta: make([]runtime.ValidatorMeta, 1),
		},
	}
}

func (c *compiler) initRuntimeTypeIDs(registry *schema.Registry) {
	if registry == nil {
		return
	}
	c.runtimeTypeIDs = make(map[schema.TypeID]runtime.TypeID, len(registry.TypeOrder))
	c.builtinTypeIDs = make(map[types.TypeName]runtime.TypeID, len(builtinTypeNames()))

	next := runtime.TypeID(1)
	for _, name := range builtinTypeNames() {
		c.builtinTypeIDs[name] = next
		next++
	}
	for _, entry := range registry.TypeOrder {
		c.runtimeTypeIDs[entry.ID] = next
		next++
	}
}

func (c *compiler) compileRegistry(registry *schema.Registry) error {
	for _, name := range builtinTypeNames() {
		if name == types.TypeNameAnyType {
			continue
		}
		bt := types.GetBuiltin(name)
		if bt == nil {
			continue
		}
		if _, err := c.compileType(bt); err != nil {
			return fmt.Errorf("builtin type %s: %w", name, err)
		}
	}
	for _, entry := range registry.TypeOrder {
		st, ok := types.AsSimpleType(entry.Type)
		if !ok {
			continue
		}
		if types.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("type %s: unresolved placeholder", entry.QName)
		}
		_, err := c.compileType(st)
		if err != nil {
			return fmt.Errorf("type %s: %w", entry.QName, err)
		}
	}
	for _, entry := range registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok {
			continue
		}
		if _, ok := ct.Content().(*types.SimpleContent); !ok {
			continue
		}
		textType, err := c.simpleContentTextType(ct)
		if err != nil {
			return fmt.Errorf("type %s: %w", entry.QName, err)
		}
		if textType == nil {
			return fmt.Errorf("type %s: simpleContent base missing", entry.QName)
		}
		if _, err := c.compileType(textType); err != nil {
			return fmt.Errorf("type %s: %w", entry.QName, err)
		}
	}
	return nil
}

func (c *compiler) result(registry *schema.Registry) *CompiledValidators {
	out := &CompiledValidators{
		Validators:              c.bundle,
		Facets:                  c.facets,
		Patterns:                c.patterns,
		Enums:                   c.enums.table(),
		Values:                  c.values.table(),
		TypeValidators:          make(map[schema.TypeID]runtime.ValidatorID),
		ValidatorByType:         make(map[types.Type]runtime.ValidatorID),
		ElementDefaults:         c.elemDefaults,
		ElementDefaultKeys:      c.elemDefaultKeys,
		ElementFixed:            c.elemFixed,
		ElementFixedKeys:        c.elemFixedKeys,
		AttributeDefaults:       c.attrDefaults,
		AttributeDefaultKeys:    c.attrDefaultKeys,
		AttributeFixed:          c.attrFixed,
		AttributeFixedKeys:      c.attrFixedKeys,
		AttrUseDefaults:         c.attrUseDefaults,
		AttrUseDefaultKeys:      c.attrUseDefaultKeys,
		AttrUseFixed:            c.attrUseFixed,
		AttrUseFixedKeys:        c.attrUseFixedKeys,
		ElementDefaultMembers:   c.elemDefaultMembers,
		ElementFixedMembers:     c.elemFixedMembers,
		AttributeDefaultMembers: c.attrDefaultMembers,
		AttributeFixedMembers:   c.attrFixedMembers,
		AttrUseDefaultMembers:   c.attrUseDefaultMembers,
		AttrUseFixedMembers:     c.attrUseFixedMembers,
	}
	if len(c.simpleContent) > 0 {
		out.SimpleContentTypes = make(map[*types.ComplexType]types.Type, len(c.simpleContent))
		maps.Copy(out.SimpleContentTypes, c.simpleContent)
	}
	maps.Copy(out.ValidatorByType, c.validatorByType)
	for _, entry := range registry.TypeOrder {
		st, ok := types.AsSimpleType(entry.Type)
		if !ok {
			continue
		}
		key := c.canonicalTypeKey(st)
		if id, ok := c.validatorByType[key]; ok {
			out.TypeValidators[entry.ID] = id
		}
	}
	return out
}
