package xsd

import (
	"fmt"
	"unicode/utf8"
)

func validateBuiltinDerived(kind builtinValidationKind, norm string, actual actualValue) error {
	switch kind {
	case builtinValidationNone:
		return nil
	case builtinValidationInteger:
		dec := actual.Decimal
		if !actual.Valid || actual.Kind != primDecimal {
			var err error
			dec, err = parseDecimalValue(norm)
			if err != nil {
				return err
			}
		}
		if !dec.IntegerLexical {
			return fmt.Errorf("invalid integer")
		}
	case builtinValidationName:
		if !isXMLName(norm) {
			return fmt.Errorf("invalid Name")
		}
	case builtinValidationNCName, builtinValidationEntity:
		if !isNCName(norm) {
			return fmt.Errorf("invalid NCName")
		}
		if kind == builtinValidationEntity {
			return unsupported(ErrUnsupportedEntity, "ENTITY requires DTD entity declarations, which are not supported")
		}
	case builtinValidationNMTOKEN:
		if !isNMTOKEN(norm) {
			return fmt.Errorf("invalid NMTOKEN")
		}
	case builtinValidationLanguage:
		if !isLanguage(norm) {
			return fmt.Errorf("invalid language")
		}
	case builtinValidationXMLLang:
		if norm != "" && !isLanguage(norm) {
			return fmt.Errorf("invalid language")
		}
	case builtinValidationXMLSpace:
		if norm != xmlValueDefault && norm != xmlValuePreserve {
			return fmt.Errorf("invalid xml:space")
		}
	}
	return nil
}

func applyAtomicFacets(st *simpleType, norm string, actual actualValue) error {
	if st.Facets.needsLength() {
		if err := applyAtomicLengthFacets(st, norm, actual); err != nil {
			return err
		}
	}
	if st.Primitive == primDecimal {
		dec := actual.Decimal
		if !actual.Valid || actual.Kind != primDecimal {
			var err error
			dec, err = parseDecimalValue(norm)
			if err != nil {
				return err
			}
		}
		return applyDecimalFacets(st.Facets, dec)
	}
	return applyPrimitiveBounds(st.Primitive, st.Facets, norm, actual)
}

func applyAtomicLengthFacets(st *simpleType, norm string, actual actualValue) error {
	if st.Primitive == primQName || st.Primitive == primNotation {
		return nil
	}
	length := actual.Length
	if !actual.Valid || actual.Kind != st.Primitive {
		var err error
		length, err = atomicLength(st.Primitive, norm)
		if err != nil {
			return err
		}
	}
	return applyLengthFacets(st.Facets, length)
}

func applyDecimalFacets(f facetSet, dec decimalValue) error {
	if f.TotalDigits != nil && dec.TotalDigits > *f.TotalDigits {
		return fmt.Errorf("totalDigits facet failed")
	}
	if f.FractionDigits != nil && dec.FractionDigits > *f.FractionDigits {
		return fmt.Errorf("fractionDigits facet failed")
	}
	return applyDecimalBounds(f, dec)
}

func validateDecimalNoOutput(f facetSet, norm string) error {
	dec, err := parseDecimalMode(norm, decimalValueOnly)
	if err != nil {
		return err
	}
	return applyDecimalFacets(f, dec)
}

func applyPrimitiveBounds(kind primitiveKind, f facetSet, norm string, actual actualValue) error {
	switch kind {
	case primFloat, primDouble:
		return applyFloatBounds(kind, f, norm, actual)
	case primDuration:
		return applyDurationBounds(f, norm, actual)
	case primGDay, primGMonthDay, primGMonth, primGYearMonth, primGYear:
		_, parse, ok := gValueFacet(kind)
		if !ok {
			return fmt.Errorf("invalid g value primitive")
		}
		return applyGValueBounds(kind, f, norm, actual, parse)
	case primDate, primDateTime, primTime:
		return applyTemporalBounds(kind, f, norm, actual)
	default:
		return nil
	}
}

func atomicLength(kind primitiveKind, norm string) (uint32, error) {
	switch kind {
	case primHexBinary:
		return hexBinaryLength(norm)
	case primBase64Binary:
		return base64BinaryLength(norm)
	default:
		return checkedUint32(utf8.RuneCountInString(norm), "value length exceeds uint32 limit")
	}
}

