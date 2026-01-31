package runtimebuild

import (
	"encoding/base64"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
)

type CompiledValidators struct {
	AttributeDefaults map[schema.AttrID]runtime.ValueRef
	TypeValidators    map[schema.TypeID]runtime.ValidatorID
	ValidatorByType   map[types.Type]runtime.ValidatorID
	ElementDefaults   map[schema.ElemID]runtime.ValueRef
	ElementFixed      map[schema.ElemID]runtime.ValueRef
	AttributeFixed    map[schema.AttrID]runtime.ValueRef
	AttrUseDefaults   map[*types.AttributeDecl]runtime.ValueRef
	AttrUseFixed      map[*types.AttributeDecl]runtime.ValueRef
	Validators        runtime.ValidatorsBundle
	Enums             runtime.EnumTable
	Facets            []runtime.FacetInstr
	Patterns          []runtime.Pattern
	Values            runtime.ValueBlob
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
	registry        *schema.Registry
	runtimeTypeIDs  map[schema.TypeID]runtime.TypeID
	builtinTypeIDs  map[types.TypeName]runtime.TypeID
	values          valueBuilder
	attrDefaults    map[schema.AttrID]runtime.ValueRef
	elemFixed       map[schema.ElemID]runtime.ValueRef
	simpleContent   map[*types.ComplexType]types.Type
	attrUseFixed    map[*types.AttributeDecl]runtime.ValueRef
	attrUseDefaults map[*types.AttributeDecl]runtime.ValueRef
	attrFixed       map[schema.AttrID]runtime.ValueRef
	res             *typeResolver
	elemDefaults    map[schema.ElemID]runtime.ValueRef
	facetsCache     map[*types.SimpleType][]types.Facet
	compiling       map[types.Type]bool
	validatorByType map[types.Type]runtime.ValidatorID
	schema          *parser.Schema
	bundle          runtime.ValidatorsBundle
	enums           enumBuilder
	facets          []runtime.FacetInstr
	patterns        []runtime.Pattern
}

func newCompiler(sch *parser.Schema) *compiler {
	return &compiler{
		schema:          sch,
		res:             newTypeResolver(sch),
		validatorByType: make(map[types.Type]runtime.ValidatorID),
		compiling:       make(map[types.Type]bool),
		facetsCache:     make(map[*types.SimpleType][]types.Facet),
		elemDefaults:    make(map[schema.ElemID]runtime.ValueRef),
		elemFixed:       make(map[schema.ElemID]runtime.ValueRef),
		attrDefaults:    make(map[schema.AttrID]runtime.ValueRef),
		attrFixed:       make(map[schema.AttrID]runtime.ValueRef),
		attrUseDefaults: make(map[*types.AttributeDecl]runtime.ValueRef),
		attrUseFixed:    make(map[*types.AttributeDecl]runtime.ValueRef),
		simpleContent:   make(map[*types.ComplexType]types.Type),
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
		if name == types.TypeNameAnyType || name == types.TypeNameAnySimpleType {
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
	return nil
}

func (c *compiler) result(registry *schema.Registry) *CompiledValidators {
	out := &CompiledValidators{
		Validators:        c.bundle,
		Facets:            c.facets,
		Patterns:          c.patterns,
		Enums:             c.enums.table(),
		Values:            c.values.table(),
		TypeValidators:    make(map[schema.TypeID]runtime.ValidatorID),
		ValidatorByType:   make(map[types.Type]runtime.ValidatorID),
		ElementDefaults:   c.elemDefaults,
		ElementFixed:      c.elemFixed,
		AttributeDefaults: c.attrDefaults,
		AttributeFixed:    c.attrFixed,
		AttrUseDefaults:   c.attrUseDefaults,
		AttrUseFixed:      c.attrUseFixed,
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

	if itemName, ok := builtinListItemTypeName(name); ok {
		itemType := types.GetBuiltin(itemName)
		if itemType == nil {
			return 0, fmt.Errorf("builtin list %s: item type %s not found", name, itemName)
		}
		itemID, err := c.compileType(itemType)
		if err != nil {
			return 0, err
		}
		return c.addListValidator(ws, runtime.FacetProgramRef{}, itemID), nil
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
		return c.addUnionValidator(ws, facetRef, memberIDs, memberTypeIDs), nil
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

func (c *compiler) compileFacetProgram(st *types.SimpleType, facets, partial []types.Facet) (runtime.FacetProgramRef, error) {
	if len(facets) == 0 {
		return runtime.FacetProgramRef{}, nil
	}
	start := len(c.facets)
	for _, facet := range facets {
		switch f := facet.(type) {
		case *types.Pattern:
			pid, err := c.addPattern(f)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FPattern, Arg0: uint32(pid)})
		case *types.PatternSet:
			pid, err := c.addPatternSet(f)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FPattern, Arg0: uint32(pid)})
		case *types.Enumeration:
			enumID, err := c.compileEnumeration(f, st, partial)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FEnum, Arg0: uint32(enumID)})
		case *types.Length:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FLength, Arg0: uint32(f.Value)})
		case *types.MinLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMinLength, Arg0: uint32(f.Value)})
		case *types.MaxLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMaxLength, Arg0: uint32(f.Value)})
		case *types.TotalDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FTotalDigits, Arg0: uint32(f.Value)})
		case *types.FractionDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FFractionDigits, Arg0: uint32(f.Value)})
		case *types.RangeFacet:
			op, ok := rangeFacetOp(f.Name())
			if !ok {
				return runtime.FacetProgramRef{}, fmt.Errorf("unknown range facet %s", f.Name())
			}
			lexical := f.GetLexical()
			canon, err := c.canonicalizeLexical(lexical, st, nil)
			if err != nil {
				return runtime.FacetProgramRef{}, fmt.Errorf("%s: %w", f.Name(), err)
			}
			ref := c.values.add(canon)
			c.facets = append(c.facets, runtime.FacetInstr{Op: op, Arg0: ref.Off, Arg1: ref.Len})
		default:
			// ignore unknown facets for now
		}
	}
	return runtime.FacetProgramRef{Off: uint32(start), Len: uint32(len(c.facets) - start)}, nil
}

