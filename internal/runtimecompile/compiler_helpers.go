package runtimecompile

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

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
			normalized := c.normalizeLexical(lexical, st)
			canon, err := c.canonicalizeNormalized(lexical, normalized, st, nil)
			if err != nil {
				return runtime.FacetProgramRef{}, fmt.Errorf("%s: %w", f.Name(), err)
			}
			stored := canon
			ref := c.values.add(stored)
			c.facets = append(c.facets, runtime.FacetInstr{Op: op, Arg0: ref.Off, Arg1: ref.Len})
		default:
			// ignore unknown facets for now
		}
	}
	return runtime.FacetProgramRef{Off: uint32(start), Len: uint32(len(c.facets) - start)}, nil
}

func (c *compiler) compileEnumeration(enum *types.Enumeration, st *types.SimpleType, partial []types.Facet) (runtime.EnumID, error) {
	if enum == nil {
		return 0, nil
	}
	values := enum.Values()
	if len(values) == 0 {
		return 0, nil
	}
	contexts := enum.ValueContexts()
	if len(contexts) > 0 && len(contexts) != len(values) {
		return 0, fmt.Errorf("enumeration contexts %d do not match values %d", len(contexts), len(values))
	}
	keys := make([]runtime.ValueKey, 0, len(values))
	for i, val := range values {
		var ctx map[string]string
		if len(contexts) > 0 {
			ctx = contexts[i]
		}
		normalized := c.normalizeLexical(val, st)
		if err := c.validatePartialFacets(normalized, st, partial); err != nil {
			return 0, err
		}
		enumKeys, err := c.valueKeysForNormalized(val, normalized, st, ctx)
		if err != nil {
			return 0, err
		}
		keys = append(keys, enumKeys...)
	}
	return c.enums.add(keys), nil
}

type canonicalizeMode uint8

const (
	// canonicalizeGeneral is used for ordinary validation, applying full facet checks.
	canonicalizeGeneral canonicalizeMode = iota
	// canonicalizeDefault is used for default/fixed values so unions follow runtime default validation order.
	canonicalizeDefault
)

func (c *compiler) canonicalizeNormalized(lexical, normalized string, typ types.Type, ctx map[string]string) ([]byte, error) {
	return c.canonicalizeNormalizedCore(lexical, normalized, typ, ctx, canonicalizeGeneral)
}

