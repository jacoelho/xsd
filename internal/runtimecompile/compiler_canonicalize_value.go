package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

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
