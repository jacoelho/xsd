package validator

import (
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) valueLength(meta runtime.ValidatorMeta, normalized []byte, metrics *ValueMetrics) (int, error) {
	if metrics != nil && metrics.lengthSet {
		return metrics.length, nil
	}
	switch meta.Kind {
	case runtime.VList:
		if metrics != nil && metrics.listSet {
			metrics.length = metrics.listCount
			metrics.lengthSet = true
			return metrics.length, nil
		}
		count := listItemCount(normalized)
		if metrics != nil {
			metrics.length = count
			metrics.lengthSet = true
		}
		return count, nil
	case runtime.VHexBinary:
		return binaryOctetLength(value.ParseHexBinary, normalized, metrics, "hexBinary")
	case runtime.VBase64Binary:
		return binaryOctetLength(value.ParseBase64Binary, normalized, metrics, "base64Binary")
	default:
		return utf8.RuneCount(normalized), nil
	}
}

func binaryOctetLength(parse func([]byte) ([]byte, error), normalized []byte, metrics *ValueMetrics, label string) (int, error) {
	decoded, err := parse(normalized)
	if err != nil {
		return 0, valueErrorf(valueErrInvalid, "invalid %s", label)
	}
	length := len(decoded)
	if metrics != nil {
		metrics.length = length
		metrics.lengthSet = true
	}
	return length, nil
}

func digitCounts(kind runtime.ValidatorKind, canonical []byte, metrics *ValueMetrics) (int, int, error) {
	if metrics != nil && metrics.digitsSet {
		return metrics.totalDigits, metrics.fractionDigits, nil
	}
	if kind != runtime.VDecimal && kind != runtime.VInteger {
		return 0, 0, valueErrorf(valueErrInvalid, "digits facet not applicable")
	}
	total := 0
	fraction := 0
	switch kind {
	case runtime.VDecimal:
		var dec num.Dec
		if metrics != nil && metrics.decSet {
			dec = metrics.decVal
		} else {
			decVal, perr := num.ParseDec(canonical)
			if perr != nil {
				return 0, 0, valueErrorMsg(valueErrInvalid, "invalid decimal")
			}
			dec = decVal
		}
		total = len(dec.Coef)
		fraction = int(dec.Scale)
	case runtime.VInteger:
		var intVal num.Int
		if metrics != nil && metrics.intSet {
			intVal = metrics.intVal
		} else {
			parsed, perr := num.ParseInt(canonical)
			if perr != nil {
				return 0, 0, valueErrorMsg(valueErrInvalid, "invalid integer")
			}
			intVal = parsed
		}
		total = len(intVal.Digits)
		fraction = 0
	}
	if metrics != nil {
		metrics.totalDigits = total
		metrics.fractionDigits = fraction
		metrics.digitsSet = true
	}
	return total, fraction, nil
}
