package validator

import (
	"unsafe"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

func (s *Session) compareValue(kind runtime.ValidatorKind, canonical, bound []byte, metrics *ValueMetrics) (int, error) {
	switch kind {
	case runtime.VDecimal:
		val, err := s.decForCanonical(canonical, metrics)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseDec(bound)
		if perr != nil {
			return 0, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		return val.Compare(boundVal), nil
	case runtime.VInteger:
		val, err := s.intForCanonical(canonical, metrics)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseInt(bound)
		if perr != nil {
			return 0, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		return val.Compare(boundVal), nil
	case runtime.VDuration:
		val, err := durationlex.Parse(unsafe.String(unsafe.SliceData(canonical), len(canonical)))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		boundVal, err := durationlex.Parse(unsafe.String(unsafe.SliceData(bound), len(bound)))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		cmp, err := durationlex.Compare(val, boundVal)
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		valTemporal, err := parseTemporalForKind(kind, canonical)
		if err != nil {
			return 0, err
		}
		boundTemporal, err := parseTemporalForKind(kind, bound)
		if err != nil {
			return 0, err
		}
		cmp, err := temporal.Compare(valTemporal, boundTemporal)
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	default:
		return 0, valueErrorf(valueErrInvalid, "unsupported comparable type %d", kind)
	}
}
