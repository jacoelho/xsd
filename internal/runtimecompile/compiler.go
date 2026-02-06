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

func (c *compiler) compileType(typ types.Type) (runtime.ValidatorID, error) {
	if typ == nil {
		return 0, nil
	}
	key := c.canonicalTypeKey(typ)
	if id, ok := c.validatorByType[key]; ok {
		return id, nil
	}
	if c.compiling[key] {
		return 0, fmt.Errorf("validator cycle detected")
	}
	c.compiling[key] = true
	defer delete(c.compiling, key)

	switch t := key.(type) {
	case *types.SimpleType:
		id, err := c.compileSimpleType(t)
		if err != nil {
			return 0, err
		}
		c.validatorByType[key] = id
		return id, nil
	case *types.BuiltinType:
		id, err := c.compileBuiltin(t)
		if err != nil {
			return 0, err
		}
		c.validatorByType[key] = id
		return id, nil
	default:
		return 0, nil
	}
}

func (c *compiler) canonicalTypeKey(typ types.Type) types.Type {
	if st, ok := types.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
			return builtin
		}
	}
	return typ
}

func (c *compiler) compileBuiltin(bt *types.BuiltinType) (runtime.ValidatorID, error) {
	name := bt.Name().Local
	ws := c.res.whitespaceMode(bt)

	if itemName, ok := types.BuiltinListItemTypeName(name); ok {
		itemType := types.GetBuiltin(itemName)
		if itemType == nil {
			return 0, fmt.Errorf("builtin list %s: item type %s not found", name, itemName)
		}
		itemID, err := c.compileType(itemType)
		if err != nil {
			return 0, err
		}
		start := len(c.facets)
		c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMinLength, Arg0: 1})
		facetRef := runtime.FacetProgramRef{Off: uint32(start), Len: 1}
		return c.addListValidator(ws, facetRef, itemID), nil
	}

	kind, err := builtinValidatorKind(name)
	if err != nil {
		return 0, err
	}
	return c.addAtomicValidator(kind, ws, runtime.FacetProgramRef{}, stringKindForBuiltin(name), integerKindForBuiltin(name)), nil
}

func (c *compiler) compileSimpleType(st *types.SimpleType) (runtime.ValidatorID, error) {
	if st == nil {
		return 0, nil
	}
	if types.IsPlaceholderSimpleType(st) {
		return 0, fmt.Errorf("placeholder simpleType")
	}

	if base := c.res.baseType(st); base != nil {
		if _, err := c.compileType(base); err != nil {
			return 0, err
		}
	}

	switch c.res.variety(st) {
	case types.ListVariety:
		item, ok := c.res.listItemType(st)
		if !ok || item == nil {
			return 0, fmt.Errorf("list type missing item type")
		}
		if _, err := c.compileType(item); err != nil {
			return 0, err
		}
	case types.UnionVariety:
		members := c.res.unionMemberTypes(st)
		if len(members) == 0 {
			return 0, fmt.Errorf("union has no member types")
		}
		for _, member := range members {
			if _, err := c.compileType(member); err != nil {
				return 0, err
			}
		}
	}

	facets, err := c.collectFacets(st)
	if err != nil {
		return 0, err
	}
	partialFacets := filterFacets(facets, func(f types.Facet) bool {
		_, ok := f.(*types.Enumeration)
		return !ok
	})

	facetRef, err := c.compileFacetProgram(st, facets, partialFacets)
	if err != nil {
		return 0, err
	}

	ws := c.res.whitespaceMode(st)
	switch c.res.variety(st) {
	case types.ListVariety:
		item, _ := c.res.listItemType(st)
		itemID, err := c.compileType(item)
		if err != nil {
			return 0, err
		}
		return c.addListValidator(ws, facetRef, itemID), nil
	case types.UnionVariety:
		members := c.res.unionMemberTypes(st)
		memberIDs := make([]runtime.ValidatorID, 0, len(members))
		memberTypeIDs := make([]runtime.TypeID, 0, len(members))
		for _, member := range members {
			id, err := c.compileType(member)
			if err != nil {
				return 0, err
			}
			memberIDs = append(memberIDs, id)
			typeID, ok := c.typeIDForType(member)
			if !ok {
				return 0, fmt.Errorf("union member type id not found")
			}
			memberTypeIDs = append(memberTypeIDs, typeID)
		}
		typeID, _ := c.typeIDForType(st)
		return c.addUnionValidator(ws, facetRef, memberIDs, memberTypeIDs, st.QName.String(), typeID)
	default:
		kind, err := c.validatorKind(st)
		if err != nil {
			return 0, err
		}
		return c.addAtomicValidator(kind, ws, facetRef, c.stringKindForType(st), c.integerKindForType(st)), nil
	}
}

