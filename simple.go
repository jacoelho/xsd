package xsd

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

type qnameResolver func(string) (string, bool)

type simpleValue struct {
	Canonical string
	IDs       string
	IDRefs    string
	Identity  string
	Type      simpleTypeID
}

//nolint:govet // Tagged value payload; fields stay grouped by primitive kind.
type actualValue struct {
	Time     xsdTimeValue
	G        xsdGValue
	Duration xsdDurationValue
	Temporal xsdTemporalValue
	Decimal  decimalValue
	Float    float64
	Length   uint32
	Kind     primitiveKind
	Valid    bool
	Boolean  bool
}

type primitiveActual struct {
	Canonical string
	Actual    actualValue
}

func simpleIdentityKey(kind primitiveKind, canonical string) string {
	var b strings.Builder
	b.Grow(2 + len(canonical))
	b.WriteByte(byte(kind))
	b.WriteByte('\x1e')
	b.WriteString(canonical)
	return b.String()
}

func primitiveIdentityKey(kind primitiveKind, canonical string, actual actualValue) string {
	if kind == primDecimal && actual.Valid && actual.Kind == primDecimal {
		canonical = actual.Decimal.canonical()
	}
	return simpleIdentityKey(kind, canonical)
}

func validateSimpleValue(rt *runtimeSchema, id simpleTypeID, lexical string, resolve qnameResolver) (string, error) {
	v, err := validateSimpleValueInfo(rt, id, lexical, resolve)
	if err != nil {
		return "", err
	}
	return v.Canonical, nil
}

func validateSimpleValueInfo(rt *runtimeSchema, id simpleTypeID, lexical string, resolve qnameResolver) (simpleValue, error) {
	return validateSimpleValueMode(rt, id, lexical, resolve, true, false)
}

func validateSimpleValueIdentityInfo(rt *runtimeSchema, id simpleTypeID, lexical string, resolve qnameResolver) (simpleValue, error) {
	return validateSimpleValueMode(rt, id, lexical, resolve, true, true)
}

func simpleTypeIdentity(rt *runtimeSchema, id simpleTypeID, st simpleType) simpleIdentityKind {
	if rt.SimpleIdentitiesClassified {
		return st.Identity
	}
	return computeSimpleValueIdentity(rt, id)
}

func computeSimpleValueIdentity(rt *runtimeSchema, id simpleTypeID) simpleIdentityKind {
	if id == noSimpleType || !validUint32Index(uint32(id), len(rt.SimpleTypes)) {
		return simpleIdentityNone
	}
	st := rt.SimpleTypes[id]
	if st.Identity != simpleIdentityNone {
		return st.Identity
	}
	if id == rt.Builtin.ID {
		return simpleIdentityID
	}
	if id == rt.Builtin.IDREF {
		return simpleIdentityIDREF
	}
	switch st.Variety {
	case varietyAtomic:
		if st.Base != id {
			return computeSimpleValueIdentity(rt, st.Base)
		}
	case varietyList:
		if computeSimpleValueIdentity(rt, st.ListItem) == simpleIdentityIDREF {
			return simpleIdentityIDREFList
		}
	}
	return simpleIdentityNone
}

func validateSimpleValueMode(rt *runtimeSchema, id simpleTypeID, lexical string, resolve qnameResolver, needCanonical, needIdentity bool) (simpleValue, error) {
	if id == noSimpleType {
		return simpleValue{Canonical: lexical, Type: noSimpleType}, nil
	}
	st := rt.SimpleTypes[id]
	if st.Missing {
		return simpleValue{}, fmt.Errorf("missing type")
	}
	if st.Variety == varietyList {
		return validateListValue(rt, id, st, lexical, resolve, needCanonical, needIdentity)
	}
	norm := normalizeWhitespace(lexical, st.Whitespace)
	switch st.Variety {
	case varietyUnion:
		return validateUnionValue(rt, st, norm, resolve, needCanonical, needIdentity)
	default:
		return validateAtomicValue(rt, id, st, norm, resolve, needCanonical, needIdentity)
	}
}

func validateAtomicValue(rt *runtimeSchema, id simpleTypeID, st simpleType, norm string, resolve qnameResolver, needCanonical, needIdentity bool) (simpleValue, error) {
	identity := simpleTypeIdentity(rt, id, st)
	needPrimitiveCanonical := needCanonical ||
		identity != simpleIdentityNone ||
		st.Primitive != primDecimal && (st.Facets.needsCanonical() || needIdentity)
	parsed, err := validatePrimitiveActual(rt, st, norm, resolve, needPrimitiveCanonical)
	if err != nil {
		return simpleValue{}, err
	}
	canon := parsed.Canonical
	if parsed.Actual.Valid && parsed.Actual.Kind == primDecimal && simpleTypeUsesIntegerLexical(rt, id, st) {
		if !parsed.Actual.Decimal.IntegerLexical {
			return simpleValue{}, fmt.Errorf("invalid integer")
		}
		if needPrimitiveCanonical {
			canon = parsed.Actual.Decimal.integerCanonical()
		}
	}
	if err := validateBuiltinDerived(st.Builtin, norm, parsed.Actual); err != nil {
		return simpleValue{}, err
	}
	if err := applyFacets(st, norm, canon, parsed.Actual, false); err != nil {
		return simpleValue{}, err
	}
	v := simpleValue{Canonical: canon, Type: id}
	if needIdentity {
		v.Identity = primitiveIdentityKey(st.Primitive, canon, parsed.Actual)
	}
	switch identity {
	case simpleIdentityID:
		v.IDs = canon
	case simpleIdentityIDREF:
		v.IDRefs = canon
	}
	return v, nil
}

