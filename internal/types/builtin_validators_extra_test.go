package types

import "testing"

func TestValidateNumericRanges(t *testing.T) {
	tests := []struct {
		fn    func(string) error
		name  string
		value string
		valid bool
	}{
		{name: "long max", value: "9223372036854775807", valid: true, fn: validateLong},
		{name: "long overflow", value: "9223372036854775808", valid: false, fn: validateLong},
		{name: "short min", value: "-32768", valid: true, fn: validateShort},
		{name: "short overflow", value: "32768", valid: false, fn: validateShort},
		{name: "byte min", value: "-128", valid: true, fn: validateByte},
		{name: "byte overflow", value: "128", valid: false, fn: validateByte},
		{name: "nonNegative zero", value: "0", valid: true, fn: validateNonNegativeInteger},
		{name: "nonNegative -0", value: "-0", valid: true, fn: validateNonNegativeInteger},
		{name: "nonNegative -00", value: "-00", valid: true, fn: validateNonNegativeInteger},
		{name: "nonNegative neg", value: "-1", valid: false, fn: validateNonNegativeInteger},
		{name: "unsignedLong max", value: "18446744073709551615", valid: true, fn: validateUnsignedLong},
		{name: "unsignedLong overflow", value: "18446744073709551616", valid: false, fn: validateUnsignedLong},
		{name: "unsignedInt max", value: "4294967295", valid: true, fn: validateUnsignedInt},
		{name: "unsignedInt overflow", value: "4294967296", valid: false, fn: validateUnsignedInt},
		{name: "unsignedShort max", value: "65535", valid: true, fn: validateUnsignedShort},
		{name: "unsignedShort overflow", value: "65536", valid: false, fn: validateUnsignedShort},
		{name: "unsignedByte max", value: "255", valid: true, fn: validateUnsignedByte},
		{name: "unsignedByte overflow", value: "256", valid: false, fn: validateUnsignedByte},
		{name: "nonPositive zero", value: "0", valid: true, fn: validateNonPositiveInteger},
		{name: "nonPositive positive", value: "1", valid: false, fn: validateNonPositiveInteger},
		{name: "negative -1", value: "-1", valid: true, fn: validateNegativeInteger},
		{name: "negative zero", value: "0", valid: false, fn: validateNegativeInteger},
	}

	for _, tt := range tests {
		err := tt.fn(tt.value)
		if (err == nil) != tt.valid {
			t.Fatalf("%s: value %q error=%v, valid=%v", tt.name, tt.value, err, tt.valid)
		}
	}
}

func TestValidateStringDerivedTypes(t *testing.T) {
	if err := validateNormalizedString("line\nbreak"); err == nil {
		t.Fatalf("expected normalizedString error")
	}
	if err := validateNormalizedString("plain"); err != nil {
		t.Fatalf("unexpected normalizedString error: %v", err)
	}

	if err := validateName("1bad"); err == nil {
		t.Fatalf("expected Name start character error")
	}
	if err := validateName("goodName"); err != nil {
		t.Fatalf("unexpected Name error: %v", err)
	}

	if err := validateID("good"); err != nil {
		t.Fatalf("unexpected ID error: %v", err)
	}
	if err := validateIDREF("1bad"); err == nil {
		t.Fatalf("expected IDREF error")
	}
	if err := validateIDREFS("id1 id2"); err != nil {
		t.Fatalf("unexpected IDREFS error: %v", err)
	}
	if err := validateIDREFS("bad:ref"); err == nil {
		t.Fatalf("expected IDREFS error")
	}

	if err := validateENTITY("good"); err != nil {
		t.Fatalf("unexpected ENTITY error: %v", err)
	}
	if err := validateENTITIES("good ent2"); err != nil {
		t.Fatalf("unexpected ENTITIES error: %v", err)
	}
	if err := validateENTITIES("1bad"); err == nil {
		t.Fatalf("expected ENTITIES error")
	}

	if err := validateNMTOKEN("a.b"); err != nil {
		t.Fatalf("unexpected NMTOKEN error: %v", err)
	}
	if err := validateNMTOKEN("a b"); err == nil {
		t.Fatalf("expected NMTOKEN error")
	}
	if err := validateNMTOKENS("a b"); err != nil {
		t.Fatalf("unexpected NMTOKENS error: %v", err)
	}
	if err := validateNMTOKENS("bad@ref"); err == nil {
		t.Fatalf("expected NMTOKENS error")
	}

	if err := validateLanguage("en-US"); err != nil {
		t.Fatalf("unexpected language error: %v", err)
	}
	if err := validateLanguage("en_US"); err == nil {
		t.Fatalf("expected language error")
	}
}

