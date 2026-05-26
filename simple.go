package xsd

import (
	"encoding/base64"
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

type primitiveNeed uint8

const (
	primitiveNeedCanonical primitiveNeed = 1 << iota
	primitiveNeedLength
)

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

type simpleValueNeed uint8

const (
	simpleNeedCanonical simpleValueNeed = 1 << iota
	simpleNeedIdentity
)

func (n simpleValueNeed) has(need simpleValueNeed) bool {
	return n&need != 0
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
	case varietyUnion:
	}
	return simpleIdentityNone
}

func validateSimpleValueMode(rt *runtimeSchema, id simpleTypeID, lexical string, resolve qnameResolver, needs simpleValueNeed) (simpleValue, error) {
	if id == noSimpleType {
		return simpleValue{Canonical: lexical, Type: noSimpleType}, nil
	}
	st := &rt.SimpleTypes[id]
	if st.Missing {
		return simpleValue{}, fmt.Errorf("missing type")
	}
	if st.Variety == varietyList {
		return validateListValue(rt, id, *st, lexical, resolve, needs)
	}
	norm := normalizeWhitespace(lexical, st.Whitespace)
	if st.Variety == varietyUnion {
		return validateUnionValue(rt, *st, norm, resolve, needs)
	}
	return validateAtomicValue(rt, id, st, norm, resolve, needs)
}

func validateAtomicValue(rt *runtimeSchema, id simpleTypeID, st *simpleType, norm string, resolve qnameResolver, valueNeeds simpleValueNeed) (simpleValue, error) {
	identity := st.Identity
	if !rt.SimpleIdentitiesClassified {
		identity = computeSimpleValueIdentity(rt, id)
	}
	if id == rt.Builtin.Int && valueNeeds == 0 && identity == simpleIdentityNone {
		if err := validateBuiltinIntNoCanonical(norm); err != nil {
			return simpleValue{}, err
		}
		return simpleValue{Type: id}, nil
	}
	if canValidateDecimalNoOutputFast(st, identity, valueNeeds) {
		if err := validateDecimalNoOutput(st.Facets, norm); err != nil {
			return simpleValue{}, err
		}
		return simpleValue{Type: id}, nil
	}
	if canValidateStringPatternsNoOutputFast(st, identity, valueNeeds) {
		if err := applyPatterns(st.Facets, norm); err != nil {
			return simpleValue{}, err
		}
		return simpleValue{Type: id}, nil
	}
	if canValidateStringEnumerationNoOutputFast(st, identity, valueNeeds) {
		if err := applyStringEnumeration(st.Facets, norm); err != nil {
			return simpleValue{}, err
		}
		return simpleValue{Type: id}, nil
	}
	if canAcceptStringValueFast(st, identity) {
		v := simpleValue{Type: id}
		if valueNeeds.has(simpleNeedCanonical) {
			v.Canonical = norm
		}
		if valueNeeds.has(simpleNeedIdentity) {
			v.Identity = simpleIdentityKey(primString, norm)
		}
		return v, nil
	}
	var primitiveNeeds primitiveNeed
	if valueNeeds.has(simpleNeedCanonical) ||
		identity != simpleIdentityNone ||
		st.Primitive != primDecimal && (st.Facets.needsCanonical() || valueNeeds.has(simpleNeedIdentity)) {
		primitiveNeeds |= primitiveNeedCanonical
	}
	if st.Facets.needsLength() {
		primitiveNeeds |= primitiveNeedLength
	}
	parsed, err := validatePrimitiveActual(rt, st, norm, resolve, primitiveNeeds)
	if err != nil {
		return simpleValue{}, err
	}
	canon := parsed.Canonical
	if err := validateAtomicBuiltin(st, norm, parsed.Actual); err != nil {
		return simpleValue{}, err
	}
	if st.Builtin == builtinValidationInteger && primitiveNeeds&primitiveNeedCanonical != 0 {
		canon = parsed.Actual.Decimal.integerCanonical()
	}
	if !st.Facets.empty() {
		if err := applyAtomicFacets(st, norm, parsed.Actual); err != nil {
			return simpleValue{}, err
		}
		if err := applyPatternAndEnumeration(st.Facets, norm, canon, parsed.Actual); err != nil {
			return simpleValue{}, err
		}
	}
	v := simpleValue{Canonical: canon, Type: id}
	if valueNeeds.has(simpleNeedIdentity) {
		v.Identity = primitiveIdentityKey(st.Primitive, canon, parsed.Actual)
	}
	switch identity {
	case simpleIdentityID:
		v.IDs = canon
	case simpleIdentityIDREF:
		v.IDRefs = canon
	case simpleIdentityNone, simpleIdentityIDREFList:
	}
	return v, nil
}

