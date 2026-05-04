package xsd

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

type qnameResolver func(string) (string, bool)

func validateSimpleValue(rt *runtimeSchema, id simpleTypeID, lexical string, resolve qnameResolver) (string, error) {
	return validateSimpleValueMode(rt, id, lexical, resolve, true)
}

func validateSimpleValueMode(rt *runtimeSchema, id simpleTypeID, lexical string, resolve qnameResolver, needCanonical bool) (string, error) {
	if id == noSimpleType {
		return lexical, nil
	}
	st := rt.SimpleTypes[id]
	if st.Missing {
		return "", fmt.Errorf("missing type")
	}
	if st.Variety == varietyList {
		return validateListValue(rt, st, lexical, resolve, needCanonical)
	}
	norm := normalizeWhitespace(lexical, st.Whitespace)
	switch st.Variety {
	case varietyUnion:
		return validateUnionValue(rt, st, norm, resolve, needCanonical)
	default:
		return validateAtomicValue(rt, id, st, norm, resolve, needCanonical)
	}
}

func validateAtomicValue(rt *runtimeSchema, id simpleTypeID, st simpleType, norm string, resolve qnameResolver, needCanonical bool) (string, error) {
	if st.Base != noSimpleType && st.Base != id && st.Base != rt.Builtin.AnySimpleType {
		if _, err := validateSimpleValueMode(rt, st.Base, norm, resolve, false); err != nil {
			return "", err
		}
	}
	canon, err := validatePrimitive(rt, st, norm, resolve, needCanonical || st.Facets.needsCanonical())
	if err != nil {
		return "", err
	}
	if err := validateBuiltinDerived(rt, st, norm); err != nil {
		return "", err
	}
	if err := applyFacets(st, norm, canon, false); err != nil {
		return "", err
	}
	return canon, nil
}

func validateListValue(rt *runtimeSchema, st simpleType, lexical string, resolve qnameResolver, needCanonical bool) (string, error) {
	needStrings := needCanonical || st.Facets.needsLexical() || st.Facets.needsCanonical()
	var canon strings.Builder
	var norm strings.Builder
	count := uint32(0)
	if err := forEachListItem(lexical, func(item string) error {
		itemCanon, err := validateSimpleValueMode(rt, st.ListItem, item, resolve, needStrings)
		if err != nil {
			return err
		}
		if needStrings {
			if count > 0 {
				canon.WriteByte(' ')
				norm.WriteByte(' ')
			}
			canon.WriteString(itemCanon)
			norm.WriteString(item)
		}
		count++
		return nil
	}); err != nil {
		return "", err
	}
	c := ""
	n := ""
	if needStrings {
		c = canon.String()
		n = norm.String()
	}
	if err := applyLengthFacets(st.Facets, count); err != nil {
		return "", err
	}
	if needStrings {
		if err := applyPatternAndEnumeration(st.Facets, n, c); err != nil {
			return "", err
		}
	}
	return c, nil
}