func TestValidateDateTimeFamily(t *testing.T) {
	if err := validateAnyType("anything"); err != nil {
		t.Fatalf("unexpected anyType error: %v", err)
	}
	if err := validateAnySimpleType("anything"); err != nil {
		t.Fatalf("unexpected anySimpleType error: %v", err)
	}
	if err := validateDuration("P1DT2H"); err != nil {
		t.Fatalf("unexpected duration error: %v", err)
	}
	if err := validateDuration("P"); err == nil {
		t.Fatalf("expected duration error")
	}
	if _, err := ParseXSDDuration("PT1H"); err != nil {
		t.Fatalf("unexpected ParseXSDDuration error: %v", err)
	}
	if _, err := ParseXSDDuration(""); err == nil {
		t.Fatalf("expected ParseXSDDuration error")
	}

	main, tz := splitTimezone("2024-01-01Z")
	if main != "2024-01-01" || tz != "Z" {
		t.Fatalf("splitTimezone mismatch: %q %q", main, tz)
	}
	main, tz = splitTimezone("2024-01-01+02:00")
	if main != "2024-01-01" || tz != "+02:00" {
		t.Fatalf("splitTimezone mismatch: %q %q", main, tz)
	}

	if _, ok := parseFixedDigits("2024", 0, 4); !ok {
		t.Fatalf("expected parseFixedDigits to succeed")
	}
	if _, _, _, ok := parseDateParts("2024-02-29"); !ok {
		t.Fatalf("expected parseDateParts to succeed")
	}
	if _, _, _, ok := parseDateParts("2024-2-29"); ok {
		t.Fatalf("expected parseDateParts to fail")
	}
	if _, _, _, _, ok := parseTimeParts("12:34:56.789"); !ok {
		t.Fatalf("expected parseTimeParts to succeed")
	}
	if _, _, _, _, ok := parseTimeParts("12:34"); ok {
		t.Fatalf("expected parseTimeParts to fail")
	}

	if err := validateTimezoneOffset("Z"); err != nil {
		t.Fatalf("unexpected timezone error: %v", err)
	}
	if err := validateTimezoneOffset("+14:00"); err != nil {
		t.Fatalf("unexpected timezone error: %v", err)
	}
	if err := validateTimezoneOffset("+14:01"); err == nil {
		t.Fatalf("expected timezone range error")
	}

	if !isValidDate(2024, 2, 29) {
		t.Fatalf("expected leap day to be valid")
	}
	if isValidDate(2023, 2, 29) {
		t.Fatalf("expected non-leap day to be invalid")
	}

	if err := validateDateTime("2024-02-29T12:34:56Z"); err != nil {
		t.Fatalf("unexpected dateTime error: %v", err)
	}
	if err := validateDateTime("0000-01-01T00:00:00Z"); err == nil {
		t.Fatalf("expected year zero error")
	}
	if err := validateDate("2024-02-29"); err != nil {
		t.Fatalf("unexpected date error: %v", err)
	}
	if err := validateDate("2024-02-30"); err == nil {
		t.Fatalf("expected invalid date error")
	}
	if err := validateTime("23:59:59"); err != nil {
		t.Fatalf("unexpected time error: %v", err)
	}
	if err := validateTime("24:00:00"); err == nil {
		t.Fatalf("expected invalid time error")
	}
	if err := validateGYear("2024"); err != nil {
		t.Fatalf("unexpected gYear error: %v", err)
	}
	if err := validateGYear("0000"); err == nil {
		t.Fatalf("expected gYear error")
	}
	if err := validateGYearMonth("2024-12"); err != nil {
		t.Fatalf("unexpected gYearMonth error: %v", err)
	}
	if err := validateGYearMonth("2024-13"); err == nil {
		t.Fatalf("expected gYearMonth error")
	}
	if err := validateGMonth("--12"); err != nil {
		t.Fatalf("unexpected gMonth error: %v", err)
	}
	if err := validateGMonth("--13"); err == nil {
		t.Fatalf("expected gMonth error")
	}
	if err := validateGMonthDay("--12-25"); err != nil {
		t.Fatalf("unexpected gMonthDay error: %v", err)
	}
	if err := validateGMonthDay("--12-32"); err == nil {
		t.Fatalf("expected gMonthDay error")
	}
	if err := validateGDay("---15"); err != nil {
		t.Fatalf("unexpected gDay error: %v", err)
	}
	if err := validateGDay("---32"); err == nil {
		t.Fatalf("expected gDay error")
	}
}