func applyLengthFacets(f facetSet, length uint32) error {
	if f.Length != nil && length != *f.Length {
		return fmt.Errorf("length facet failed")
	}
	if f.MinLength != nil && length < *f.MinLength {
		return fmt.Errorf("minLength facet failed")
	}
	if f.MaxLength != nil && length > *f.MaxLength {
		return fmt.Errorf("maxLength facet failed")
	}
	return nil
}

func applyPatterns(f facetSet, norm string) error {
	for _, group := range f.Patterns {
		ok := false
		for _, p := range group.Patterns {
			if p.matches(norm) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("pattern facet failed")
		}
	}
	return nil
}

func applyPatternsBytes(f facetSet, norm []byte) error {
	for _, group := range f.Patterns {
		ok := false
		for _, p := range group.Patterns {
			if p.matchesBytes(norm) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("pattern facet failed")
		}
	}
	return nil
}

func applyStringEnumeration[T byteText](f facetSet, norm T) error {
	for _, lit := range f.Enumeration {
		if byteTextEqual(lit.Canonical, norm) {
			return nil
		}
	}
	return fmt.Errorf("enumeration facet failed")
}

func applyPatternAndEnumeration(f facetSet, norm, canon string, actual actualValue) error {
	if err := applyPatterns(f, norm); err != nil {
		return err
	}
	if len(f.Enumeration) != 0 {
		for _, lit := range f.Enumeration {
			if actualEqualsLiteral(actual, canon, lit) {
				return nil
			}
		}
		return fmt.Errorf("enumeration facet failed")
	}
	return nil
}

func actualEqualsLiteral(actual actualValue, canon string, lit compiledLiteral) bool {
	if !actual.Valid || !lit.Actual.Valid || actual.Kind != lit.Actual.Kind {
		return lit.Canonical == canon
	}
	switch actual.Kind {
	case primBoolean:
		return actual.Boolean == lit.Actual.Boolean
	case primDecimal:
		return compareDecimalValues(actual.Decimal, lit.Actual.Decimal) == 0
	case primFloat, primDouble:
		return equalXSDFloat(actual.Float, lit.Actual.Float)
	case primDuration:
		return equalXSDDuration(actual.Duration, lit.Actual.Duration)
	case primDate, primDateTime:
		return actual.Temporal.hasTZ == lit.Actual.Temporal.hasTZ &&
			compareXSDDateTimePoint(actual.Temporal.instant, lit.Actual.Temporal.instant) == 0
	case primTime:
		return actual.Time.hasTZ == lit.Actual.Time.hasTZ && compareXSDTime(actual.Time, lit.Actual.Time) == 0
	case primGYearMonth, primGYear, primGMonthDay, primGDay, primGMonth:
		return equalXSDGValue(actual.G, lit.Actual.G)
	default:
		return lit.Canonical == canon
	}
}

func facetCanonical(l *compiledLiteral) string {
	return l.Canonical
}

func facetLexical(l *compiledLiteral) string {
	return l.Lexical
}

func applyDecimalBounds(f facetSet, value decimalValue) error {
	if f.MinInclusive != nil && compareDecimalValues(value, literalDecimal(f.MinInclusive)) < 0 {
		return fmt.Errorf("minInclusive facet failed")
	}
	if f.MaxInclusive != nil && compareDecimalValues(value, literalDecimal(f.MaxInclusive)) > 0 {
		return fmt.Errorf("maxInclusive facet failed")
	}
	if f.MinExclusive != nil && compareDecimalValues(value, literalDecimal(f.MinExclusive)) <= 0 {
		return fmt.Errorf("minExclusive facet failed")
	}
	if f.MaxExclusive != nil && compareDecimalValues(value, literalDecimal(f.MaxExclusive)) >= 0 {
		return fmt.Errorf("maxExclusive facet failed")
	}
	return nil
}

func literalDecimal(l *compiledLiteral) decimalValue {
	if l == nil {
		return decimalValue{}
	}
	if dec, ok := actualDecimalLiteral(l); ok {
		return dec
	}
	dec, err := parseDecimalValue(l.Canonical)
	if err == nil {
		return dec
	}
	return decimalValue{Canonical: l.Canonical, IntegerCanonical: l.Canonical}
}

func actualDecimalLiteral(l *compiledLiteral) (decimalValue, bool) {
	if l.Actual.Valid && l.Actual.Kind == primDecimal {
		return l.Actual.Decimal, true
	}
	return decimalValue{}, false
}
