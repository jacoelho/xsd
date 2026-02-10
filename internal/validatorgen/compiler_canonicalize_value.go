package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
)

func (c *compiler) comparableValue(lexical string, typ model.Type) (model.ComparableValue, error) {
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
			return model.ComparableInt{Value: v}, nil
		}
		dec, perr := num.ParseDec([]byte(lexical))
		if perr != nil {
			return nil, fmt.Errorf("invalid decimal: %s", lexical)
		}
		return model.ComparableDec{Value: dec}, nil
	case "float":
		v, err := value.ParseFloat([]byte(lexical))
		if err != nil {
			return nil, err
		}
		return model.ComparableFloat32{Value: v}, nil
	case "double":
		v, err := value.ParseDouble([]byte(lexical))
		if err != nil {
			return nil, err
		}
		return model.ComparableFloat64{Value: v}, nil
	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		tv, err := temporal.ParsePrimitive(primName, []byte(lexical))
		if err != nil {
			return nil, err
		}
		return model.ComparableTime{
			Value:        tv.Time,
			TimezoneKind: temporal.ValueTimezoneKind(tv.TimezoneKind),
			Kind:         tv.Kind,
			LeapSecond:   tv.LeapSecond,
		}, nil
	case "duration":
		dur, err := durationlex.Parse(lexical)
		if err != nil {
			return nil, err
		}
		return model.ComparableXSDDuration{Value: dur}, nil
	default:
		return nil, fmt.Errorf("unsupported comparable type %s", primName)
	}
}

func (c *compiler) normalizeLexical(lexical string, typ model.Type) string {
	ws := c.res.whitespaceMode(typ)
	if ws == runtime.WS_Preserve || lexical == "" {
		return lexical
	}
	normalized := value.NormalizeWhitespace(wsmode.ToValue(ws), []byte(lexical), nil)
	return string(normalized)
}