func (c *compiler) canonicalizeNormalizedCore(lexical, normalized string, typ types.Type, ctx map[string]string, mode canonicalizeMode) ([]byte, error) {
	switch c.res.varietyForType(typ) {
	case types.ListVariety:
		item, ok := c.res.listItemTypeFromType(typ)
		if !ok || item == nil {
			return nil, fmt.Errorf("list type missing item type")
		}
		var buf []byte
		count := 0
		for itemLex := range types.FieldsXMLWhitespaceSeq(normalized) {
			itemNorm := c.normalizeLexical(itemLex, item)
			canon, err := c.canonicalizeNormalizedCore(itemLex, itemNorm, item, ctx, mode)
			if err != nil {
				return nil, err
			}
			if count > 0 {
				buf = append(buf, ' ')
			}
			buf = append(buf, canon...)
			count++
		}
		if count == 0 {
			return []byte{}, nil
		}
		return buf, nil
	case types.UnionVariety:
		members := c.res.unionMemberTypesFromType(typ)
		if len(members) == 0 {
			return nil, fmt.Errorf("union has no member types")
		}
		for _, member := range members {
			memberLex := c.normalizeLexical(lexical, member)
			memberFacets, facetErr := c.facetsForType(member)
			if facetErr != nil {
				return nil, facetErr
			}
			switch mode {
			case canonicalizeDefault:
				if validateErr := c.validatePartialFacets(memberLex, member, memberFacets); validateErr != nil {
					continue
				}
				canon, canonErr := c.canonicalizeNormalizedCore(lexical, memberLex, member, ctx, mode)
				if canonErr != nil {
					continue
				}
				if enumErr := c.validateEnumSets(lexical, memberLex, member, ctx); enumErr != nil {
					continue
				}
				return canon, nil
			default:
				if validateErr := c.validateMemberFacets(memberLex, member, memberFacets, ctx, true); validateErr != nil {
					continue
				}
				canon, canonErr := c.canonicalizeNormalizedCore(lexical, memberLex, member, ctx, mode)
				if canonErr == nil {
					return canon, nil
				}
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
	case "string":
		if err := runtime.ValidateStringKind(c.stringKindForType(typ), []byte(normalized)); err != nil {
			return nil, err
		}
		return []byte(normalized), nil
	case "anyURI":
		if err := value.ValidateAnyURI([]byte(normalized)); err != nil {
			return nil, err
		}
		return []byte(normalized), nil
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			v, perr := num.ParseInt([]byte(normalized))
			if perr != nil {
				return nil, fmt.Errorf("invalid integer: %s", normalized)
			}
			if err := runtime.ValidateIntegerKind(c.integerKindForType(typ), v); err != nil {
				return nil, err
			}
			return v.RenderCanonical(nil), nil
		}
		v, perr := num.ParseDec([]byte(normalized))
		if perr != nil {
			return nil, fmt.Errorf("invalid decimal: %s", normalized)
		}
		return v.RenderCanonical(nil), nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		v, perr := num.ParseInt([]byte(normalized))
		if perr != nil {
			return nil, fmt.Errorf("invalid integer: %s", normalized)
		}
		if err := runtime.ValidateIntegerKind(c.integerKindForType(typ), v); err != nil {
			return nil, err
		}
		return v.RenderCanonical(nil), nil
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
		tv, err := temporal.Parse(temporal.KindDateTime, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "date":
		tv, err := temporal.Parse(temporal.KindDate, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "time":
		tv, err := temporal.Parse(temporal.KindTime, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gYearMonth":
		tv, err := temporal.Parse(temporal.KindGYearMonth, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gYear":
		tv, err := temporal.Parse(temporal.KindGYear, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gMonthDay":
		tv, err := temporal.Parse(temporal.KindGMonthDay, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gDay":
		tv, err := temporal.Parse(temporal.KindGDay, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gMonth":
		tv, err := temporal.Parse(temporal.KindGMonth, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
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
		return []byte(base64.StdEncoding.EncodeToString(b)), nil
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

func (c *compiler) validateMemberFacets(normalized string, typ types.Type, facets []types.Facet, ctx map[string]string, includeEnum bool) error {
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
			if !includeEnum {
				continue
			}
			if c.res.isQNameOrNotation(typ) {
				if err := f.ValidateLexicalQName(normalized, typ, ctx); err != nil {
					return err
				}
				continue
			}
			if err := f.ValidateLexical(normalized, typ); err != nil {
				return err
			}
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
			v, perr := num.ParseInt([]byte(lexical))
			if perr != nil {
				return nil, fmt.Errorf("invalid integer: %s", lexical)
			}
			return types.ComparableInt{Value: v}, nil
		}
		dec, perr := num.ParseDec([]byte(lexical))
		if perr != nil {
			return nil, fmt.Errorf("invalid decimal: %s", lexical)
		}
		return types.ComparableDec{Value: dec}, nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		v, perr := num.ParseInt([]byte(lexical))
		if perr != nil {
			return nil, fmt.Errorf("invalid integer: %s", lexical)
		}
		return types.ComparableInt{Value: v}, nil
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
		tv, err := temporal.ParsePrimitive(primName, []byte(lexical))
		if err != nil {
			return nil, err
		}
		return types.ComparableTime{
			Value:        tv.Time,
			TimezoneKind: temporal.ValueTimezoneKind(tv.TimezoneKind),
			Kind:         tv.Kind,
			LeapSecond:   tv.LeapSecond,
		}, nil
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

func (c *compiler) normalizeLexical(lexical string, typ types.Type) string {
	ws := c.res.whitespaceMode(typ)
	if ws == runtime.WS_Preserve || lexical == "" {
		return lexical
	}
	normalized := value.NormalizeWhitespace(toValueWhitespaceMode(ws), []byte(lexical), nil)
	return string(normalized)
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

	if st.IsBuiltin() && isBuiltinListName(st.Name().Local) {
		result = append(result, &types.MinLength{Value: 1})
	} else if base := c.res.baseType(st); base != nil {
		if bt := builtinForType(base); bt != nil && isBuiltinListName(bt.Name().Local) {
			result = append(result, &types.MinLength{Value: 1})
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
				resolved, err := typeops.DefaultDeferredFacetConverter(facet, base)
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
	if bt, ok := types.AsBuiltinType(typ); ok {
		if isBuiltinListName(bt.Name().Local) {
			return []types.Facet{&types.MinLength{Value: 1}}, nil
		}
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
	case "anySimpleType":
		return runtime.VString, nil
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

func (c *compiler) shouldSkipLengthFacet(typ types.Type, facet types.Facet) bool {
	if !types.IsLengthFacet(facet) {
		return false
	}
	if c.res.isListType(typ) {
		return false
	}
	return c.res.isQNameOrNotation(typ)
}