func (c *compiler) compileEnumeration(enum *types.Enumeration, st *types.SimpleType, partial []types.Facet) (runtime.EnumID, error) {
	if enum == nil || len(enum.Values) == 0 {
		return 0, nil
	}
	contexts := enum.ValueContexts()
	if len(contexts) > 0 && len(contexts) != len(enum.Values) {
		return 0, fmt.Errorf("enumeration contexts %d do not match values %d", len(contexts), len(enum.Values))
	}
	keys := make([]runtime.ValueKey, 0, len(enum.Values))
	for i, val := range enum.Values {
		var ctx map[string]string
		if len(contexts) > 0 {
			ctx = contexts[i]
		}
		normalized := c.normalizeLexical(val, st)
		if err := c.validatePartialFacets(normalized, st, partial); err != nil {
			return 0, err
		}
		enumKeys, err := c.valueKeysForNormalized(normalized, st, ctx)
		if err != nil {
			return 0, err
		}
		keys = append(keys, enumKeys...)
	}
	return c.enums.add(keys), nil
}

func (c *compiler) canonicalizeLexical(lexical string, typ types.Type, ctx map[string]string) ([]byte, error) {
	normalized := c.normalizeLexical(lexical, typ)
	return c.canonicalizeNormalized(normalized, typ, ctx)
}

func (c *compiler) canonicalizeNormalized(normalized string, typ types.Type, ctx map[string]string) ([]byte, error) {
	switch c.res.varietyForType(typ) {
	case types.ListVariety:
		item, ok := c.res.listItemTypeFromType(typ)
		if !ok || item == nil {
			return nil, fmt.Errorf("list type missing item type")
		}
		items := splitXMLWhitespace(normalized)
		if len(items) == 0 {
			return []byte{}, nil
		}
		buf := strings.Builder{}
		for i, itemLex := range items {
			canon, err := c.canonicalizeNormalized(itemLex, item, ctx)
			if err != nil {
				return nil, err
			}
			if i > 0 {
				buf.WriteByte(' ')
			}
			buf.Write(canon)
		}
		return []byte(buf.String()), nil
	case types.UnionVariety:
		members := c.res.unionMemberTypesFromType(typ)
		if len(members) == 0 {
			return nil, fmt.Errorf("union has no member types")
		}
		for _, member := range members {
			memberLex := c.normalizeLexical(normalized, member)
			memberFacets, err := c.facetsForType(member)
			if err != nil {
				return nil, err
			}
			partial := filterFacets(memberFacets, func(f types.Facet) bool {
				_, ok := f.(*types.Enumeration)
				return !ok
			})
			err = c.validatePartialFacets(memberLex, member, partial)
			if err != nil {
				continue
			}
			canon, err := c.canonicalizeNormalized(memberLex, member, ctx)
			if err == nil {
				return canon, nil
			}
		}
		return nil, fmt.Errorf("union value does not match any member type")
	default:
		return c.canonicalizeAtomic(normalized, typ, ctx)
	}
}