func (c *compiler) typeIDForType(typ types.Type) (runtime.TypeID, bool) {
	if c == nil || c.registry == nil || typ == nil {
		return 0, false
	}
	if bt, ok := types.AsBuiltinType(typ); ok && bt != nil {
		if id, ok := c.builtinTypeIDs[types.TypeName(bt.Name().Local)]; ok {
			return id, true
		}
	}
	if st, ok := types.AsSimpleType(typ); ok && st != nil {
		if st.IsBuiltin() {
			if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
				if id, ok := c.builtinTypeIDs[types.TypeName(builtin.Name().Local)]; ok {
					return id, true
				}
			}
		}
		if name := st.Name(); !name.IsZero() {
			if schemaID, ok := c.registry.Types[name]; ok {
				if id, ok := c.runtimeTypeIDs[schemaID]; ok {
					return id, true
				}
			}
		}
	}
	if schemaID, ok := c.registry.AnonymousTypes[typ]; ok {
		if id, ok := c.runtimeTypeIDs[schemaID]; ok {
			return id, true
		}
	}
	return 0, false
}

func (c *compiler) addAtomicValidator(kind runtime.ValidatorKind, ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, stringKind runtime.StringKind, intKind runtime.IntegerKind) runtime.ValidatorID {
	index := uint32(0)
	switch kind {
	case runtime.VString:
		index = uint32(len(c.bundle.String))
		if stringKind == 0 {
			stringKind = runtime.StringAny
		}
		c.bundle.String = append(c.bundle.String, runtime.StringValidator{Kind: stringKind})
	case runtime.VBoolean:
		index = uint32(len(c.bundle.Boolean))
		c.bundle.Boolean = append(c.bundle.Boolean, runtime.BooleanValidator{})
	case runtime.VDecimal:
		index = uint32(len(c.bundle.Decimal))
		c.bundle.Decimal = append(c.bundle.Decimal, runtime.DecimalValidator{})
	case runtime.VInteger:
		index = uint32(len(c.bundle.Integer))
		if intKind == 0 {
			intKind = runtime.IntegerAny
		}
		c.bundle.Integer = append(c.bundle.Integer, runtime.IntegerValidator{Kind: intKind})
	case runtime.VFloat:
		index = uint32(len(c.bundle.Float))
		c.bundle.Float = append(c.bundle.Float, runtime.FloatValidator{})
	case runtime.VDouble:
		index = uint32(len(c.bundle.Double))
		c.bundle.Double = append(c.bundle.Double, runtime.DoubleValidator{})
	case runtime.VDuration:
		index = uint32(len(c.bundle.Duration))
		c.bundle.Duration = append(c.bundle.Duration, runtime.DurationValidator{})
	case runtime.VDateTime:
		index = uint32(len(c.bundle.DateTime))
		c.bundle.DateTime = append(c.bundle.DateTime, runtime.DateTimeValidator{})
	case runtime.VTime:
		index = uint32(len(c.bundle.Time))
		c.bundle.Time = append(c.bundle.Time, runtime.TimeValidator{})
	case runtime.VDate:
		index = uint32(len(c.bundle.Date))
		c.bundle.Date = append(c.bundle.Date, runtime.DateValidator{})
	case runtime.VGYearMonth:
		index = uint32(len(c.bundle.GYearMonth))
		c.bundle.GYearMonth = append(c.bundle.GYearMonth, runtime.GYearMonthValidator{})
	case runtime.VGYear:
		index = uint32(len(c.bundle.GYear))
		c.bundle.GYear = append(c.bundle.GYear, runtime.GYearValidator{})
	case runtime.VGMonthDay:
		index = uint32(len(c.bundle.GMonthDay))
		c.bundle.GMonthDay = append(c.bundle.GMonthDay, runtime.GMonthDayValidator{})
	case runtime.VGDay:
		index = uint32(len(c.bundle.GDay))
		c.bundle.GDay = append(c.bundle.GDay, runtime.GDayValidator{})
	case runtime.VGMonth:
		index = uint32(len(c.bundle.GMonth))
		c.bundle.GMonth = append(c.bundle.GMonth, runtime.GMonthValidator{})
	case runtime.VAnyURI:
		index = uint32(len(c.bundle.AnyURI))
		c.bundle.AnyURI = append(c.bundle.AnyURI, runtime.AnyURIValidator{})
	case runtime.VQName:
		index = uint32(len(c.bundle.QName))
		c.bundle.QName = append(c.bundle.QName, runtime.QNameValidator{})
	case runtime.VNotation:
		index = uint32(len(c.bundle.Notation))
		c.bundle.Notation = append(c.bundle.Notation, runtime.NotationValidator{})
	case runtime.VHexBinary:
		index = uint32(len(c.bundle.HexBinary))
		c.bundle.HexBinary = append(c.bundle.HexBinary, runtime.HexBinaryValidator{})
	case runtime.VBase64Binary:
		index = uint32(len(c.bundle.Base64Binary))
		c.bundle.Base64Binary = append(c.bundle.Base64Binary, runtime.Base64BinaryValidator{})
	default:
		index = uint32(len(c.bundle.String))
		c.bundle.String = append(c.bundle.String, runtime.StringValidator{})
	}

	id := runtime.ValidatorID(len(c.bundle.Meta))
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       kind,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      c.validatorFlags(facets),
	})
	return id
}