func canValidateDecimalNoOutputFast(st *simpleType, identity simpleIdentityKind, needs simpleValueNeed) bool {
	return st.Primitive == primDecimal &&
		st.Builtin == builtinValidationNone &&
		identity == simpleIdentityNone &&
		needs == 0 &&
		len(st.Facets.Patterns) == 0 &&
		len(st.Facets.Enumeration) == 0
}

func canValidateStringPatternsNoOutputFast(st *simpleType, identity simpleIdentityKind, needs simpleValueNeed) bool {
	return st.Primitive == primString &&
		st.Builtin == builtinValidationNone &&
		identity == simpleIdentityNone &&
		needs == 0 &&
		st.Facets.onlyPatterns()
}

func canValidateStringEnumerationNoOutputFast(st *simpleType, identity simpleIdentityKind, needs simpleValueNeed) bool {
	return st.Primitive == primString &&
		st.Builtin == builtinValidationNone &&
		identity == simpleIdentityNone &&
		needs == 0 &&
		st.Facets.onlyEnumeration()
}

func validateRawSimpleContentFast(rt *runtimeSchema, id simpleTypeID, raw []byte) (bool, error) {
	if id == noSimpleType || !validUint32Index(uint32(id), len(rt.SimpleTypes)) {
		return false, nil
	}
	st := &rt.SimpleTypes[id]
	if st.Missing {
		return false, nil
	}
	if st.Variety == varietyList {
		return validateRawListValueFast(rt, st, raw)
	}
	if st.Variety == varietyUnion {
		return validateRawUnionValueFast(rt, st, raw)
	}
	if st.Variety != varietyAtomic {
		return false, nil
	}
	identity := st.Identity
	if !rt.SimpleIdentitiesClassified {
		identity = computeSimpleValueIdentity(rt, id)
	}
	if canAcceptStringValueFast(st, identity) {
		return true, nil
	}
	if canValidateStringPatternsNoOutputFast(st, identity, 0) {
		if st.Whitespace == whitespacePreserve || !hasXMLWhitespaceBytes(raw) {
			return true, applyPatternsBytes(st.Facets, raw)
		}
	}
	if canValidateStringEnumerationNoOutputFast(st, identity, 0) {
		if st.Whitespace == whitespacePreserve || !hasXMLWhitespaceBytes(raw) {
			return true, applyStringEnumerationBytes(st.Facets, raw)
		}
	}
	if hasXMLWhitespaceBytes(raw) {
		return false, nil
	}
	if id == rt.Builtin.Int && identity == simpleIdentityNone {
		return true, validateBuiltinIntNoCanonicalBytes(raw)
	}
	if canValidateBooleanNoOutputFast(st, identity) {
		return true, validateBooleanNoCanonicalBytes(raw)
	}
	if canValidateDateNoOutputFast(st, identity) {
		return validateDateNoOutputBytesFast(raw)
	}
	if canValidateDecimalNoOutputFast(st, identity, 0) {
		return validateDecimalNoOutputBytesFast(st.Facets, raw)
	}
	return false, nil
}

