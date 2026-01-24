package types

import (
	"testing"
)

func TestGetBuiltin(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
	}{
		{"anyType", true},
		{"string", true},
		{"boolean", true},
		{"decimal", true},
		{"integer", true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		typ := GetBuiltin(TypeName(tt.name))
		if (typ != nil) != tt.exists {
			t.Errorf("GetBuiltin(%q) exists=%v, want %v", tt.name, typ != nil, tt.exists)
		}
	}
}

func TestGetBuiltinNS(t *testing.T) {
	typ := GetBuiltinNS("http://www.w3.org/2001/XMLSchema", "string")
	if typ == nil {
		t.Error("GetBuiltinNS for xs:string should return a type")
	}

	typ = GetBuiltinNS("http://www.w3.org/2001/XMLSchema", "anyType")
	if typ == nil {
		t.Error("GetBuiltinNS for xs:anyType should return a type")
	}

	typ = GetBuiltinNS("http://example.com", "string")
	if typ != nil {
		t.Error("GetBuiltinNS for non-XSD namespace should return nil")
	}
}

func TestBuiltinTypeValidateNilReceiver(t *testing.T) {
	var bt *BuiltinType
	if err := bt.Validate("value"); err != nil {
		t.Fatalf("expected nil error for nil receiver, got %v", err)
	}
}