func (c *compiler) addListValidator(ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, item runtime.ValidatorID) runtime.ValidatorID {
	index := uint32(len(c.bundle.List))
	c.bundle.List = append(c.bundle.List, runtime.ListValidator{Item: item})

	id := runtime.ValidatorID(len(c.bundle.Meta))
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       runtime.VList,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      c.validatorFlags(facets),
	})
	return id
}

func (c *compiler) addUnionValidator(ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, members []runtime.ValidatorID, memberTypes []runtime.TypeID, unionName string, typeID runtime.TypeID) (runtime.ValidatorID, error) {
	if len(memberTypes) != len(members) {
		if typeID != 0 {
			return 0, fmt.Errorf("union member type count mismatch for %s (type %d): validators=%d memberTypes=%d", unionName, typeID, len(members), len(memberTypes))
		}
		return 0, fmt.Errorf("union member type count mismatch for %s: validators=%d memberTypes=%d", unionName, len(members), len(memberTypes))
	}
	off := uint32(len(c.bundle.UnionMembers))
	c.bundle.UnionMembers = append(c.bundle.UnionMembers, members...)
	c.bundle.UnionMemberTypes = append(c.bundle.UnionMemberTypes, memberTypes...)
	sameWS := make([]uint8, len(members))
	for i, member := range members {
		if int(member) >= len(c.bundle.Meta) {
			return 0, fmt.Errorf("union member validator %d out of range for %s", member, unionName)
		}
		if c.bundle.Meta[member].WhiteSpace == ws {
			sameWS[i] = 1
		}
	}
	c.bundle.UnionMemberSameWS = append(c.bundle.UnionMemberSameWS, sameWS...)
	index := uint32(len(c.bundle.Union))
	c.bundle.Union = append(c.bundle.Union, runtime.UnionValidator{
		MemberOff: off,
		MemberLen: uint32(len(members)),
	})

	id := runtime.ValidatorID(len(c.bundle.Meta))
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       runtime.VUnion,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      c.validatorFlags(facets),
	})
	return id, nil
}

func (c *compiler) validatorFlags(facets runtime.FacetProgramRef) runtime.ValidatorFlags {
	if facets.Len == 0 {
		return 0
	}
	end := facets.Off + facets.Len
	if int(end) > len(c.facets) {
		return 0
	}
	for i := facets.Off; i < end; i++ {
		if c.facets[i].Op == runtime.FEnum {
			return runtime.ValidatorHasEnum
		}
	}
	return 0
}