func validateRawListValueFast(rt *runtimeSchema, st *simpleType, raw []byte) (bool, error) {
	if st.Identity != simpleIdentityNone || !st.Facets.empty() {
		return false, nil
	}
	if computeSimpleValueIdentity(rt, st.ListItem) != simpleIdentityNone {
		return false, nil
	}
	if !canValidateRawNMTOKENListItem(rt, st.ListItem) {
		return false, nil
	}
	return true, validateNMTOKENListBytes(raw)
}

func canValidateRawNMTOKENListItem(rt *runtimeSchema, id simpleTypeID) bool {
	if id == noSimpleType || !validUint32Index(uint32(id), len(rt.SimpleTypes)) {
		return false
	}
	st := &rt.SimpleTypes[id]
	if st.Missing || st.Variety != varietyAtomic {
		return false
	}
	identity := st.Identity
	if !rt.SimpleIdentitiesClassified {
		identity = computeSimpleValueIdentity(rt, id)
	}
	return st.Builtin == builtinValidationNMTOKEN &&
		identity == simpleIdentityNone &&
		st.Facets.empty()
}

func validateNMTOKENListBytes(raw []byte) error {
	for len(raw) > 0 {
		for len(raw) > 0 && isXMLWhitespaceByte(raw[0]) {
			raw = raw[1:]
		}
		if len(raw) == 0 {
			return nil
		}
		end := 0
		for end < len(raw) && !isXMLWhitespaceByte(raw[end]) {
			end++
		}
		if !isNMTOKENBytes(raw[:end]) {
			return fmt.Errorf("invalid NMTOKEN")
		}
		raw = raw[end:]
	}
	return nil
}

func validateRawUnionValueFast(rt *runtimeSchema, st *simpleType, raw []byte) (bool, error) {
	if st.Identity != simpleIdentityNone || !st.Facets.empty() || hasXMLWhitespaceBytes(raw) {
		return false, nil
	}
	for _, member := range st.Union {
		if canValidateRawUnionBooleanMember(rt, member) {
			if validBooleanNoCanonicalBytes(raw) {
				return true, nil
			}
			continue
		}
		if computeSimpleValueIdentity(rt, member) != simpleIdentityNone {
			return false, nil
		}
		ok, err := validateRawSimpleContentFast(rt, member, raw)
		if !ok {
			return false, nil
		}
		if err == nil {
			return true, nil
		}
	}
	return true, fmt.Errorf("value does not match any union member")
}

func canValidateRawUnionBooleanMember(rt *runtimeSchema, id simpleTypeID) bool {
	if id == noSimpleType || !validUint32Index(uint32(id), len(rt.SimpleTypes)) {
		return false
	}
	st := &rt.SimpleTypes[id]
	if st.Missing || st.Variety != varietyAtomic {
		return false
	}
	identity := st.Identity
	if !rt.SimpleIdentitiesClassified {
		identity = computeSimpleValueIdentity(rt, id)
	}
	return canValidateBooleanNoOutputFast(st, identity)
}

func canAcceptStringValueFast(st *simpleType, identity simpleIdentityKind) bool {
	return st.Primitive == primString &&
		st.Builtin == builtinValidationNone &&
		identity == simpleIdentityNone &&
		st.Facets.empty()
}

func canValidateBooleanNoOutputFast(st *simpleType, identity simpleIdentityKind) bool {
	return st.Primitive == primBoolean &&
		st.Builtin == builtinValidationNone &&
		identity == simpleIdentityNone &&
		st.Facets.empty()
}

func canValidateDateNoOutputFast(st *simpleType, identity simpleIdentityKind) bool {
	return st.Primitive == primDate &&
		st.Builtin == builtinValidationNone &&
		identity == simpleIdentityNone &&
		st.Facets.empty()
}

func validateAtomicBuiltin(st *simpleType, norm string, actual actualValue) error {
	if st.Builtin != builtinValidationInteger {
		return validateBuiltinDerived(st.Builtin, norm, actual)
	}
	if !actual.Valid || actual.Kind != primDecimal {
		return validateBuiltinDerived(st.Builtin, norm, actual)
	}
	if !actual.Decimal.IntegerLexical {
		return fmt.Errorf("invalid integer")
	}
	return nil
}

