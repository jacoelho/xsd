package xsd

import (
	"encoding/hex"
	"fmt"
	"unicode/utf8"
)

func validateBuiltinDerived(rt *runtimeSchema, st simpleType, norm string, actual actualValue) error {
	if rt.Names.Namespace(st.Name.Namespace) != xsdNamespaceURI {
		return nil
	}
	local := rt.Names.Local(st.Name.Local)
	switch local {
	case "integer", "nonPositiveInteger", "negativeInteger", "nonNegativeInteger", "positiveInteger",
		"long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		dec := actual.Decimal
		if !actual.Valid || actual.Kind != primDecimal {
			var err error
			dec, err = parseDecimal(norm)
			if err != nil {
				return err
			}
		}
		if !dec.IntegerLexical {
			return fmt.Errorf("invalid integer")
		}
		return validateIntegerRange(local, dec.IntegerCanonical)
	case "Name":
		if !isXMLName(norm) {
			return fmt.Errorf("invalid Name")
		}
	case "NCName", "ID", "IDREF", "ENTITY":
		if !isNCName(norm) {
			return fmt.Errorf("invalid NCName")
		}
		if local == "ENTITY" {
			return unsupported(ErrUnsupportedEntity, "ENTITY requires DTD entity declarations, which are not supported")
		}
	case "NMTOKEN":
		if !isNMTOKEN(norm) {
			return fmt.Errorf("invalid NMTOKEN")
		}
	case "language":
		if !isLanguage(norm) {
			return fmt.Errorf("invalid language")
		}
	}
	return nil
}

func validateIntegerRange(local, value string) error {
	cmp := func(v string) int {
		return compareCanonicalDecimal(value, v)
	}
	switch local {
	case "nonPositiveInteger":
		if cmp("0") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "negativeInteger":
		if cmp("0") >= 0 {
			return fmt.Errorf("integer out of range")
		}
	case "nonNegativeInteger":
		if cmp("0") < 0 {
			return fmt.Errorf("integer out of range")
		}
	case "positiveInteger":
		if cmp("0") <= 0 {
			return fmt.Errorf("integer out of range")
		}
	case "long":
		if cmp("-9223372036854775808") < 0 || cmp("9223372036854775807") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "int":
		if cmp("-2147483648") < 0 || cmp("2147483647") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "short":
		if cmp("-32768") < 0 || cmp("32767") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "byte":
		if cmp("-128") < 0 || cmp("127") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "unsignedLong":
		if cmp("0") < 0 || cmp("18446744073709551615") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "unsignedInt":
		if cmp("0") < 0 || cmp("4294967295") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "unsignedShort":
		if cmp("0") < 0 || cmp("65535") > 0 {
			return fmt.Errorf("integer out of range")
		}
	case "unsignedByte":
		if cmp("0") < 0 || cmp("255") > 0 {
			return fmt.Errorf("integer out of range")
		}
	}
	return nil
}

func applyFacets(st simpleType, norm, canon string, actual actualValue, list bool) error {
	if st.Facets.empty() {
		return nil
	}
	if list {
		return applyPatternAndEnumeration(st.Facets, norm, canon, actualValue{})
	}
	if err := applyAtomicFacets(st, norm, actual); err != nil {
		return err
	}
	return applyPatternAndEnumeration(st.Facets, norm, canon, actual)
}

func applyAtomicFacets(st simpleType, norm string, actual actualValue) error {
	if err := applyAtomicLengthFacets(st, norm, actual); err != nil {
		return err
	}
	if st.Primitive == primDecimal {
		dec := actual.Decimal
		if !actual.Valid || actual.Kind != primDecimal {
			var err error
			dec, err = parseDecimal(norm)
			if err != nil {
				return err
			}
		}
		return applyDecimalFacets(st.Facets, dec)
	}
	return applyPrimitiveBounds(st.Primitive, st.Facets, norm, actual)
}

func applyAtomicLengthFacets(st simpleType, norm string, actual actualValue) error {
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

func applyPrimitiveBounds(kind primitiveKind, f facetSet, norm string, actual actualValue) error {
	switch kind {
	case primFloat, primDouble:
		return applyFloatBounds(kind, f, norm, actual)
	case primDuration:
		return applyDurationBounds(f, norm, actual)
	case primGDay:
		return applyGDayBounds(f, norm, actual)
	case primGMonthDay:
		return applyGMonthDayBounds(f, norm, actual)
	case primGMonth:
		return applyGMonthBounds(f, norm, actual)
	case primGYearMonth:
		return applyGYearMonthBounds(f, norm, actual)
	case primGYear:
		return applyGYearBounds(f, norm, actual)
	case primDate, primDateTime, primTime:
		return applyTemporalBounds(kind, f, norm, actual)
	default:
		return nil
	}
}

func atomicLength(kind primitiveKind, norm string) (uint32, error) {
	switch kind {
	case primHexBinary:
		decoded, err := hex.DecodeString(norm)
		if err != nil {
			return 0, fmt.Errorf("invalid hexBinary")
		}
		return uint32(len(decoded)), nil
	case primBase64Binary:
		decoded, err := decodeXSDBase64(norm)
		if err != nil {
			return 0, fmt.Errorf("invalid base64Binary")
		}
		return uint32(len(decoded)), nil
	default:
		return uint32(utf8.RuneCountInString(norm)), nil
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

func applyPatternAndEnumeration(f facetSet, norm, canon string, actual actualValue) error {
	for _, group := range f.Patterns {
		ok := false
		for _, p := range group.Patterns {
			if p.RE.MatchString(norm) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("pattern facet failed")
		}
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
		return compareCanonicalDecimal(actual.Decimal.Canonical, lit.Actual.Decimal.Canonical) == 0
	case primFloat, primDouble:
		return actual.Float == lit.Actual.Float || actual.Float != actual.Float && lit.Actual.Float != lit.Actual.Float
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

func applyDecimalBounds(f facetSet, value decimalValue) error {
	if f.MinInclusive != nil && compareCanonicalDecimal(value.Canonical, literalDecimal(f.MinInclusive).Canonical) < 0 {
		return fmt.Errorf("minInclusive facet failed")
	}
	if f.MaxInclusive != nil && compareCanonicalDecimal(value.Canonical, literalDecimal(f.MaxInclusive).Canonical) > 0 {
		return fmt.Errorf("maxInclusive facet failed")
	}
	if f.MinExclusive != nil && compareCanonicalDecimal(value.Canonical, literalDecimal(f.MinExclusive).Canonical) <= 0 {
		return fmt.Errorf("minExclusive facet failed")
	}
	if f.MaxExclusive != nil && compareCanonicalDecimal(value.Canonical, literalDecimal(f.MaxExclusive).Canonical) >= 0 {
		return fmt.Errorf("maxExclusive facet failed")
	}
	return nil
}

func literalDecimal(l *compiledLiteral) decimalValue {
	if l != nil && l.Actual.Valid && l.Actual.Kind == primDecimal {
		return l.Actual.Decimal
	}
	if l == nil {
		return decimalValue{}
	}
	dec, err := parseDecimal(l.Canonical)
	if err != nil {
		return decimalValue{Canonical: l.Canonical, IntegerCanonical: l.Canonical}
	}
	return dec
}
