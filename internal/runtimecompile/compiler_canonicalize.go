package runtimecompile

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
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