func validateListValue(rt *runtimeSchema, id simpleTypeID, st simpleType, lexical string, resolve qnameResolver, needs simpleValueNeed) (simpleValue, error) {
	identity := st.Identity
	if !rt.SimpleIdentitiesClassified {
		identity = computeSimpleValueIdentity(rt, id)
	}
	needStrings := needs.has(simpleNeedCanonical) || needs.has(simpleNeedIdentity) || st.Facets.needsLexical() || st.Facets.needsCanonical() || identity != simpleIdentityNone
	itemNeeds := simpleValueNeed(0)
	if needStrings {
		itemNeeds = simpleNeedCanonical
	}
	v := simpleValue{Type: id}
	var canon strings.Builder
	var norm strings.Builder
	var idrefs strings.Builder
	count := uint32(0)
	for item := range xmlFieldsSeq(lexical) {
		itemValue, err := validateSimpleValueMode(rt, st.ListItem, item, resolve, itemNeeds)
		if err != nil {
			return simpleValue{}, err
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
	if needs.has(simpleNeedIdentity) {
		v.Identity = simpleIdentityKey(primString, v.Canonical)
	}
	return v, nil
}

func validateUnionValue(rt *runtimeSchema, st simpleType, norm string, resolve qnameResolver, needs simpleValueNeed) (simpleValue, error) {
	needMemberCanon := needs.has(simpleNeedCanonical) || needs.has(simpleNeedIdentity) || st.Facets.needsCanonical() || st.Identity != simpleIdentityNone
	memberNeeds := needs
	if needMemberCanon {
		memberNeeds |= simpleNeedCanonical
	}
	var unsupportedErr error
	for _, member := range st.Union {
		value, err := validateSimpleValueMode(rt, member, norm, resolve, memberNeeds)
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

func validatePrimitiveActual(rt *runtimeSchema, st *simpleType, norm string, resolve qnameResolver, needs primitiveNeed) (primitiveActual, error) {
	actual := actualValue{Kind: st.Primitive, Valid: true}
	needCanonical := needs&primitiveNeedCanonical != 0
	needLength := needs&primitiveNeedLength != 0
	switch st.Primitive {
	case primString:
		if needLength {
			length, err := checkedUint32(utf8.RuneCountInString(norm), "string length exceeds uint32 limit")
			if err != nil {
				return primitiveActual{}, err
			}
			actual.Length = length
		}
		return primitiveActual{Canonical: norm, Actual: actual}, nil
	case primAnyURI:
		if !isAnyURI(norm) {
			return primitiveActual{}, fmt.Errorf("invalid anyURI")
		}
		if needLength {
			length, err := checkedUint32(utf8.RuneCountInString(norm), "anyURI length exceeds uint32 limit")
			if err != nil {
				return primitiveActual{}, err
			}
			actual.Length = length
		}
		return primitiveActual{Canonical: norm, Actual: actual}, nil
	case primBoolean:
		canon, value, err := parseBooleanPrimitive(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		actual.Boolean = value
		return primitiveActual{Canonical: canon, Actual: actual}, nil
	case primDecimal:
		mode := decimalValueOnly
		if needCanonical {
			mode = decimalWithCanonical
		}
		dec, err := parseDecimalMode(norm, mode)
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
		return parseTemporalPrimitiveActual(st.Primitive, norm, needCanonical)
	case primHexBinary, primBase64Binary:
		return parseBinaryPrimitiveActual(st.Primitive, norm, needs)
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

func parseTemporalPrimitiveActual(kind primitiveKind, norm string, needCanonical bool) (primitiveActual, error) {
	switch kind {
	case primDate:
		return parseDatePrimitiveActual(norm, needCanonical)
	case primDateTime:
		return parseDateTimePrimitiveActual(norm, needCanonical)
	case primTime:
		return parseTimePrimitiveActual(norm, needCanonical)
	case primGYearMonth, primGYear, primGMonthDay, primGDay, primGMonth:
		return parseGPrimitiveActual(kind, norm, needCanonical)
	default:
		return primitiveActual{}, fmt.Errorf("invalid temporal primitive")
	}
}

func parseDatePrimitiveActual(norm string, needCanonical bool) (primitiveActual, error) {
	value, err := parseXSDDateValue(norm)
	if err != nil {
		return primitiveActual{}, err
	}
	actual := actualValue{Kind: primDate, Valid: true, Temporal: xsdTemporalValue{instant: value.point, hasTZ: value.hasTZ}}
	canon := ""
	if needCanonical {
		canon = formatXSDDate(value)
	}
	return primitiveActual{Canonical: canon, Actual: actual}, nil
}

func parseDateTimePrimitiveActual(norm string, needCanonical bool) (primitiveActual, error) {
	value, err := parseXSDDateTimeValue(norm)
	if err != nil {
		return primitiveActual{}, err
	}
	actual := actualValue{Kind: primDateTime, Valid: true, Temporal: xsdTemporalValue(value)}
	canon := ""
	if needCanonical {
		canon = formatXSDDateTime(value.instant, value.hasTZ)
	}
	return primitiveActual{Canonical: canon, Actual: actual}, nil
}

func parseTimePrimitiveActual(norm string, needCanonical bool) (primitiveActual, error) {
	value, err := parseXSDTimeValue(norm)
	if err != nil {
		return primitiveActual{}, err
	}
	actual := actualValue{Kind: primTime, Valid: true, Time: value}
	canon := ""
	if needCanonical {
		canon = formatXSDTime(value)
	}
	return primitiveActual{Canonical: canon, Actual: actual}, nil
}

func parseGPrimitiveActual(kind primitiveKind, norm string, needCanonical bool) (primitiveActual, error) {
	var value xsdGValue
	canon := ""
	switch kind {
	case primGYearMonth:
		v, err := parseXSDGYearMonthValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		value = v
		if needCanonical {
			canon = formatXSDGYearMonth(value)
		}
	case primGYear:
		v, err := parseXSDGYearValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		value = v
		if needCanonical {
			canon = formatXSDGYear(value)
		}
	case primGMonthDay:
		v, err := parseXSDGMonthDayValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		value = v
		if needCanonical {
			canon = formatXSDGMonthDay(value)
		}
	case primGDay:
		v, err := parseXSDGDayValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		value = v
		if needCanonical {
			canon = formatXSDGDay(value)
		}
	case primGMonth:
		v, err := parseXSDGMonthValue(norm)
		if err != nil {
			return primitiveActual{}, err
		}
		value = v
		if needCanonical {
			canon = formatXSDGMonth(value)
		}
	default:
		return primitiveActual{}, fmt.Errorf("invalid temporal primitive")
	}
	return primitiveActual{
		Canonical: canon,
		Actual:    actualValue{Kind: kind, Valid: true, G: value},
	}, nil
}

func parseBinaryPrimitiveActual(kind primitiveKind, norm string, needs primitiveNeed) (primitiveActual, error) {
	actual := actualValue{Kind: kind, Valid: true}
	needCanonical := needs&primitiveNeedCanonical != 0
	needLength := needs&primitiveNeedLength != 0
	switch kind {
	case primHexBinary:
		return parseHexBinaryPrimitiveActual(norm, actual, needCanonical, needLength)
	case primBase64Binary:
		if needCanonical {
			decoded, err := decodeXSDBase64(norm)
			if err != nil {
				return primitiveActual{}, fmt.Errorf("invalid base64Binary")
			}
			length, err := checkedUint32(len(decoded), "base64Binary length exceeds uint32 limit")
			if err != nil {
				return primitiveActual{}, err
			}
			actual.Length = length
			return primitiveActual{Canonical: base64.StdEncoding.EncodeToString(decoded), Actual: actual}, nil
		}
		length, err := base64BinaryLength(norm)
		if err != nil {
			return primitiveActual{}, fmt.Errorf("invalid base64Binary")
		}
		if needLength {
			actual.Length = length
		}
		return primitiveActual{Canonical: norm, Actual: actual}, nil
	default:
		return primitiveActual{}, fmt.Errorf("invalid binary primitive")
	}
}

func parseHexBinaryPrimitiveActual(norm string, actual actualValue, needCanonical, needLength bool) (primitiveActual, error) {
	length, err := hexBinaryLength(norm)
	if err != nil {
		return primitiveActual{}, err
	}
	if needLength {
		actual.Length = length
	}
	canon := norm
	if needCanonical {
		canon = strings.ToUpper(norm)
	}
	return primitiveActual{Canonical: canon, Actual: actual}, nil
}

func hexBinaryLength(norm string) (uint32, error) {
	if len(norm)%2 != 0 {
		return 0, fmt.Errorf("invalid hexBinary")
	}
	for i := 0; i < len(norm); i++ {
		if !isHexDigit(norm[i]) {
			return 0, fmt.Errorf("invalid hexBinary")
		}
	}
	return checkedUint32(len(norm)/2, "hexBinary length exceeds uint32 limit")
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

func parseBooleanLexical(v string) (bool, bool) {
	switch v {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	default:
		return false, false
	}
}

func validateBooleanNoCanonicalBytes(v []byte) error {
	if validBooleanNoCanonicalBytes(v) {
		return nil
	}
	return fmt.Errorf("invalid boolean")
}

func validBooleanNoCanonicalBytes(v []byte) bool {
	switch len(v) {
	case 1:
		if v[0] == '0' || v[0] == '1' {
			return true
		}
	case 4:
		if v[0] == 't' && v[1] == 'r' && v[2] == 'u' && v[3] == 'e' {
			return true
		}
	case 5:
		if v[0] == 'f' && v[1] == 'a' && v[2] == 'l' && v[3] == 's' && v[4] == 'e' {
			return true
		}
	}
	return false
}

func parseBooleanPrimitive(norm string) (string, bool, error) {
	value, ok := parseBooleanLexical(norm)
	if !ok {
		return "", false, fmt.Errorf("invalid boolean")
	}
	if value {
		return "true", true, nil
	}
	return "false", false, nil
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
	if _, err := scanBase64Binary(s); err != nil {
		return nil, err
	}
	s = removeXMLWhitespace(s)
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func base64BinaryLength(s string) (uint32, error) {
	scan, err := scanBase64Binary(s)
	if err != nil {
		return 0, err
	}
	return scan.length()
}

type base64BinaryScan struct {
	cleanLen int
	pads     int
}

func (s base64BinaryScan) length() (uint32, error) {
	return checkedUint32(s.cleanLen/4*3-s.pads, "base64Binary length exceeds uint32 limit")
}

func scanBase64Binary(s string) (base64BinaryScan, error) {
	var scan base64BinaryScan
	var lastData byte
	for i := 0; i < len(s); i++ {
		b := s[i]
		if isXMLWhitespaceByte(b) {
			continue
		}
		if b == '=' {
			scan.pads++
			scan.cleanLen++
			continue
		}
		if scan.pads > 0 {
			return base64BinaryScan{}, fmt.Errorf("invalid base64Binary")
		}
		if _, ok := base64Value(b); !ok {
			return base64BinaryScan{}, fmt.Errorf("invalid base64Binary")
		}
		lastData = b
		scan.cleanLen++
	}
	if scan.cleanLen%4 != 0 || scan.pads > 2 {
		return base64BinaryScan{}, fmt.Errorf("invalid base64Binary")
	}
	switch scan.pads {
	case 1:
		v, ok := base64Value(lastData)
		if !ok || v&0x03 != 0 {
			return base64BinaryScan{}, fmt.Errorf("invalid base64Binary")
		}
	case 2:
		v, ok := base64Value(lastData)
		if !ok || v&0x0f != 0 {
			return base64BinaryScan{}, fmt.Errorf("invalid base64Binary")
		}
	}
	return scan, nil
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
