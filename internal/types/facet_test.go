package types

import "testing"

func TestPattern(t *testing.T) {
	p := &Pattern{Value: `\d{3}-\d{3}-\d{4}`}
	if err := p.ValidateSyntax(); err != nil {
		t.Fatalf("ValidateSyntax() failed: %v", err)
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"123-456-7890", true},
		{"555-123-4567", true},
		{"123-45-6789", false},
		{"abc-def-ghij", false},
	}

	for _, tt := range tests {
		tv := &StringTypedValue{Value: tt.value, Typ: nil}
		err := p.Validate(tv, nil)
		if (err == nil) != tt.valid {
			t.Errorf("Pattern.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestEnumeration(t *testing.T) {
	e := &Enumeration{Values: []string{"red", "green", "blue"}}

	tests := []struct {
		value string
		valid bool
	}{
		{"red", true},
		{"green", true},
		{"blue", true},
		{"yellow", false},
		{"", false},
	}

	for _, tt := range tests {
		tv := &StringTypedValue{Value: tt.value, Typ: nil}
		err := e.Validate(tv, nil)
		if (err == nil) != tt.valid {
			t.Errorf("Enumeration.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestLength(t *testing.T) {
	l := &Length{Value: 5}

	tests := []struct {
		value string
		valid bool
	}{
		{"hello", true},
		{"12345", true},
		{"hi", false},
		{"too long", false},
	}

	for _, tt := range tests {
		tv := &StringTypedValue{Value: tt.value, Typ: nil}
		err := l.Validate(tv, nil)
		if (err == nil) != tt.valid {
			t.Errorf("Length.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestMinLength(t *testing.T) {
	m := &MinLength{Value: 3}

	tests := []struct {
		value string
		valid bool
	}{
		{"abc", true},
		{"abcd", true},
		{"ab", false},
		{"", false},
	}

	for _, tt := range tests {
		tv := &StringTypedValue{Value: tt.value, Typ: nil}
		err := m.Validate(tv, nil)
		if (err == nil) != tt.valid {
			t.Errorf("MinLength.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestMaxLength(t *testing.T) {
	m := &MaxLength{Value: 5}

	tests := []struct {
		value string
		valid bool
	}{
		{"abc", true},
		{"12345", true},
		{"123456", false},
	}

	for _, tt := range tests {
		tv := &StringTypedValue{Value: tt.value, Typ: nil}
		err := m.Validate(tv, nil)
		if (err == nil) != tt.valid {
			t.Errorf("MaxLength.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestMinInclusive(t *testing.T) {
	baseType := &SimpleType{
		QName: QName{Local: "integer"},
		Restriction: &Restriction{
			Base: QName{Local: "integer"},
		},
	}
	baseType.MarkBuiltin()

	m, err := NewMinInclusive("10", baseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"10", true},
		{"15", true},
		{"20", true},
		{"5", false},
		{"0", false},
		{"-5", false},
	}

	for _, tt := range tests {
		intVal, err := ParseInteger(tt.value)
		if err != nil {
			t.Fatalf("ParseInteger(%q) error = %v", tt.value, err)
		}
		tv := NewIntegerValue(NewParsedValue(tt.value, intVal), baseType)

		err = m.Validate(tv, baseType)
		if (err == nil) != tt.valid {
			t.Errorf("MinInclusive.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestMaxInclusive(t *testing.T) {
	baseType := &SimpleType{
		QName: QName{Local: "integer"},
		Restriction: &Restriction{
			Base: QName{Local: "integer"},
		},
	}
	baseType.MarkBuiltin()

	m, err := NewMaxInclusive("100", baseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"100", true},
		{"50", true},
		{"0", true},
		{"150", false},
		{"200", false},
	}

	for _, tt := range tests {
		intVal, err := ParseInteger(tt.value)
		if err != nil {
			t.Fatalf("ParseInteger(%q) error = %v", tt.value, err)
		}
		tv := NewIntegerValue(NewParsedValue(tt.value, intVal), baseType)

		err = m.Validate(tv, baseType)
		if (err == nil) != tt.valid {
			t.Errorf("MaxInclusive.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestMinExclusive(t *testing.T) {
	baseType := &SimpleType{
		QName: QName{Local: "integer"},
		Restriction: &Restriction{
			Base: QName{Local: "integer"},
		},
	}
	baseType.MarkBuiltin()

	m, err := NewMinExclusive("10", baseType)
	if err != nil {
		t.Fatalf("NewMinExclusive() error = %v", err)
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"11", true},
		{"15", true},
		{"10", false},
		{"5", false},
	}

	for _, tt := range tests {
		intVal, err := ParseInteger(tt.value)
		if err != nil {
			t.Fatalf("ParseInteger(%q) error = %v", tt.value, err)
		}
		tv := NewIntegerValue(NewParsedValue(tt.value, intVal), baseType)

		err = m.Validate(tv, baseType)
		if (err == nil) != tt.valid {
			t.Errorf("MinExclusive.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestMaxExclusive(t *testing.T) {
	baseType := &SimpleType{
		QName: QName{Local: "integer"},
		Restriction: &Restriction{
			Base: QName{Local: "integer"},
		},
	}
	baseType.MarkBuiltin()

	m, err := NewMaxExclusive("100", baseType)
	if err != nil {
		t.Fatalf("NewMaxExclusive() error = %v", err)
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"99", true},
		{"50", true},
		{"100", false},
		{"150", false},
	}

	for _, tt := range tests {
		intVal, err := ParseInteger(tt.value)
		if err != nil {
			t.Fatalf("ParseInteger(%q) error = %v", tt.value, err)
		}
		tv := NewIntegerValue(NewParsedValue(tt.value, intVal), baseType)

		err = m.Validate(tv, baseType)
		if (err == nil) != tt.valid {
			t.Errorf("MaxExclusive.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestTotalDigits(t *testing.T) {
	baseType := &SimpleType{
		QName: QName{Local: "decimal"},
		Restriction: &Restriction{
			Base: QName{Local: "decimal"},
		},
	}
	baseType.MarkBuiltin()

	td := &TotalDigits{Value: 5}

	tests := []struct {
		value string
		valid bool
	}{
		{"12345", true},
		{"1234", true},
		{"123", true},
		{"123456", false},
		{"12345.67", false}, // 7 digits total
		{"12.34", true},     // 4 digits total
		{"-123", true},      // 3 digits (sign doesn't count)
		{"+123", true},      // 3 digits
	}

	for _, tt := range tests {
		tv := &StringTypedValue{Value: tt.value, Typ: baseType}
		err := td.Validate(tv, baseType)
		if (err == nil) != tt.valid {
			t.Errorf("TotalDigits.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestFractionDigits(t *testing.T) {
	baseType := &SimpleType{
		QName: QName{Local: "decimal"},
		Restriction: &Restriction{
			Base: QName{Local: "decimal"},
		},
	}
	baseType.MarkBuiltin()

	fd := &FractionDigits{Value: 2}

	tests := []struct {
		value string
		valid bool
	}{
		{"123.45", true},
		{"123.4", true},
		{"123", true},       // no fraction digits
		{"123.456", false},  // 3 fraction digits
		{"123.4567", false}, // 4 fraction digits
		{"0.12", true},
		{"0.1", true},
		{"0.123", false},
		{"1.23E4", true}, // exponent doesn't affect fraction digits
	}

	for _, tt := range tests {
		tv := &StringTypedValue{Value: tt.value, Typ: baseType}
		err := fd.Validate(tv, baseType)
		if (err == nil) != tt.valid {
			t.Errorf("FractionDigits.Validate(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

// TestLengthCalculation tests the getLength function for different types
func TestLengthCalculation(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		baseType   Type
		wantLength int
	}{
		// string types - length in characters
		{
			name:       "string type",
			value:      "hello",
			baseType:   GetBuiltin(TypeNameString),
			wantLength: 5,
		},
		{
			name:       "empty string",
			value:      "",
			baseType:   GetBuiltin(TypeNameString),
			wantLength: 0,
		},
		{
			name:       "unicode string",
			value:      "café",
			baseType:   GetBuiltin(TypeNameString),
			wantLength: 4, // é is one rune
		},

		// hexBinary - length in octets (hex chars / 2)
		{
			name:       "hexBinary 2 chars = 1 octet",
			value:      "12",
			baseType:   GetBuiltin(TypeNameHexBinary),
			wantLength: 1,
		},
		{
			name:       "hexBinary 4 chars = 2 octets",
			value:      "1234",
			baseType:   GetBuiltin(TypeNameHexBinary),
			wantLength: 2,
		},
		{
			name:       "hexBinary 6 chars = 3 octets",
			value:      "123456",
			baseType:   GetBuiltin(TypeNameHexBinary),
			wantLength: 3,
		},
		{
			name:       "hexBinary empty",
			value:      "",
			baseType:   GetBuiltin(TypeNameHexBinary),
			wantLength: 0,
		},
		{
			name:       "hexBinary 8 chars = 4 octets",
			value:      "ABCDEF01",
			baseType:   GetBuiltin(TypeNameHexBinary),
			wantLength: 4,
		},
		{
			name:       "hexBinary odd chars (invalid, fallback to char count)",
			value:      "123",
			baseType:   GetBuiltin(TypeNameHexBinary),
			wantLength: 3, // invalid hexBinary, falls back to character count
		},

		// base64Binary - length in octets (decoded bytes)
		{
			name:       "base64Binary 1 byte",
			value:      "QQ==", // "A" in base64
			baseType:   GetBuiltin(TypeNameBase64Binary),
			wantLength: 1,
		},
		{
			name:       "base64Binary 2 bytes",
			value:      "QUI=", // "AB" in base64
			baseType:   GetBuiltin(TypeNameBase64Binary),
			wantLength: 2,
		},
		{
			name:       "base64Binary 3 bytes",
			value:      "QUJD", // "ABC" in base64
			baseType:   GetBuiltin(TypeNameBase64Binary),
			wantLength: 3,
		},
		{
			name:       "base64Binary empty",
			value:      "",
			baseType:   GetBuiltin(TypeNameBase64Binary),
			wantLength: 0,
		},
		{
			name:       "base64Binary with whitespace",
			value:      "QU JD", // "ABC" with space (decodes to 3 bytes)
			baseType:   GetBuiltin(TypeNameBase64Binary),
			wantLength: 3,
		},

		// list types - length in items
		// note: List types need to be checked before primitive type check
		// for now, we'll test list types separately since they require special setup

		// nil baseType - should use character count
		{
			name:       "nil baseType",
			value:      "hello",
			baseType:   nil,
			wantLength: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test through MaxLength facet (which uses getLength internally)
			// use a maxLength that should pass for the expected length
			maxLen := &MaxLength{Value: tt.wantLength + 1}
			tv := &StringTypedValue{Value: tt.value, Typ: tt.baseType}

			// should pass (value length <= maxLength)
			err := maxLen.Validate(tv, tt.baseType)
			if err != nil {
				t.Errorf("MaxLength.Validate(%q) with max=%d failed: %v", tt.value, tt.wantLength+1, err)
			}

			// test with exact maxLength
			exactMaxLen := &MaxLength{Value: tt.wantLength}
			err = exactMaxLen.Validate(tv, tt.baseType)
			if err != nil {
				t.Errorf("MaxLength.Validate(%q) with max=%d failed: %v", tt.value, tt.wantLength, err)
			}

			// test with maxLength one less (should fail)
			if tt.wantLength > 0 {
				tooSmallMaxLen := &MaxLength{Value: tt.wantLength - 1}
				err = tooSmallMaxLen.Validate(tv, tt.baseType)
				if err == nil {
					t.Errorf("MaxLength.Validate(%q) with max=%d should fail but didn't", tt.value, tt.wantLength-1)
				}
			}

			// test Length facet with exact length
			exactLen := &Length{Value: tt.wantLength}
			err = exactLen.Validate(tv, tt.baseType)
			if err != nil {
				t.Errorf("Length.Validate(%q) with length=%d failed: %v", tt.value, tt.wantLength, err)
			}
		})
	}
}

// TestLengthFacetsWithHexBinary tests length facets specifically with hexBinary
func TestLengthFacetsWithHexBinary(t *testing.T) {
	hexBinaryType := GetBuiltin(TypeNameHexBinary)

	tests := []struct {
		name      string
		value     string
		facet     Facet
		wantValid bool
	}{
		{
			name:      "maxLength 4 with 6 hex chars (3 octets) - valid",
			value:     "123456",
			facet:     &MaxLength{Value: 4},
			wantValid: true,
		},
		{
			name:      "maxLength 2 with 6 hex chars (3 octets) - invalid",
			value:     "123456",
			facet:     &MaxLength{Value: 2},
			wantValid: false,
		},
		{
			name:      "maxLength 3 with 6 hex chars (3 octets) - valid",
			value:     "123456",
			facet:     &MaxLength{Value: 3},
			wantValid: true,
		},
		{
			name:      "length 3 with 6 hex chars (3 octets) - valid",
			value:     "123456",
			facet:     &Length{Value: 3},
			wantValid: true,
		},
		{
			name:      "length 4 with 6 hex chars (3 octets) - invalid",
			value:     "123456",
			facet:     &Length{Value: 4},
			wantValid: false,
		},
		{
			name:      "minLength 2 with 4 hex chars (2 octets) - valid",
			value:     "1234",
			facet:     &MinLength{Value: 2},
			wantValid: true,
		},
		{
			name:      "minLength 3 with 4 hex chars (2 octets) - invalid",
			value:     "1234",
			facet:     &MinLength{Value: 3},
			wantValid: false,
		},
		{
			name:      "maxLength 4 with 8 hex chars (4 octets) - valid",
			value:     "ABCDEF01",
			facet:     &MaxLength{Value: 4},
			wantValid: true,
		},
		{
			name:      "maxLength 3 with 8 hex chars (4 octets) - invalid",
			value:     "ABCDEF01",
			facet:     &MaxLength{Value: 3},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := &StringTypedValue{Value: tt.value, Typ: hexBinaryType}
			err := tt.facet.Validate(tv, hexBinaryType)
			if (err == nil) != tt.wantValid {
				t.Errorf("Facet.Validate(%q) = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
		})
	}
}

// TestLengthFacetsWithBase64Binary tests length facets specifically with base64Binary
func TestLengthFacetsWithBase64Binary(t *testing.T) {
	base64BinaryType := GetBuiltin(TypeNameBase64Binary)

	tests := []struct {
		name      string
		value     string
		facet     Facet
		wantValid bool
	}{
		{
			name:      "maxLength 4 with 3-byte base64 - valid",
			value:     "QUJD", // "ABC" = 3 bytes
			facet:     &MaxLength{Value: 4},
			wantValid: true,
		},
		{
			name:      "maxLength 2 with 3-byte base64 - invalid",
			value:     "QUJD", // "ABC" = 3 bytes
			facet:     &MaxLength{Value: 2},
			wantValid: false,
		},
		{
			name:      "length 3 with 3-byte base64 - valid",
			value:     "QUJD", // "ABC" = 3 bytes
			facet:     &Length{Value: 3},
			wantValid: true,
		},
		{
			name:      "maxLength 1 with 1-byte base64 - valid",
			value:     "QQ==", // "A" = 1 byte
			facet:     &MaxLength{Value: 1},
			wantValid: true,
		},
		{
			name:      "maxLength 0 with 1-byte base64 - invalid",
			value:     "QQ==", // "A" = 1 byte
			facet:     &MaxLength{Value: 0},
			wantValid: false,
		},
		{
			name:      "base64 with whitespace",
			value:     "QU JD", // "ABC" = 3 bytes, with space
			facet:     &MaxLength{Value: 3},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := &StringTypedValue{Value: tt.value, Typ: base64BinaryType}
			err := tt.facet.Validate(tv, base64BinaryType)
			if (err == nil) != tt.wantValid {
				t.Errorf("Facet.Validate(%q) = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
		})
	}
}

// TestLengthFacetsWithListType tests length facets with list types
func TestLengthFacetsWithListType(t *testing.T) {
	itemType := GetBuiltin(TypeNameInteger)
	listType := &SimpleType{
		QName: QName{Local: "integerList"},
		List:  &ListType{ItemType: itemType.Name()},
	}
	listType.SetVariety(ListVariety)
	listType.ItemType = itemType

	tests := []struct {
		name      string
		value     string
		facet     Facet
		wantValid bool
	}{
		{
			name:      "maxLength 5 with 3 items - valid",
			value:     "1 2 3",
			facet:     &MaxLength{Value: 5},
			wantValid: true,
		},
		{
			name:      "maxLength 2 with 3 items - invalid",
			value:     "1 2 3",
			facet:     &MaxLength{Value: 2},
			wantValid: false,
		},
		{
			name:      "length 3 with 3 items - valid",
			value:     "1 2 3",
			facet:     &Length{Value: 3},
			wantValid: true,
		},
		{
			name:      "length 4 with 3 items - invalid",
			value:     "1 2 3",
			facet:     &Length{Value: 4},
			wantValid: false,
		},
		{
			name:      "minLength 2 with 3 items - valid",
			value:     "1 2 3",
			facet:     &MinLength{Value: 2},
			wantValid: true,
		},
		{
			name:      "minLength 4 with 3 items - invalid",
			value:     "1 2 3",
			facet:     &MinLength{Value: 4},
			wantValid: false,
		},
		{
			name:      "empty list",
			value:     "",
			facet:     &Length{Value: 0},
			wantValid: true,
		},
		{
			name:      "single item",
			value:     "42",
			facet:     &Length{Value: 1},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := &StringTypedValue{Value: tt.value, Typ: listType}
			err := tt.facet.Validate(tv, listType)
			if (err == nil) != tt.wantValid {
				t.Errorf("Facet.Validate(%q) = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
		})
	}
}

// TestLengthFacetsWithQName tests length facets with QName type
// Per XSD 1.0 errata, length, minLength, and maxLength facets are ignored for QName and NOTATION
// This is because the value space length depends on namespace context, not lexical form.
func TestLengthFacetsWithQName(t *testing.T) {
	qnameType := GetBuiltin(TypeNameQName)

	tests := []struct {
		name      string
		value     string
		facet     Facet
		wantValid bool
	}{
		{
			name:      "maxLength 1 with local name only (1 char) - valid",
			value:     "a",
			facet:     &MaxLength{Value: 1},
			wantValid: true,
		},
		{
			name:      "maxLength 1 with prefix:local (local name is 1 char) - valid",
			value:     "prefix:a",
			facet:     &MaxLength{Value: 1},
			wantValid: true,
		},
		{
			name:      "maxLength 1 with prefix:localname - ignored per XSD errata",
			value:     "prefix:localname",
			facet:     &MaxLength{Value: 1},
			wantValid: true, // per XSD 1.0 errata, length facets are ignored for QName
		},
		{
			name:      "length 1 with local name only (1 char) - valid",
			value:     "a",
			facet:     &Length{Value: 1},
			wantValid: true,
		},
		{
			name:      "length 9 with prefix:localname (local name is 9 chars) - valid",
			value:     "prefix:localname",
			facet:     &Length{Value: 9},
			wantValid: true,
		},
		{
			name:      "length 1 with prefix:localname - ignored per XSD errata",
			value:     "prefix:localname",
			facet:     &Length{Value: 1},
			wantValid: true, // per XSD 1.0 errata, length facets are ignored for QName
		},
		{
			name:      "minLength 9 with prefix:localname (local name is 9 chars) - valid",
			value:     "prefix:localname",
			facet:     &MinLength{Value: 9},
			wantValid: true,
		},
		{
			name:      "minLength 10 with prefix:localname - ignored per XSD errata",
			value:     "prefix:localname",
			facet:     &MinLength{Value: 10},
			wantValid: true, // per XSD 1.0 errata, length facets are ignored for QName
		},
		// W3C test case: length facets should be ignored for QName values (per XSD 1.0 errata)
		{
			name:      "W3C: maxLength 1 with QName - ignored per errata",
			value:     "a",
			facet:     &MaxLength{Value: 1},
			wantValid: true, // ignored per XSD 1.0 errata
		},
		{
			name:      "W3C: maxLength 1 with prefixed QName - ignored per errata",
			value:     "ns:a",
			facet:     &MaxLength{Value: 1},
			wantValid: true, // ignored per XSD 1.0 errata
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := &StringTypedValue{Value: tt.value, Typ: qnameType}
			err := tt.facet.Validate(tv, qnameType)
			if (err == nil) != tt.wantValid {
				t.Errorf("Facet.Validate(%q) = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
		})
	}
}

// TestLengthFacetsWithQNameRestriction tests length facets with a SimpleType that restricts QName
// Per XSD 1.0 errata, length facets are ignored for types derived from QName.
func TestLengthFacetsWithQNameRestriction(t *testing.T) {
	// create a SimpleType that restricts QName (like in W3C tests)
	qnameBaseType := GetBuiltin(TypeNameQName)
	restrictedType := &SimpleType{
		QName: QName{Local: "restrictedQName"},
		Restriction: &Restriction{
			Base: qnameBaseType.Name(),
		},
	}
	restrictedType.ResolvedBase = qnameBaseType
	restrictedType.SetVariety(AtomicVariety)

	tests := []struct {
		name      string
		value     string
		facet     Facet
		wantValid bool
	}{
		{
			name:      "W3C case: maxLength 1 with prefixed QName - ignored per errata",
			value:     "someprefix:a",
			facet:     &MaxLength{Value: 1},
			wantValid: true, // ignored per XSD 1.0 errata
		},
		{
			name:      "maxLength 1 with single char QName - ignored per errata",
			value:     "a",
			facet:     &MaxLength{Value: 1},
			wantValid: true, // ignored per XSD 1.0 errata
		},
		{
			name:      "Restriction of QName with length facet - ignored per errata",
			value:     "prefix:a",
			facet:     &MaxLength{Value: 1},
			wantValid: true, // ignored per XSD 1.0 errata
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// verify PrimitiveType is set correctly
			primitive := restrictedType.PrimitiveType()
			if primitive == nil {
				t.Fatalf("PrimitiveType() returned nil - this would cause length to be measured incorrectly")
			}
			if primitive.Name().Local != string(TypeNameQName) {
				t.Fatalf("PrimitiveType() = %v, want QName", primitive.Name())
			}

			tv := &StringTypedValue{Value: tt.value, Typ: restrictedType}
			err := tt.facet.Validate(tv, restrictedType)
			if (err == nil) != tt.wantValid {
				t.Errorf("Facet.Validate(%q) = %v, want valid=%v", tt.value, err, tt.wantValid)
				if err != nil {
					t.Logf("Error details: %v", err)
				}
			}
		})
	}
}
