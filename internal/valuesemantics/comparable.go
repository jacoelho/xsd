package valuesemantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

// ComparableForPrimitiveName decodes lexical input to comparable form for range facets.
func ComparableForPrimitiveName(primitive, lexical string, integerDerived bool) (model.ComparableValue, error) {
	switch primitive {
	case "decimal":
		if integerDerived {
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
		tv, err := temporal.ParsePrimitive(primitive, []byte(lexical))
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
		return nil, fmt.Errorf("unsupported comparable type %s", primitive)
	}
}