func (c *compiler) canonicalizeAtomic(normalized string, typ types.Type, ctx map[string]string) ([]byte, error) {
	if c.res.isQNameOrNotation(typ) {
		resolver := mapResolver(ctx)
		return value.CanonicalQName([]byte(normalized), resolver, nil)
	}

	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return nil, err
	}

	switch primName {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN", "anyURI":
		return []byte(normalized), nil
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			v, err := value.ParseInteger([]byte(normalized))
			if err != nil {
				return nil, err
			}
			return []byte(v.String()), nil
		}
		_, err := value.ParseDecimal([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return value.CanonicalDecimalBytes([]byte(normalized), nil), nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		v, err := value.ParseInteger([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(v.String()), nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return nil, err
		}
		if v {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case "float":
		v, err := value.ParseFloat([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalFloat(float64(v), 32)), nil
	case "double":
		v, err := value.ParseDouble([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalFloat(v, 64)), nil
	case "dateTime":
		v, err := value.ParseDateTime([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "dateTime", value.HasTimezone([]byte(normalized)))), nil
	case "date":
		v, err := value.ParseDate([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "date", value.HasTimezone([]byte(normalized)))), nil
	case "time":
		v, err := value.ParseTime([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "time", value.HasTimezone([]byte(normalized)))), nil
	case "gYearMonth":
		v, err := value.ParseGYearMonth([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "gYearMonth", value.HasTimezone([]byte(normalized)))), nil
	case "gYear":
		v, err := value.ParseGYear([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "gYear", value.HasTimezone([]byte(normalized)))), nil
	case "gMonthDay":
		v, err := value.ParseGMonthDay([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "gMonthDay", value.HasTimezone([]byte(normalized)))), nil
	case "gDay":
		v, err := value.ParseGDay([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "gDay", value.HasTimezone([]byte(normalized)))), nil
	case "gMonth":
		v, err := value.ParseGMonth([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalDateTimeString(v, "gMonth", value.HasTimezone([]byte(normalized)))), nil
	case "duration":
		dur, err := types.ParseXSDDuration(normalized)
		if err != nil {
			return nil, err
		}
		return []byte(types.ComparableXSDDuration{Value: dur}.String()), nil
	case "hexBinary":
		b, err := types.ParseHexBinary(normalized)
		if err != nil {
			return nil, err
		}
		return []byte(strings.ToUpper(fmt.Sprintf("%x", b))), nil
	case "base64Binary":
		b, err := types.ParseBase64Binary(normalized)
		if err != nil {
			return nil, err
		}
		return []byte(encodeBase64(b)), nil
	default:
		return nil, fmt.Errorf("unsupported primitive type %s", primName)
	}
}

func (c *compiler) validatePartialFacets(normalized string, typ types.Type, facets []types.Facet) error {
	if len(facets) == 0 {
		return nil
	}
	for _, facet := range facets {
		if c.shouldSkipLengthFacet(typ, facet) {
			continue
		}
		switch f := facet.(type) {
		case *types.RangeFacet:
			if err := c.validateRangeFacet(normalized, typ, f); err != nil {
				return err
			}
		case *types.Enumeration:
			// enumeration handled separately
			continue
		case types.LexicalValidator:
			if err := f.ValidateLexical(normalized, typ); err != nil {
				return err
			}
		default:
			// ignore unsupported facets
		}
	}
	return nil
}

func (c *compiler) validateRangeFacet(normalized string, typ types.Type, facet *types.RangeFacet) error {
	actual, err := c.comparableValue(normalized, typ)
	if err != nil {
		return err
	}
	bound, err := c.comparableValue(facet.GetLexical(), typ)
	if err != nil {
		return err
	}
	cmp, err := actual.Compare(bound)
	if err != nil {
		return err
	}
	ok := false
	switch facet.Name() {
	case "minInclusive":
		ok = cmp >= 0
	case "maxInclusive":
		ok = cmp <= 0
	case "minExclusive":
		ok = cmp > 0
	case "maxExclusive":
		ok = cmp < 0
	default:
		return fmt.Errorf("unknown range facet %s", facet.Name())
	}
	if !ok {
		return fmt.Errorf("facet %s violation", facet.Name())
	}
	return nil
}

func (c *compiler) comparableValue(lexical string, typ types.Type) (types.ComparableValue, error) {
	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return nil, err
	}

	switch primName {
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			v, err := value.ParseInteger([]byte(lexical))
			if err != nil {
				return nil, err
			}
			return types.ComparableBigInt{Value: v}, nil
		}
		rat, err := value.ParseDecimal([]byte(lexical))
		if err != nil {
			return nil, err
		}
		return types.ComparableBigRat{Value: rat}, nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		v, err := value.ParseInteger([]byte(lexical))
		if err != nil {
			return nil, err
		}
		return types.ComparableBigInt{Value: v}, nil
	case "float":
		v, err := value.ParseFloat([]byte(lexical))
		if err != nil {
			return nil, err
		}
		return types.ComparableFloat32{Value: v}, nil
	case "double":
		v, err := value.ParseDouble([]byte(lexical))
		if err != nil {
			return nil, err
		}
		return types.ComparableFloat64{Value: v}, nil
	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		t, err := c.parseTemporal(primName, lexical)
		if err != nil {
			return nil, err
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone([]byte(lexical))}, nil
	case "duration":
		dur, err := types.ParseXSDDuration(lexical)
		if err != nil {
			return nil, err
		}
		return types.ComparableXSDDuration{Value: dur}, nil
	default:
		return nil, fmt.Errorf("unsupported comparable type %s", primName)
	}
}

func (c *compiler) parseTemporal(kind, lexical string) (time.Time, error) {
	switch kind {
	case "dateTime":
		return value.ParseDateTime([]byte(lexical))
	case "date":
		return value.ParseDate([]byte(lexical))
	case "time":
		return value.ParseTime([]byte(lexical))
	case "gYearMonth":
		return value.ParseGYearMonth([]byte(lexical))
	case "gYear":
		return value.ParseGYear([]byte(lexical))
	case "gMonthDay":
		return value.ParseGMonthDay([]byte(lexical))
	case "gDay":
		return value.ParseGDay([]byte(lexical))
	case "gMonth":
		return value.ParseGMonth([]byte(lexical))
	default:
		return time.Time{}, fmt.Errorf("unsupported temporal type %s", kind)
	}
}

func (c *compiler) normalizeLexical(lexical string, typ types.Type) string {
	if typ != nil && c.res.varietyForType(typ) == types.UnionVariety {
		return lexical
	}
	ws := c.res.whitespaceMode(typ)
	normalized := value.NormalizeWhitespace(ws, []byte(lexical), nil)
	return string(normalized)
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

func (c *compiler) addUnionValidator(ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, members []runtime.ValidatorID, memberTypes []runtime.TypeID) runtime.ValidatorID {
	off := uint32(len(c.bundle.UnionMembers))
	c.bundle.UnionMembers = append(c.bundle.UnionMembers, members...)
	if len(memberTypes) != len(members) {
		panic("union member type count mismatch")
	}
	c.bundle.UnionMemberTypes = append(c.bundle.UnionMemberTypes, memberTypes...)
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
	return id
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

func (c *compiler) addPattern(p *types.Pattern) (runtime.PatternID, error) {
	if p.GoPattern == "" {
		if err := p.ValidateSyntax(); err != nil {
			return 0, err
		}
	}
	re, err := regexp.Compile(p.GoPattern)
	if err != nil {
		return 0, err
	}
	c.patterns = append(c.patterns, runtime.Pattern{Source: []byte(p.GoPattern), Re: re})
	return runtime.PatternID(len(c.patterns) - 1), nil
}

func (c *compiler) addPatternSet(set *types.PatternSet) (runtime.PatternID, error) {
	if set == nil || len(set.Patterns) == 0 {
		return 0, nil
	}
	if len(set.Patterns) == 1 {
		return c.addPattern(set.Patterns[0])
	}
	bodies := make([]string, 0, len(set.Patterns))
	for _, pat := range set.Patterns {
		if pat.GoPattern == "" {
			if err := pat.ValidateSyntax(); err != nil {
				return 0, err
			}
		}
		body := stripAnchors(pat.GoPattern)
		bodies = append(bodies, body)
	}
	goPattern := "^(?:" + strings.Join(bodies, "|") + ")$"
	re, err := regexp.Compile(goPattern)
	if err != nil {
		return 0, err
	}
	c.patterns = append(c.patterns, runtime.Pattern{Source: []byte(goPattern), Re: re})
	return runtime.PatternID(len(c.patterns) - 1), nil
}

func (c *compiler) collectFacets(st *types.SimpleType) ([]types.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if cached, ok := c.facetsCache[st]; ok {
		return cached, nil
	}

	seen := make(map[*types.SimpleType]bool)
	facets, err := c.collectFacetsRecursive(st, seen)
	if err != nil {
		return nil, err
	}
	c.facetsCache[st] = facets
	return facets, nil
}

func (c *compiler) collectFacetsRecursive(st *types.SimpleType, seen map[*types.SimpleType]bool) ([]types.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if seen[st] {
		return nil, nil
	}
	seen[st] = true
	defer delete(seen, st)

	var result []types.Facet
	if base := c.res.baseType(st); base != nil {
		if baseST, ok := types.AsSimpleType(base); ok {
			baseFacets, err := c.collectFacetsRecursive(baseST, seen)
			if err != nil {
				return nil, err
			}
			result = append(result, baseFacets...)
		}
	}

	if st.Restriction != nil {
		var stepPatterns []*types.Pattern
		for _, f := range st.Restriction.Facets {
			switch facet := f.(type) {
			case types.Facet:
				if patternFacet, ok := facet.(*types.Pattern); ok {
					if err := patternFacet.ValidateSyntax(); err != nil {
						continue
					}
					stepPatterns = append(stepPatterns, patternFacet)
					continue
				}
				if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
					if err := compilable.ValidateSyntax(); err != nil {
						continue
					}
				}
				result = append(result, facet)
			case *types.DeferredFacet:
				base := c.res.baseType(st)
				resolved, err := convertDeferredFacet(facet, base)
				if err != nil {
					return nil, err
				}
				if resolved != nil {
					result = append(result, resolved)
				}
			}
		}
		if len(stepPatterns) == 1 {
			result = append(result, stepPatterns[0])
		} else if len(stepPatterns) > 1 {
			result = append(result, &types.PatternSet{Patterns: stepPatterns})
		}
	}

	return result, nil
}

func (c *compiler) facetsForType(typ types.Type) ([]types.Facet, error) {
	if st, ok := types.AsSimpleType(typ); ok {
		return c.collectFacets(st)
	}
	return nil, nil
}

func (c *compiler) validatorKind(st *types.SimpleType) (runtime.ValidatorKind, error) {
	primName, err := c.res.primitiveName(st)
	if err != nil {
		return 0, err
	}
	if primName == "decimal" && c.res.isIntegerDerived(st) {
		return runtime.VInteger, nil
	}
	return builtinValidatorKind(primName)
}

func builtinValidatorKind(name string) (runtime.ValidatorKind, error) {
	switch name {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return runtime.VString, nil
	case "boolean":
		return runtime.VBoolean, nil
	case "decimal":
		return runtime.VDecimal, nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		return runtime.VInteger, nil
	case "float":
		return runtime.VFloat, nil
	case "double":
		return runtime.VDouble, nil
	case "duration":
		return runtime.VDuration, nil
	case "dateTime":
		return runtime.VDateTime, nil
	case "time":
		return runtime.VTime, nil
	case "date":
		return runtime.VDate, nil
	case "gYearMonth":
		return runtime.VGYearMonth, nil
	case "gYear":
		return runtime.VGYear, nil
	case "gMonthDay":
		return runtime.VGMonthDay, nil
	case "gDay":
		return runtime.VGDay, nil
	case "gMonth":
		return runtime.VGMonth, nil
	case "anyURI":
		return runtime.VAnyURI, nil
	case "QName":
		return runtime.VQName, nil
	case "NOTATION":
		return runtime.VNotation, nil
	case "hexBinary":
		return runtime.VHexBinary, nil
	case "base64Binary":
		return runtime.VBase64Binary, nil
	default:
		return 0, fmt.Errorf("unsupported validator kind %s", name)
	}
}

func (c *compiler) stringKindForType(typ types.Type) runtime.StringKind {
	if c == nil || c.res == nil {
		return runtime.StringAny
	}
	name, ok := c.res.builtinNameForType(typ)
	if !ok {
		return runtime.StringAny
	}
	return stringKindForBuiltin(string(name))
}

func (c *compiler) integerKindForType(typ types.Type) runtime.IntegerKind {
	if c == nil || c.res == nil {
		return runtime.IntegerAny
	}
	name, ok := c.res.builtinNameForType(typ)
	if !ok {
		return runtime.IntegerAny
	}
	return integerKindForBuiltin(string(name))
}

func stringKindForBuiltin(name string) runtime.StringKind {
	switch name {
	case "normalizedString":
		return runtime.StringNormalized
	case "token":
		return runtime.StringToken
	case "language":
		return runtime.StringLanguage
	case "Name":
		return runtime.StringName
	case "NCName":
		return runtime.StringNCName
	case "ID":
		return runtime.StringID
	case "IDREF":
		return runtime.StringIDREF
	case "ENTITY":
		return runtime.StringEntity
	case "NMTOKEN":
		return runtime.StringNMTOKEN
	default:
		return runtime.StringAny
	}
}

func integerKindForBuiltin(name string) runtime.IntegerKind {
	switch name {
	case "long":
		return runtime.IntegerLong
	case "int":
		return runtime.IntegerInt
	case "short":
		return runtime.IntegerShort
	case "byte":
		return runtime.IntegerByte
	case "nonNegativeInteger":
		return runtime.IntegerNonNegative
	case "positiveInteger":
		return runtime.IntegerPositive
	case "nonPositiveInteger":
		return runtime.IntegerNonPositive
	case "negativeInteger":
		return runtime.IntegerNegative
	case "unsignedLong":
		return runtime.IntegerUnsignedLong
	case "unsignedInt":
		return runtime.IntegerUnsignedInt
	case "unsignedShort":
		return runtime.IntegerUnsignedShort
	case "unsignedByte":
		return runtime.IntegerUnsignedByte
	default:
		return runtime.IntegerAny
	}
}

func builtinListItemTypeName(name string) (types.TypeName, bool) {
	switch name {
	case "NMTOKENS":
		return types.TypeNameNMTOKEN, true
	case "IDREFS":
		return types.TypeNameIDREF, true
	case "ENTITIES":
		return types.TypeNameENTITY, true
	default:
		return "", false
	}
}

func rangeFacetOp(name string) (runtime.FacetOp, bool) {
	switch name {
	case "minInclusive":
		return runtime.FMinInclusive, true
	case "maxInclusive":
		return runtime.FMaxInclusive, true
	case "minExclusive":
		return runtime.FMinExclusive, true
	case "maxExclusive":
		return runtime.FMaxExclusive, true
	default:
		return 0, false
	}
}

func stripAnchors(goPattern string) string {
	const prefix = "^(?:"
	const suffix = ")$"
	if strings.HasPrefix(goPattern, prefix) && strings.HasSuffix(goPattern, suffix) {
		return goPattern[len(prefix) : len(goPattern)-len(suffix)]
	}
	return goPattern
}

func filterFacets(facets []types.Facet, keep func(types.Facet) bool) []types.Facet {
	if len(facets) == 0 {
		return nil
	}
	out := make([]types.Facet, 0, len(facets))
	for _, facet := range facets {
		if keep(facet) {
			out = append(out, facet)
		}
	}
	return out
}

func splitXMLWhitespace(input string) []string {
	var out []string
	for field := range types.FieldsXMLWhitespaceSeq(input) {
		out = append(out, field)
	}
	return out
}

type mapResolver map[string]string

func (m mapResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	if m == nil {
		return nil, false
	}
	ns, ok := m[string(prefix)]
	if !ok {
		return nil, false
	}
	return []byte(ns), true
}

func encodeBase64(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func (c *compiler) shouldSkipLengthFacet(typ types.Type, facet types.Facet) bool {
	if !types.IsLengthFacet(facet) {
		return false
	}
	if c.res.isListType(typ) {
		return false
	}
	return c.res.isQNameOrNotation(typ)
}

func convertDeferredFacet(df *types.DeferredFacet, base types.Type) (types.Facet, error) {
	if df == nil || base == nil {
		return nil, nil
	}
	switch df.FacetName {
	case "minInclusive":
		return types.NewMinInclusive(df.FacetValue, base)
	case "maxInclusive":
		return types.NewMaxInclusive(df.FacetValue, base)
	case "minExclusive":
		return types.NewMinExclusive(df.FacetValue, base)
	case "maxExclusive":
		return types.NewMaxExclusive(df.FacetValue, base)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", df.FacetName)
	}
}