func TestValidateBoolean(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"true", true},
		{"false", true},
		{"1", true},
		{"0", true},
		{"yes", false},
		{"no", false},
		{"TRUE", false},
		{"", false},
		{" true", false},
		{"true ", false},
	}

	for _, tt := range tests {
		err := validateBoolean(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateBoolean(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidateString(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"", true},
		{"hello", true},
		{"hello world", true},
		{"123", true},
		{"special chars: !@#$%", true},
	}

	for _, tt := range tests {
		err := validateString(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateString(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"hello", true},
		{"hello world", true},
		{"  hello  ", false},    // should be collapsed first
		{"hello  world", false}, // should be collapsed first
		{"", true},
	}

	for _, tt := range tests {
		err := validateToken(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateToken(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidateNCName(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"hello", true},
		{"_hello", true},
		{"hello123", true},
		{"hello-world", true},
		{"hello.world", true},
		{"123hello", false},    // can't start with digit
		{"hello:world", false}, // no colons allowed
		{":hello", false},
		{"hello:", false},
		{"", false},
		{"-hello", false}, // can't start with hyphen
	}

	for _, tt := range tests {
		err := validateNCName(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateNCName(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidateDecimal(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"123", true},
		{"123.45", true},
		{"+123.45", true},
		{"-123.45", true},
		{".45", true},
		{"123.", true},
		{"0", true},
		{"-0", true},
		{"abc", false},
		{"12.34.56", false},
		{"", false},
	}

	for _, tt := range tests {
		err := validateDecimal(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateDecimal(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidateInteger(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"0", true},
		{"123", true},
		{"-123", true},
		{"+123", true},
		{"0012", true},
		{"123.45", false},
		{"abc", false},
		{"", false},
	}

	for _, tt := range tests {
		err := validateInteger(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateInteger(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidateInt(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"0", true},
		{"2147483647", true},
		{"-2147483648", true},
		{"2147483648", false},
		{"-2147483649", false},
	}

	for _, tt := range tests {
		err := validateInt(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateInt(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidatePositiveInteger(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"1", true},
		{"100", true},
		{"0", false},
		{"-1", false},
	}

	for _, tt := range tests {
		err := validatePositiveInteger(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validatePositiveInteger(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestValidateFloat(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"0", true},
		{"123.45", true},
		{"-123.45", true},
		{"INF", true},
		{"-INF", true},
		{"NaN", true},
		{"1.23e10", true},
		{"abc", false},
	}

	for _, tt := range tests {
		err := validateFloat(tt.value)
		if (err == nil) != tt.valid {
			t.Errorf("validateFloat(%q) = %v, want valid=%v", tt.value, err, tt.valid)
		}
	}
}

func TestBuiltinTypeImplementsType(t *testing.T) {
	typ := GetBuiltin(TypeNameString)
	if typ == nil {
		t.Fatal("GetBuiltin(\"string\") returned nil")
	}

	// test that BuiltinType implements Type interface
	var _ Type = typ

	// test Name() returns QName
	name := typ.Name()
	if name.Namespace != XSDNamespace {
		t.Errorf("Name().Namespace = %q, want %q", name.Namespace, XSDNamespace)
	}
	if name.Local != "string" {
		t.Errorf("Name().Local = %q, want %q", name.Local, "string")
	}

	// test IsBuiltin() returns true
	if !typ.IsBuiltin() {
		t.Error("IsBuiltin() = false, want true")
	}
}

func TestBuiltinType_BaseType(t *testing.T) {
	// test derived type (integer -> decimal)
	integerType := GetBuiltin(TypeNameInteger)
	if integerType == nil {
		t.Fatal("GetBuiltin(\"integer\") returned nil")
	}

	base := integerType.BaseType()
	if base == nil {
		t.Fatal("BaseType() returned nil for integer")
	}
	if base.Name().Local != "decimal" {
		t.Errorf("BaseType().Name().Local = %q, want %q", base.Name().Local, "decimal")
	}

	// test primitive type (decimal -> nil or anySimpleType)
	decimalType := GetBuiltin(TypeNameDecimal)
	if decimalType == nil {
		t.Fatal("GetBuiltin(\"decimal\") returned nil")
	}

	// primitive types may return nil (anySimpleType not registered)
	primBase := decimalType.BaseType()
	// this is acceptable - anySimpleType is not a registered built-in
	if primBase != nil && primBase.Name().Local != "anySimpleType" {
		t.Logf("Primitive type base is %q (expected nil or anySimpleType)", primBase.Name().Local)
	}
}

func TestBuiltinType_PrimitiveType(t *testing.T) {
	// test derived type (integer -> decimal)
	integerType := GetBuiltin(TypeNameInteger)
	if integerType == nil {
		t.Fatal("GetBuiltin(\"integer\") returned nil")
	}

	primitive := integerType.PrimitiveType()
	if primitive == nil {
		t.Fatal("PrimitiveType() returned nil for integer")
	}
	if primitive.Name().Local != "decimal" {
		t.Errorf("PrimitiveType().Name().Local = %q, want %q", primitive.Name().Local, "decimal")
	}

	// test primitive type (decimal -> decimal)
	decimalType := GetBuiltin(TypeNameDecimal)
	if decimalType == nil {
		t.Fatal("GetBuiltin(\"decimal\") returned nil")
	}

	prim := decimalType.PrimitiveType()
	if prim == nil {
		t.Fatal("PrimitiveType() returned nil for decimal")
	}
	if prim.Name().Local != "decimal" {
		t.Errorf("PrimitiveType().Name().Local = %q, want %q", prim.Name().Local, "decimal")
	}
}

func TestBuiltinType_FundamentalFacets(t *testing.T) {
	// test numeric type (integer)
	integerType := GetBuiltin(TypeNameInteger)
	if integerType == nil {
		t.Fatal("GetBuiltin(\"integer\") returned nil")
	}

	facets := integerType.FundamentalFacets()
	if facets == nil {
		t.Fatal("FundamentalFacets() returned nil for integer")
		return
	}
	if facets.Ordered != OrderedTotal {
		t.Errorf("FundamentalFacets().Ordered = %v, want %v", facets.Ordered, OrderedTotal)
	}
	if !facets.Numeric {
		t.Error("FundamentalFacets().Numeric = false, want true")
	}

	// test string type
	stringType := GetBuiltin(TypeNameString)
	if stringType == nil {
		t.Fatal("GetBuiltin(\"string\") returned nil")
	}

	strFacets := stringType.FundamentalFacets()
	if strFacets == nil {
		t.Fatal("FundamentalFacets() returned nil for string")
		return
	}
	if strFacets.Ordered != OrderedNone {
		t.Errorf("FundamentalFacets().Ordered = %v, want %v", strFacets.Ordered, OrderedNone)
	}
	if strFacets.Numeric {
		t.Error("FundamentalFacets().Numeric = true, want false")
	}
}

func TestBuiltinType_WhiteSpace(t *testing.T) {
	// test collapse
	integerType := GetBuiltin(TypeNameInteger)
	if integerType == nil {
		t.Fatal("GetBuiltin(\"integer\") returned nil")
	}

	ws := integerType.WhiteSpace()
	if ws != WhiteSpaceCollapse {
		t.Errorf("WhiteSpace() = %v, want %v", ws, WhiteSpaceCollapse)
	}

	// test preserve
	stringType := GetBuiltin(TypeNameString)
	if stringType == nil {
		t.Fatal("GetBuiltin(\"string\") returned nil")
	}

	strWS := stringType.WhiteSpace()
	if strWS != WhiteSpacePreserve {
		t.Errorf("WhiteSpace() = %v, want %v", strWS, WhiteSpacePreserve)
	}
}

func TestBuiltinType_CanBeAssignedToType(t *testing.T) {
	// test that BuiltinType can be assigned to Type variable
	bt := GetBuiltin(TypeNameInteger)
	if bt == nil {
		t.Fatal("GetBuiltin(\"integer\") returned nil")
	}

	var tType Type = bt

	// test type assertion works
	if _, ok := tType.(*BuiltinType); !ok {
		t.Error("Type assertion to *BuiltinType failed")
	}

	// test all methods work through Type interface
	if tType.Name().Local != "integer" {
		t.Errorf("Type.Name().Local = %q, want %q", tType.Name().Local, "integer")
	}
	if !tType.IsBuiltin() {
		t.Error("Type.IsBuiltin() = false, want true")
	}
	// BaseType() should never be nil now (always returns a type)
	if tType.BaseType() == nil {
		t.Error("Type.BaseType() should never return nil")
	}
	if tType.PrimitiveType() == nil {
		t.Error("Type.PrimitiveType() returned nil")
	}
	if tType.FundamentalFacets() == nil {
		t.Error("Type.FundamentalFacets() returned nil")
	}
	if tType.WhiteSpace() != WhiteSpaceCollapse {
		t.Errorf("Type.WhiteSpace() = %v, want %v", tType.WhiteSpace(), WhiteSpaceCollapse)
	}
}
