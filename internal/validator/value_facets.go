package validator

import (
	"bytes"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) applyFacets(meta runtime.ValidatorMeta, normalized, canonical []byte, metrics *valueMetrics) error {
	if s == nil || s.rt == nil {
		return valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	if meta.Facets.Len == 0 {
		return nil
	}
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	if start < 0 || end < 0 || end > len(s.rt.Facets) {
		return valueErrorf(valueErrInvalid, "facet program out of range")
	}
	for _, instr := range s.rt.Facets[start:end] {
		switch instr.Op {
		case runtime.FPattern:
			if metrics != nil && metrics.patternChecked {
				continue
			}
			if int(instr.Arg0) >= len(s.rt.Patterns) {
				return valueErrorf(valueErrInvalid, "pattern %d out of range", instr.Arg0)
			}
			pat := s.rt.Patterns[instr.Arg0]
			if pat.Re != nil && !pat.Re.Match(normalized) {
				return valueErrorf(valueErrFacet, "pattern violation")
			}
		case runtime.FEnum:
			if metrics != nil && metrics.enumChecked {
				continue
			}
			enumID := runtime.EnumID(instr.Arg0)
			if metrics == nil || !metrics.keySet {
				kind, key, err := s.deriveKeyFromCanonical(meta.Kind, canonical)
				if err != nil {
					return err
				}
				if metrics != nil {
					metrics.keyKind = kind
					metrics.keyBytes = key
					metrics.keySet = true
				}
				if !runtime.EnumContains(&s.rt.Enums, enumID, kind, key) {
					return valueErrorf(valueErrFacet, "enumeration violation")
				}
			} else if !runtime.EnumContains(&s.rt.Enums, enumID, metrics.keyKind, metrics.keyBytes) {
				return valueErrorf(valueErrFacet, "enumeration violation")
			}
		case runtime.FMinInclusive, runtime.FMaxInclusive, runtime.FMinExclusive, runtime.FMaxExclusive:
			ref := runtime.ValueRef{Off: instr.Arg0, Len: instr.Arg1, Present: true}
			bound := valueBytes(s.rt.Values, ref)
			if bound == nil {
				return valueErrorf(valueErrInvalid, "range facet bound out of range")
			}
			switch meta.Kind {
			case runtime.VFloat:
				if err := s.checkFloat32Range(instr.Op, canonical, bound, metrics); err != nil {
					return err
				}
			case runtime.VDouble:
				if err := s.checkFloat64Range(instr.Op, canonical, bound, metrics); err != nil {
					return err
				}
			default:
				cmp, err := s.compareValue(meta.Kind, canonical, bound, metrics)
				if err != nil {
					return err
				}
				if err := compareRange(instr.Op, cmp); err != nil {
					return err
				}
			}
		case runtime.FLength, runtime.FMinLength, runtime.FMaxLength:
			if shouldSkipRuntimeLengthFacet(meta.Kind) {
				continue
			}
			length, err := s.valueLength(meta, normalized, metrics)
			if err != nil {
				return err
			}
			switch instr.Op {
			case runtime.FLength:
				if length != int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "length violation")
				}
			case runtime.FMinLength:
				if length < int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "minLength violation")
				}
			case runtime.FMaxLength:
				if length > int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "maxLength violation")
				}
			}
		case runtime.FTotalDigits, runtime.FFractionDigits:
			total, fraction, err := digitCounts(meta.Kind, canonical, metrics)
			if err != nil {
				return err
			}
			switch instr.Op {
			case runtime.FTotalDigits:
				if total > int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "totalDigits violation")
				}
			case runtime.FFractionDigits:
				if fraction > int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "fractionDigits violation")
				}
			}
		default:
			return valueErrorf(valueErrInvalid, "unknown facet op %d", instr.Op)
		}
	}
	return nil
}

func (s *Session) facetProgram(meta runtime.ValidatorMeta) ([]runtime.FacetInstr, error) {
	if s == nil || s.rt == nil {
		return nil, valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	if meta.Facets.Len == 0 {
		return nil, nil
	}
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	if start < 0 || end < 0 || end > len(s.rt.Facets) {
		return nil, valueErrorf(valueErrInvalid, "facet program out of range")
	}
	return s.rt.Facets[start:end], nil
}

func (s *Session) hasLengthFacet(meta runtime.ValidatorMeta) bool {
	if s == nil || s.rt == nil || meta.Facets.Len == 0 {
		return false
	}
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	if start < 0 || end < 0 || end > len(s.rt.Facets) {
		return false
	}
	for _, instr := range s.rt.Facets[start:end] {
		switch instr.Op {
		case runtime.FLength, runtime.FMinLength, runtime.FMaxLength:
			return true
		}
	}
	return false
}

func (s *Session) valueLength(meta runtime.ValidatorMeta, normalized []byte, metrics *valueMetrics) (int, error) {
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
		count := listItemCount(normalized, meta.WhiteSpace == runtime.WS_Collapse)
		if metrics != nil {
			metrics.length = count
			metrics.lengthSet = true
		}
		return count, nil
	case runtime.VHexBinary:
		return binaryOctetLength(types.ParseHexBinary, normalized, metrics, "hexBinary")
	case runtime.VBase64Binary:
		return binaryOctetLength(types.ParseBase64Binary, normalized, metrics, "base64Binary")
	default:
		return utf8.RuneCount(normalized), nil
	}
}