func TestValidateBinaryURIAndQName(t *testing.T) {
	if err := validateHexBinary("0A"); err != nil {
		t.Fatalf("unexpected hexBinary error: %v", err)
	}
	if err := validateHexBinary("0"); err == nil {
		t.Fatalf("expected hexBinary length error")
	}
	if err := validateBase64Binary("AQID"); err != nil {
		t.Fatalf("unexpected base64Binary error: %v", err)
	}
	if err := validateBase64Binary("%%%"); err == nil {
		t.Fatalf("expected base64Binary error")
	}
	if err := validateAnyURI("http://example.com"); err != nil {
		t.Fatalf("unexpected anyURI error: %v", err)
	}
	if err := validateAnyURI("http://ex ample.com"); err == nil {
		t.Fatalf("expected anyURI whitespace error")
	}
	if err := validateQName("p:local"); err != nil {
		t.Fatalf("unexpected QName error: %v", err)
	}
	if err := validateQName("xmlns:local"); err == nil {
		t.Fatalf("expected QName reserved prefix error")
	}
	if err := validateNOTATION("n:note"); err != nil {
		t.Fatalf("unexpected NOTATION error: %v", err)
	}
	if !isHexDigit('F') || isHexDigit('g') {
		t.Fatalf("unexpected isHexDigit result")
	}
}

func TestValidateDoubleAndDurationLexical(t *testing.T) {
	if err := validateDouble("1.5"); err != nil {
		t.Fatalf("unexpected double error: %v", err)
	}
	if err := validateDouble("bad"); err == nil {
		t.Fatalf("expected double error")
	}

	if val, err := ParseDuration("P1D"); err != nil || val != "P1D" {
		t.Fatalf("unexpected ParseDuration result: %q err=%v", val, err)
	}
	if _, err := ParseDuration("  "); err == nil {
		t.Fatalf("expected ParseDuration error")
	}
	if _, err := ParseDuration("1D"); err == nil {
		t.Fatalf("expected ParseDuration format error")
	}
}

func TestBuiltinTypeMethods(t *testing.T) {
	stringType := GetBuiltin(TypeNameString)
	if stringType == nil {
		t.Fatalf("expected builtin string type")
	}
	if err := stringType.Validate("hi"); err != nil {
		t.Fatalf("unexpected builtin validate error: %v", err)
	}
	if _, err := stringType.ParseValue("hi"); err != nil {
		t.Fatalf("unexpected ParseValue error: %v", err)
	}
	if stringType.Variety() != AtomicVariety {
		t.Fatalf("expected atomic variety")
	}
	if stringType.WhiteSpace() != WhiteSpacePreserve {
		t.Fatalf("unexpected whitespace value")
	}
	if stringType.Ordered() {
		t.Fatalf("expected string to be unordered")
	}
	if base := stringType.BaseType(); base == nil || base.Name().Local != "anySimpleType" {
		t.Fatalf("unexpected base type")
	}
	if base := stringType.ResolvedBaseType(); base == nil || base.Name().Local != "anySimpleType" {
		t.Fatalf("unexpected resolved base type")
	}
	if primitive := stringType.PrimitiveType(); primitive != stringType {
		t.Fatalf("unexpected primitive type")
	}
	if facets := stringType.FundamentalFacets(); facets == nil {
		t.Fatalf("expected fundamental facets")
	}
	if facets := stringType.FundamentalFacets(); facets == nil {
		t.Fatalf("expected cached facets")
	}

	tokenType := GetBuiltin(TypeNameToken)
	if tokenType == nil || tokenType.PrimitiveType() == nil {
		t.Fatalf("expected token primitive type")
	}
	if tokenType.PrimitiveType().Name().Local != "string" {
		t.Fatalf("unexpected token primitive type")
	}

	listType := GetBuiltin(TypeNameNMTOKENS)
	if listType.MeasureLength("a b c") != 3 {
		t.Fatalf("expected list length to count items")
	}
}