func forEachListItem(lexical string, fn func(string) error) error {
	start := -1
	for i := 0; i < len(lexical); i++ {
		if isXMLWhitespaceByte(lexical[i]) {
			if start >= 0 {
				if err := fn(lexical[start:i]); err != nil {
					return err
				}
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		return fn(lexical[start:])
	}
	return nil
}

func validateUnionValue(rt *runtimeSchema, st simpleType, norm string, resolve qnameResolver, needCanonical bool) (string, error) {
	var last error
	needMemberCanon := needCanonical || st.Facets.needsCanonical()
	for _, member := range st.Union {
		canon, err := validateSimpleValueMode(rt, member, norm, resolve, needMemberCanon)
		if err == nil {
			if facetErr := applyPatternAndEnumeration(st.Facets, norm, canon); facetErr != nil {
				return "", facetErr
			}
			return canon, nil
		}
		last = err
	}
	if last != nil {
		return "", last
	}
	return "", fmt.Errorf("value does not match any union member")
}

func validatePrimitive(rt *runtimeSchema, st simpleType, norm string, resolve qnameResolver, needCanonical bool) (string, error) {
	switch st.Primitive {
	case primString:
		return norm, nil
	case primAnyURI:
		return validateAnyURIPrimitive(norm)
	case primBoolean:
		return validateBooleanPrimitive(norm)
	case primDecimal:
		dec, err := parseDecimal(norm)
		if err != nil {
			return "", err
		}
		return dec.Canonical, nil
	case primFloat:
		return validateFloatPrimitive(norm, 32, needCanonical)
	case primDouble:
		return validateFloatPrimitive(norm, 64, needCanonical)
	case primDuration:
		return validateDurationPrimitive(norm)
	case primDate:
		return validateDatePrimitive(norm)
	case primDateTime:
		canon, err := parseXSDDateTimeCanonical(norm)
		if err != nil {
			return "", err
		}
		return canon, nil
	case primTime:
		canon, err := parseXSDTimeCanonical(norm)
		if err != nil {
			return "", err
		}
		return canon, nil
	case primGYearMonth:
		return validateGYearMonthPrimitive(norm)
	case primGYear:
		return validateGYearPrimitive(norm)
	case primGMonthDay:
		return validateGMonthDayPrimitive(norm)
	case primGDay:
		return validateGDayPrimitive(norm)
	case primGMonth:
		return validateGMonthPrimitive(norm)
	case primHexBinary:
		return validateHexBinaryPrimitive(norm)
	case primBase64Binary:
		return validateBase64BinaryPrimitive(norm)
	case primQName:
		return validateQNamePrimitive(norm, resolve)
	case primNotation:
		return validateNotationPrimitive(rt, norm, resolve)
	default:
		return norm, nil
	}
}

func validateAnyURIPrimitive(norm string) (string, error) {
	if !isAnyURI(norm) {
		return "", fmt.Errorf("invalid anyURI")
	}
	return norm, nil
}

func validateBooleanPrimitive(norm string) (string, error) {
	switch norm {
	case "true", "1":
		return "true", nil
	case "false", "0":
		return "false", nil
	default:
		return "", fmt.Errorf("invalid boolean")
	}
}

func validateFloatPrimitive(norm string, bitSize int, needCanonical bool) (string, error) {
	if !needCanonical {
		_, err := parseXSDFloat(norm, bitSize)
		return "", err
	}
	return parseFloatCanonical(norm, bitSize)
}

func validateDurationPrimitive(norm string) (string, error) {
	if err := parseXSDDuration(norm); err != nil {
		return "", err
	}
	return norm, nil
}

func validateDatePrimitive(norm string) (string, error) {
	_, err := parseXSDDate(norm)
	if err != nil {
		return "", err
	}
	return norm, nil
}

func validateGYearMonthPrimitive(norm string) (string, error) {
	if err := parseXSDGYearMonth(norm); err != nil {
		return "", err
	}
	return norm, nil
}

func validateGYearPrimitive(norm string) (string, error) {
	if err := parseXSDGYear(norm); err != nil {
		return "", err
	}
	return norm, nil
}

func validateGMonthDayPrimitive(norm string) (string, error) {
	if err := parseXSDGMonthDay(norm); err != nil {
		return "", err
	}
	return norm, nil
}

func validateGDayPrimitive(norm string) (string, error) {
	if err := parseXSDGDay(norm); err != nil {
		return "", err
	}
	return norm, nil
}

func validateGMonthPrimitive(norm string) (string, error) {
	if err := parseXSDGMonth(norm); err != nil {
		return "", err
	}
	return norm, nil
}

func validateHexBinaryPrimitive(norm string) (string, error) {
	if _, err := hex.DecodeString(norm); err != nil {
		return "", fmt.Errorf("invalid hexBinary")
	}
	return strings.ToUpper(norm), nil
}

func validateBase64BinaryPrimitive(norm string) (string, error) {
	decoded, err := decodeXSDBase64(norm)
	if err != nil {
		return "", fmt.Errorf("invalid base64Binary")
	}
	return base64.StdEncoding.EncodeToString(decoded), nil
}

func validateQNamePrimitive(norm string, resolve qnameResolver) (string, error) {
	if resolve == nil {
		if !isNCName(norm) {
			return "", fmt.Errorf("invalid QName")
		}
		return norm, nil
	}
	canon, ok := resolve(norm)
	if !ok {
		return "", fmt.Errorf("unresolved QName")
	}
	return canon, nil
}

func validateNotationPrimitive(rt *runtimeSchema, norm string, resolve qnameResolver) (string, error) {
	if resolve == nil {
		if !isNCName(norm) {
			return "", fmt.Errorf("invalid NOTATION")
		}
		if rt.Notations[norm] {
			return norm, nil
		}
		return "", fmt.Errorf("undeclared notation")
	}
	canon, ok := resolve(norm)
	if !ok {
		return "", fmt.Errorf("unresolved NOTATION")
	}
	if !rt.Notations[canon] {
		return "", fmt.Errorf("undeclared notation")
	}
	return canon, nil
}

func isAnyURI(s string) bool {
	if strings.HasPrefix(s, ":") || strings.HasSuffix(s, ":") {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '^':
			return false
		case '%':
			if i+2 >= len(s) || !isHexDigit(s[i+1]) || !isHexDigit(s[i+2]) {
				return false
			}
			i += 2
		}
	}
	return true
}

func isHexDigit(b byte) bool {
	return '0' <= b && b <= '9' || 'a' <= b && b <= 'f' || 'A' <= b && b <= 'F'
}

func decodeXSDBase64(s string) ([]byte, error) {
	s = removeXMLWhitespace(s)
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(s) == 0 {
		return decoded, nil
	}
	pads := 0
	for pads < len(s) && s[len(s)-1-pads] == '=' {
		pads++
	}
	if pads > 2 {
		return nil, fmt.Errorf("invalid base64Binary")
	}
	if strings.Contains(s[:len(s)-pads], "=") {
		return nil, fmt.Errorf("invalid base64Binary")
	}
	switch pads {
	case 1:
		v, ok := base64Value(s[len(s)-2])
		if !ok || v&0x03 != 0 {
			return nil, fmt.Errorf("invalid base64Binary")
		}
	case 2:
		v, ok := base64Value(s[len(s)-3])
		if !ok || v&0x0f != 0 {
			return nil, fmt.Errorf("invalid base64Binary")
		}
	}
	return decoded, nil
}

func base64Value(b byte) (byte, bool) {
	switch {
	case 'A' <= b && b <= 'Z':
		return b - 'A', true
	case 'a' <= b && b <= 'z':
		return b - 'a' + 26, true
	case '0' <= b && b <= '9':
		return b - '0' + 52, true
	case b == '+':
		return 62, true
	case b == '/':
		return 63, true
	default:
		return 0, false
	}
}