func binaryOctetLength(parse func(string) ([]byte, error), normalized []byte, metrics *valueMetrics, label string) (int, error) {
	decoded, err := parse(string(normalized))
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

func (s *Session) compareValue(kind runtime.ValidatorKind, canonical, bound []byte, metrics *valueMetrics) (int, error) {
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
		val, err := types.ParseXSDDuration(string(canonical))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		boundVal, err := types.ParseXSDDuration(string(bound))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		cmp, err := types.ComparableXSDDuration{Value: val}.Compare(types.ComparableXSDDuration{Value: boundVal})
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		valTime, valHasTZ, err := parseTemporalForKind(kind, canonical)
		if err != nil {
			return 0, err
		}
		boundTime, boundHasTZ, err := parseTemporalForKind(kind, bound)
		if err != nil {
			return 0, err
		}
		comp := types.ComparableTime{Value: valTime, HasTimezone: valHasTZ}
		boundComp := types.ComparableTime{Value: boundTime, HasTimezone: boundHasTZ}
		cmp, err := comp.Compare(boundComp)
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	default:
		return 0, valueErrorf(valueErrInvalid, "unsupported comparable type %d", kind)
	}
}

func digitCounts(kind runtime.ValidatorKind, canonical []byte, metrics *valueMetrics) (int, int, error) {
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

func shouldSkipRuntimeLengthFacet(kind runtime.ValidatorKind) bool {
	return kind == runtime.VQName || kind == runtime.VNotation
}

func (s *Session) deriveKeyFromCanonical(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	switch kind {
	case runtime.VString:
		key := valuekey.StringKeyBytes(s.keyTmp[:0], 0, canonical)
		s.keyTmp = key
		return runtime.VKString, key, nil
	case runtime.VBoolean:
		switch {
		case bytes.Equal(canonical, []byte("true")):
			return runtime.VKBool, []byte{1}, nil
		case bytes.Equal(canonical, []byte("false")):
			return runtime.VKBool, []byte{0}, nil
		default:
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid boolean")
		}
	case runtime.VDecimal:
		decVal, perr := num.ParseDec(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		key := num.EncodeDecKey(s.keyTmp[:0], decVal)
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VInteger:
		intVal, perr := num.ParseInt(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		key := num.EncodeIntKey(s.keyTmp[:0], intVal)
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VFloat:
		v, class, perr := num.ParseFloat32(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid float")
		}
		key := valuekey.Float32Key(s.keyTmp[:0], v, class)
		s.keyTmp = key
		return runtime.VKFloat32, key, nil
	case runtime.VDouble:
		v, class, perr := num.ParseFloat64(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid double")
		}
		key := valuekey.Float64Key(s.keyTmp[:0], v, class)
		s.keyTmp = key
		return runtime.VKFloat64, key, nil
	case runtime.VAnyURI:
		key := valuekey.StringKeyBytes(s.keyTmp[:0], 1, canonical)
		s.keyTmp = key
		return runtime.VKString, key, nil
	case runtime.VQName, runtime.VNotation:
		tag := byte(0)
		if kind == runtime.VNotation {
			tag = 1
		}
		key := valuekey.QNameKeyCanonical(s.keyTmp[:0], tag, canonical)
		if len(key) == 0 {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "invalid QName key")
		}
		s.keyTmp = key
		return runtime.VKQName, key, nil
	case runtime.VHexBinary:
		decoded, err := types.ParseHexBinary(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	case runtime.VBase64Binary:
		decoded, err := types.ParseBase64Binary(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	case runtime.VDuration:
		dur, err := types.ParseXSDDuration(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuekey.DurationKeyBytes(s.keyTmp[:0], dur)
		s.keyTmp = key
		return runtime.VKDuration, key, nil
	case runtime.VDateTime, runtime.VDate, runtime.VTime, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		t, hasTZ, err := parseTemporalForKind(kind, canonical)
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		key := valuekey.TemporalKeyBytes(s.keyTmp[:0], temporalSubkind(kind), t, hasTZ)
		s.keyTmp = key
		return runtime.VKDateTime, key, nil
	default:
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", kind)
	}
}

func (s *Session) decForCanonical(canonical []byte, metrics *valueMetrics) (num.Dec, error) {
	if metrics != nil && metrics.decSet {
		return metrics.decVal, nil
	}
	val, perr := num.ParseDec(canonical)
	if perr != nil {
		return num.Dec{}, valueErrorMsg(valueErrInvalid, "invalid decimal")
	}
	if metrics != nil {
		metrics.decVal = val
		metrics.decSet = true
	}
	return val, nil
}

func (s *Session) intForCanonical(canonical []byte, metrics *valueMetrics) (num.Int, error) {
	if metrics != nil && metrics.intSet {
		return metrics.intVal, nil
	}
	val, perr := num.ParseInt(canonical)
	if perr != nil {
		return num.Int{}, valueErrorMsg(valueErrInvalid, "invalid integer")
	}
	if metrics != nil {
		metrics.intVal = val
		metrics.intSet = true
	}
	return val, nil
}

func (s *Session) float32ForCanonical(canonical []byte, metrics *valueMetrics) (float32, num.FloatClass, error) {
	if metrics != nil && metrics.float32Set {
		return metrics.float32Val, metrics.float32Class, nil
	}
	val, class, perr := num.ParseFloat32(canonical)
	if perr != nil {
		return 0, num.FloatFinite, valueErrorMsg(valueErrInvalid, "invalid float")
	}
	if metrics != nil {
		metrics.float32Val = val
		metrics.float32Class = class
		metrics.float32Set = true
	}
	return val, class, nil
}

func (s *Session) float64ForCanonical(canonical []byte, metrics *valueMetrics) (float64, num.FloatClass, error) {
	if metrics != nil && metrics.float64Set {
		return metrics.float64Val, metrics.float64Class, nil
	}
	val, class, perr := num.ParseFloat64(canonical)
	if perr != nil {
		return 0, num.FloatFinite, valueErrorMsg(valueErrInvalid, "invalid double")
	}
	if metrics != nil {
		metrics.float64Val = val
		metrics.float64Class = class
		metrics.float64Set = true
	}
	return val, class, nil
}

func (s *Session) checkFloat32Range(op runtime.FacetOp, canonical, bound []byte, metrics *valueMetrics) error {
	val, valClass, err := s.float32ForCanonical(canonical, metrics)
	if err != nil {
		return err
	}
	boundVal, boundClass, perr := num.ParseFloat32(bound)
	if perr != nil {
		return valueErrorMsg(valueErrInvalid, "invalid float")
	}
	if boundClass == num.FloatNaN {
		if op == runtime.FMinInclusive || op == runtime.FMaxInclusive {
			if valClass == num.FloatNaN {
				return nil
			}
		}
		return rangeViolation(op)
	}
	if valClass == num.FloatNaN {
		return rangeViolation(op)
	}
	cmp, _ := num.CompareFloat32(val, valClass, boundVal, boundClass)
	return compareRange(op, cmp)
}

func (s *Session) checkFloat64Range(op runtime.FacetOp, canonical, bound []byte, metrics *valueMetrics) error {
	val, valClass, err := s.float64ForCanonical(canonical, metrics)
	if err != nil {
		return err
	}
	boundVal, boundClass, perr := num.ParseFloat64(bound)
	if perr != nil {
		return valueErrorMsg(valueErrInvalid, "invalid double")
	}
	if boundClass == num.FloatNaN {
		if op == runtime.FMinInclusive || op == runtime.FMaxInclusive {
			if valClass == num.FloatNaN {
				return nil
			}
		}
		return rangeViolation(op)
	}
	if valClass == num.FloatNaN {
		return rangeViolation(op)
	}
	cmp, _ := num.CompareFloat64(val, valClass, boundVal, boundClass)
	return compareRange(op, cmp)
}

func compareRange(op runtime.FacetOp, cmp int) error {
	switch op {
	case runtime.FMinInclusive:
		if cmp < 0 {
			return rangeViolation(op)
		}
	case runtime.FMaxInclusive:
		if cmp > 0 {
			return rangeViolation(op)
		}
	case runtime.FMinExclusive:
		if cmp <= 0 {
			return rangeViolation(op)
		}
	case runtime.FMaxExclusive:
		if cmp >= 0 {
			return rangeViolation(op)
		}
	default:
		return rangeViolation(op)
	}
	return nil
}

func rangeViolation(op runtime.FacetOp) error {
	switch op {
	case runtime.FMinInclusive:
		return valueErrorf(valueErrFacet, "minInclusive violation")
	case runtime.FMaxInclusive:
		return valueErrorf(valueErrFacet, "maxInclusive violation")
	case runtime.FMinExclusive:
		return valueErrorf(valueErrFacet, "minExclusive violation")
	case runtime.FMaxExclusive:
		return valueErrorf(valueErrFacet, "maxExclusive violation")
	default:
		return valueErrorf(valueErrFacet, "range violation")
	}
}
