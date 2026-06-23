package runtime_test

import (
	"math"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestDecimalAndIntegerCanonicalValuesDiverge(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	rt := engineRuntime(t, engine)
	decimal, err := rt.ValidateSimpleValueRuntimeBoundaryForTest(rt.Builtin.Decimal, "5", nil, runtime.SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("validateSimpleValueInfo(decimal) error = %v", err)
	}
	if decimal.Canonical != "5.0" {
		t.Fatalf("decimal canonical = %q, want 5.0", decimal.Canonical)
	}
	integer, err := rt.ValidateSimpleValueRuntimeBoundaryForTest(rt.Builtin.Int, "05", nil, runtime.SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("validateSimpleValueInfo(int) error = %v", err)
	}
	if integer.Canonical != "5" {
		t.Fatalf("int canonical = %q, want 5", integer.Canonical)
	}
}

func TestBooleanPrimitiveRuntimeParserBuildsSchemaActual(t *testing.T) {
	for _, tt := range []struct {
		lexical   string
		canonical string
		value     bool
	}{
		{lexical: "1", canonical: "true", value: true},
		{lexical: "0", canonical: "false"},
	} {
		t.Run(tt.lexical, func(t *testing.T) {
			got, err := runtime.ParsePrimitiveActual(runtime.PrimitiveBoolean, tt.lexical, runtime.PrimitiveNeedCanonical)
			if err != nil {
				t.Fatalf("ParsePrimitiveActual(%q) error = %v", tt.lexical, err)
			}
			if got.Canonical != tt.canonical {
				t.Fatalf("ParsePrimitiveActual(%q) canonical = %q, want %q", tt.lexical, got.Canonical, tt.canonical)
			}
			if !got.Actual.Valid || got.Actual.Kind != runtime.PrimitiveBoolean || got.Actual.Boolean != tt.value {
				t.Fatalf("ParsePrimitiveActual(%q) actual = %+v, want boolean %v", tt.lexical, got.Actual, tt.value)
			}
		})
	}
	_, err := runtime.ParsePrimitiveActual(runtime.PrimitiveBoolean, "yes", runtime.PrimitiveNeedCanonical)
	if err == nil || err.Error() != "invalid boolean" {
		t.Fatalf("ParsePrimitiveActual() error = %v, want invalid boolean", err)
	}
}

func TestBinaryPrimitiveRuntimeParserBuildsSchemaActual(t *testing.T) {
	for _, tt := range []struct {
		name      string
		primitive runtime.PrimitiveKind
		lexical   string
		canonical string
		length    uint32
	}{
		{name: "hex", primitive: runtime.PrimitiveHexBinary, lexical: "0aff", canonical: "0AFF", length: 2},
		{name: "base64", primitive: runtime.PrimitiveBase64Binary, lexical: "A Q I =", canonical: "AQI=", length: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runtime.ParsePrimitiveActual(tt.primitive, tt.lexical, runtime.PrimitiveNeedCanonical|runtime.PrimitiveNeedLength)
			if err != nil {
				t.Fatalf("ParsePrimitiveActual(%q) error = %v", tt.lexical, err)
			}
			if got.Canonical != tt.canonical {
				t.Fatalf("ParsePrimitiveActual(%q) canonical = %q, want %q", tt.lexical, got.Canonical, tt.canonical)
			}
			if !got.Actual.Valid || got.Actual.Kind != tt.primitive || got.Actual.Length != tt.length {
				t.Fatalf("ParsePrimitiveActual(%q) actual = %+v, want length %d", tt.lexical, got.Actual, tt.length)
			}
		})
	}
	for _, tt := range []struct {
		name      string
		primitive runtime.PrimitiveKind
		lexical   string
		wantErr   string
	}{
		{name: "hex", primitive: runtime.PrimitiveHexBinary, lexical: "0g", wantErr: "invalid hexBinary"},
		{name: "base64", primitive: runtime.PrimitiveBase64Binary, lexical: "AB==", wantErr: "invalid base64Binary"},
	} {
		t.Run("invalid "+tt.name, func(t *testing.T) {
			_, err := runtime.ParsePrimitiveActual(tt.primitive, tt.lexical, runtime.PrimitiveNeedCanonical|runtime.PrimitiveNeedLength)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ParsePrimitiveActual(%q) error = %v, want %q", tt.lexical, err, tt.wantErr)
			}
		})
	}
}

func TestTextPrimitiveRuntimeParsersBuildSchemaActual(t *testing.T) {
	for _, tt := range []struct {
		name      string
		primitive runtime.PrimitiveKind
		lexical   string
		length    uint32
	}{
		{name: "string", primitive: runtime.PrimitiveString, lexical: "a\u00e9", length: 2},
		{name: "anyURI", primitive: runtime.PrimitiveAnyURI, lexical: "path", length: 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runtime.ParsePrimitiveActual(tt.primitive, tt.lexical, runtime.PrimitiveNeedCanonical|runtime.PrimitiveNeedLength)
			if err != nil {
				t.Fatalf("ParsePrimitiveActual(%q) error = %v", tt.lexical, err)
			}
			if got.Canonical != tt.lexical {
				t.Fatalf("ParsePrimitiveActual(%q) canonical = %q, want %q", tt.lexical, got.Canonical, tt.lexical)
			}
			if !got.Actual.Valid || got.Actual.Kind != tt.primitive || got.Actual.Length != tt.length {
				t.Fatalf("ParsePrimitiveActual(%q) actual = %+v, want length %d", tt.lexical, got.Actual, tt.length)
			}
		})
	}

	_, err := runtime.ParsePrimitiveActual(runtime.PrimitiveAnyURI, ":bad", runtime.PrimitiveNeedCanonical|runtime.PrimitiveNeedLength)
	if err == nil || err.Error() != "invalid anyURI" {
		t.Fatalf("ParsePrimitiveActual() error = %v, want invalid anyURI", err)
	}
}

func TestFloatPrimitiveRuntimeParserBuildsSchemaActual(t *testing.T) {
	for _, tt := range []struct {
		name      string
		primitive runtime.PrimitiveKind
		lexical   string
		canonical string
		want      func(float64) bool
	}{
		{name: "float signed zero", primitive: runtime.PrimitiveFloat, lexical: "-0", canonical: "0", want: func(v float64) bool { return v == 0 }},
		{name: "double nan", primitive: runtime.PrimitiveDouble, lexical: "NaN", canonical: "NaN", want: math.IsNaN},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runtime.ParsePrimitiveActual(tt.primitive, tt.lexical, runtime.PrimitiveNeedCanonical)
			if err != nil {
				t.Fatalf("ParsePrimitiveActual(%q) error = %v", tt.lexical, err)
			}
			if got.Canonical != tt.canonical {
				t.Fatalf("ParsePrimitiveActual(%q) canonical = %q, want %q", tt.lexical, got.Canonical, tt.canonical)
			}
			if !got.Actual.Valid || got.Actual.Kind != tt.primitive || !tt.want(got.Actual.Float) {
				t.Fatalf("ParsePrimitiveActual(%q) actual = %+v", tt.lexical, got.Actual)
			}
		})
	}

	_, err := runtime.ParsePrimitiveActual(runtime.PrimitiveDouble, "nan", runtime.PrimitiveNeedCanonical)
	if err == nil || err.Error() != "invalid float" {
		t.Fatalf("ParsePrimitiveActual() error = %v, want invalid float", err)
	}
}
