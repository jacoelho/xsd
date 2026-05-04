package xsd

import (
	"encoding/hex"
	"fmt"
)

func validateBuiltinDerived(rt *runtimeSchema, st simpleType, norm string) error {
	local := rt.Names.Local(st.Name.Local)
	switch local {
	case "integer", "nonPositiveInteger", "negativeInteger", "nonNegativeInteger", "positiveInteger",
		"long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		dec, err := parseDecimal(norm)
		if err != nil {
			return err
		}
		if !dec.IntegerLexical {
			return fmt.Errorf("invalid integer")
		}
		return validateIntegerRange(local, dec.Canonical)
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

func applyFacets(st simpleType, norm, canon string, list bool) error {
	if st.Facets.empty() {
		return nil
	}
	if list {
		return applyPatternAndEnumeration(st.Facets, norm, canon)
	}
	if err := applyAtomicFacets(st, norm); err != nil {
		return err
	}
	return applyPatternAndEnumeration(st.Facets, norm, canon)
}

func applyAtomicFacets(st simpleType, norm string) error {
	if err := applyAtomicLengthFacets(st, norm); err != nil {
		return err
	}
	if st.Primitive == primDecimal {
		return applyDecimalFacets(st.Facets, norm)
	}
	return applyPrimitiveBounds(st.Primitive, st.Facets, norm)
}

func applyAtomicLengthFacets(st simpleType, norm string) error {
	if st.Primitive == primQName || st.Primitive == primNotation {
		return nil
	}
	length, err := atomicLength(st.Primitive, norm)
	if err != nil {
		return err
	}
	return applyLengthFacets(st.Facets, length)
}

func applyDecimalFacets(f facetSet, norm string) error {
	dec, err := parseDecimal(norm)
	if err != nil {
		return err
	}
	if f.TotalDigits != nil && dec.TotalDigits > *f.TotalDigits {
		return fmt.Errorf("totalDigits facet failed")
	}
	if f.FractionDigits != nil && dec.FractionDigits > *f.FractionDigits {
		return fmt.Errorf("fractionDigits facet failed")
	}
	return applyDecimalBounds(f, dec.Canonical)
}

func applyPrimitiveBounds(kind primitiveKind, f facetSet, norm string) error {
	switch kind {
	case primFloat, primDouble:
		return applyFloatBounds(kind, f, norm)
	case primDuration:
		return applyDurationBounds(f, norm)
	case primGDay:
		return applyGDayBounds(f, norm)
	case primGMonthDay:
		return applyGMonthDayBounds(f, norm)
	case primGMonth:
		return applyGMonthBounds(f, norm)
	case primGYearMonth:
		return applyGYearMonthBounds(f, norm)
	case primGYear:
		return applyGYearBounds(f, norm)
	case primDate, primDateTime, primTime:
		return applyTemporalBounds(kind, f, norm)
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
		return uint32(len([]rune(norm))), nil
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

func applyPatternAndEnumeration(f facetSet, norm, canon string) error {
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
			if lit.Canonical == canon {
				return nil
			}
		}
		return fmt.Errorf("enumeration facet failed")
	}
	return nil
}

func applyDecimalBounds(f facetSet, value string) error {
	if f.MinInclusive != nil && compareCanonicalDecimal(value, f.MinInclusive.Canonical) < 0 {
		return fmt.Errorf("minInclusive facet failed")
	}
	if f.MaxInclusive != nil && compareCanonicalDecimal(value, f.MaxInclusive.Canonical) > 0 {
		return fmt.Errorf("maxInclusive facet failed")
	}
	if f.MinExclusive != nil && compareCanonicalDecimal(value, f.MinExclusive.Canonical) <= 0 {
		return fmt.Errorf("minExclusive facet failed")
	}
	if f.MaxExclusive != nil && compareCanonicalDecimal(value, f.MaxExclusive.Canonical) >= 0 {
		return fmt.Errorf("maxExclusive facet failed")
	}
	return nil
}

type intValueParser func(string) (int, error)

func applyIntOrderedBounds(f facetSet, norm string, parse intValueParser) error {
	value, err := parse(norm)
	if err != nil {
		return err
	}
	cmpLit := func(l *compiledLiteral) (int, bool, error) {
		if l == nil {
			return 0, false, nil
		}
		v, err := parse(l.Canonical)
		return v, true, err
	}
	if lit, ok, err := cmpLit(f.MinInclusive); err != nil {
		return err
	} else if ok && value < lit {
		return fmt.Errorf("minInclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MaxInclusive); err != nil {
		return err
	} else if ok && value > lit {
		return fmt.Errorf("maxInclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MinExclusive); err != nil {
		return err
	} else if ok && value <= lit {
		return fmt.Errorf("minExclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MaxExclusive); err != nil {
		return err
	} else if ok && value >= lit {
		return fmt.Errorf("maxExclusive facet failed")
	}
	return nil
}

func validateIntOrderedFacetBounds(name string, f facetSet, parse intValueParser) error {
	lower, lowerExclusive, hasLower, err := intLowerBound(f, parse)
	if err != nil {
		return err
	}
	upper, upperExclusive, hasUpper, err := intUpperBound(f, parse)
	if err != nil {
		return err
	}
	if !hasLower || !hasUpper {
		return nil
	}
	if lower > upper || lower == upper && (lowerExclusive || upperExclusive) {
		return fmt.Errorf("%s lower bound cannot exceed upper bound", name)
	}
	return nil
}

func intLowerBound(f facetSet, parse intValueParser) (int, bool, bool, error) {
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parse, func(other, out int) bool { return other >= out })
}

func intUpperBound(f facetSet, parse intValueParser) (int, bool, bool, error) {
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parse, func(other, out int) bool { return other <= out })
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