func validateListValue(rt *runtimeSchema, id simpleTypeID, st simpleType, lexical string, resolve qnameResolver, needCanonical, needIdentity bool) (simpleValue, error) {
	identity := simpleTypeIdentity(rt, id, st)
	needStrings := needCanonical || needIdentity || st.Facets.needsLexical() || st.Facets.needsCanonical() || identity != simpleIdentityNone
	v := simpleValue{Type: id}
	var canon strings.Builder
	var norm strings.Builder
	var idrefs strings.Builder
	count := uint32(0)
	if err := forEachListItem(lexical, func(item string) error {
		itemValue, err := validateSimpleValueMode(rt, st.ListItem, item, resolve, needStrings, false)
		if err != nil {
			return err
		}
		if needStrings {
			if count > 0 {
				canon.WriteByte(' ')
				norm.WriteByte(' ')
			}
			canon.WriteString(itemValue.Canonical)
			norm.WriteString(item)
		}
		if itemValue.IDRefs != "" {
			if idrefs.Len() > 0 {
				idrefs.WriteByte(' ')
			}
			idrefs.WriteString(itemValue.IDRefs)
		}
		count++
		return nil
	}); err != nil {
		return simpleValue{}, err
	}
	n := ""
	if needStrings {
		v.Canonical = canon.String()
		n = norm.String()
	}
	v.IDRefs = idrefs.String()
	if err := applyLengthFacets(st.Facets, count); err != nil {
		return simpleValue{}, err
	}
	if needStrings {
		if err := applyPatternAndEnumeration(st.Facets, n, v.Canonical, actualValue{}); err != nil {
			return simpleValue{}, err
		}
	}
	if needIdentity {
		v.Identity = simpleIdentityKey(primString, v.Canonical)
	}
	return v, nil
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

func validateUnionValue(rt *runtimeSchema, st simpleType, norm string, resolve qnameResolver, needCanonical, needIdentity bool) (simpleValue, error) {
	needMemberCanon := needCanonical || needIdentity || st.Facets.needsCanonical() || st.Identity != simpleIdentityNone
	var unsupportedErr error
	for _, member := range st.Union {
		value, err := validateSimpleValueMode(rt, member, norm, resolve, needMemberCanon, needIdentity)
		if err == nil {
			if facetErr := applyPatternAndEnumeration(st.Facets, norm, value.Canonical, actualValue{}); facetErr != nil {
				return simpleValue{}, facetErr
			}
			return value, nil
		}
		if unsupportedErr == nil && IsUnsupported(err) {
			unsupportedErr = err
		}
	}
	if unsupportedErr != nil {
		return simpleValue{}, unsupportedErr
	}
	return simpleValue{}, fmt.Errorf("value does not match any union member")
}

func validatePrimitiveActual(rt *runtimeSchema, st simpleType, norm string, resolve qnameResolver, needCanonical bool) (primitiveActual, error) {
	actual := actualValue{Kind: st.Primitive, Valid: true}
	switch st.Primitive {
	case primString:
		actual.Length = uint32(utf8.RuneCountInString(norm))
		return primitiveActual{Canonical: norm, Actual: actual}, nil
	case primAnyURI:
		if !isAnyURI(norm) {
			return primitiveActual{}, fmt.Errorf("invalid anyURI")
		}
		actual.Length = uint32(utf8.RuneCountInString(norm))
		return primitiveActual{Canonical: norm, Actual: actual}, nil
	case primBoolean:
		canon, value, err := parseBooleanPrimitive(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.Boolean = value
		return primitiveActual{Canonical: canon, Actual: actual}, nil
	case primDecimal:
		dec, err := parseDecimalMode(norm, needCanonical)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.Decimal = dec
		return primitiveActual{Canonical: dec.Canonical, Actual: actual}, nil
	case primFloat:
		return parseFloatPrimitiveActual(norm, 32, needCanonical)
	case primDouble:
		return parseFloatPrimitiveActual(norm, 64, needCanonical)
	case primDuration:
		value, err := parseXSDDurationValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.Duration = value
		return primitiveActual{Canonical: norm, Actual: actual}, nil
	case primDate, primDateTime, primTime, primGYearMonth, primGYear, primGMonthDay, primGDay, primGMonth:
		return parseTemporalPrimitiveActual(st.Primitive, norm)
	case primHexBinary, primBase64Binary:
		return parseBinaryPrimitiveActual(st.Primitive, norm)
	case primQName:
		canon, err := validateQNamePrimitive(norm, resolve)
		return primitiveActual{Canonical: canon, Actual: actual}, err
	case primNotation:
		canon, err := validateNotationPrimitive(rt, norm, resolve)
		return primitiveActual{Canonical: canon, Actual: actual}, err
	default:
		return primitiveActual{Canonical: norm, Actual: actual}, nil
	}
}

func parseTemporalPrimitiveActual(kind primitiveKind, norm string) (primitiveActual, error) {
	actual := actualValue{Kind: kind, Valid: true}
	switch kind {
	case primDate:
		value, err := parseXSDDateValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.Temporal = xsdTemporalValue{instant: value.point, hasTZ: value.hasTZ}
		return primitiveActual{Canonical: formatXSDDate(value), Actual: actual}, nil
	case primDateTime:
		value, err := parseXSDDateTimeValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.Temporal = xsdTemporalValue(value)
		return primitiveActual{Canonical: formatXSDDateTime(value.instant, value.hasTZ), Actual: actual}, nil
	case primTime:
		value, err := parseXSDTimeValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.Time = value
		return primitiveActual{Canonical: formatXSDTime(value), Actual: actual}, nil
	case primGYearMonth:
		value, err := parseXSDGYearMonthValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.G = value
		return primitiveActual{Canonical: formatXSDGYearMonth(value), Actual: actual}, nil
	case primGYear:
		value, err := parseXSDGYearValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.G = value
		return primitiveActual{Canonical: formatXSDGYear(value), Actual: actual}, nil
	case primGMonthDay:
		value, err := parseXSDGMonthDayValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.G = value
		return primitiveActual{Canonical: formatXSDGMonthDay(value), Actual: actual}, nil
	case primGDay:
		value, err := parseXSDGDayValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.G = value
		return primitiveActual{Canonical: formatXSDGDay(value), Actual: actual}, nil
	case primGMonth:
		value, err := parseXSDGMonthValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.G = value
		return primitiveActual{Canonical: formatXSDGMonth(value), Actual: actual}, nil
	default:
		return primitiveActual{}, fmt.Errorf("invalid temporal primitive")
	}
}

func parseBinaryPrimitiveActual(kind primitiveKind, norm string) (primitiveActual, error) {
	actual := actualValue{Kind: kind, Valid: true}
	switch kind {
	case primHexBinary:
		decoded, err := hex.DecodeString(norm)
		if err != nil {
			return primitiveActual{}, fmt.Errorf("invalid hexBinary")
		}
		actual.Length = uint32(len(decoded))
		return primitiveActual{Canonical: strings.ToUpper(norm), Actual: actual}, nil
	case primBase64Binary:
		decoded, err := decodeXSDBase64(norm)
		if err != nil {
			return primitiveActual{}, fmt.Errorf("invalid base64Binary")
		}
		actual.Length = uint32(len(decoded))
		return primitiveActual{Canonical: base64.StdEncoding.EncodeToString(decoded), Actual: actual}, nil
	default:
		return primitiveActual{}, fmt.Errorf("invalid binary primitive")
	}
}

func simpleTypeUsesIntegerLexical(rt *runtimeSchema, id simpleTypeID, st simpleType) bool {
	if isXSDIntegerDatatype(rt, st) {
		return true
	}
	if st.Base == noSimpleType || st.Base == id || st.Base == rt.Builtin.AnySimpleType {
		return false
	}
	if !validUint32Index(uint32(st.Base), len(rt.SimpleTypes)) {
		return false
	}
	return simpleTypeUsesIntegerLexical(rt, st.Base, rt.SimpleTypes[st.Base])
}

func isXSDIntegerDatatype(rt *runtimeSchema, st simpleType) bool {
	if rt.Names.Namespace(st.Name.Namespace) != xsdNamespaceURI {
		return false
	}
	return isXSDIntegerDatatypeName(rt.Names.Local(st.Name.Local))
}

func isXSDIntegerDatatypeName(local string) bool {
	switch local {
	case "integer", "nonPositiveInteger", "negativeInteger", "nonNegativeInteger", "positiveInteger",
		"long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		return true
	default:
		return false
	}
}

func parseFloatPrimitiveActual(norm string, bitSize int, needCanonical bool) (primitiveActual, error) {
	value, err := parseXSDFloat(norm, bitSize)
	if err != nil {
		return primitiveActual{}, err
	}
	canon := ""
	if needCanonical {
		canon = formatXSDFloatCanonical(value, bitSize)
	}
	kind := primDouble
	if bitSize == 32 {
		kind = primFloat
	}
	return primitiveActual{
		Canonical: canon,
		Actual: actualValue{
			Kind:  kind,
			Valid: true,
			Float: value,
		},
	}, nil
}

func parseBooleanPrimitive(norm string) (string, bool, error) {
	switch norm {
	case "true", "1":
		return "true", true, nil
	case "false", "0":
		return "false", false, nil
	default:
		return "", false, fmt.Errorf("invalid boolean")
	}
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
