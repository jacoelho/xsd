package typedvalue

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
)

func TestNormalize_TemporalBuiltinTrimsWhitespace(t *testing.T) {
	t.Parallel()

	dateTimeType := builtins.Get(builtins.TypeNameDateTime)
	if dateTimeType == nil {
		t.Fatal("missing builtin dateTime type")
	}

	got, err := Normalize(" \t2001-10-26T21:32:52Z\r\n", dateTimeType)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got != "2001-10-26T21:32:52Z" {
		t.Fatalf("Normalize() = %q, want %q", got, "2001-10-26T21:32:52Z")
	}
}

func TestNormalize_MatchesModelNormalizeTypeValue(t *testing.T) {
	t.Parallel()

	typ := builtins.Get(builtins.TypeNameToken)
	if typ == nil {
		t.Fatal("missing builtin token type")
	}

	in := " \ta \n b\r\n "
	got, err := Normalize(in, typ)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	want, err := model.NormalizeTypeValue(in, typ)
	if err != nil {
		t.Fatalf("NormalizeTypeValue() error = %v", err)
	}
	if got != want {
		t.Fatalf("Normalize() = %q, want %q", got, want)
	}
}

func TestParseForType_BuiltinType(t *testing.T) {
	t.Parallel()

	booleanType := builtins.Get(builtins.TypeNameBoolean)
	if booleanType == nil {
		t.Fatal("missing builtin boolean type")
	}

	typed, err := ParseForType("true", builtins.TypeNameBoolean, booleanType)
	if err != nil {
		t.Fatalf("ParseForType() error = %v", err)
	}
	value, ok := typed.Native().(bool)
	if !ok {
		t.Fatalf("Native() type = %T, want bool", typed.Native())
	}
	if !value {
		t.Fatalf("Native() = %v, want true", value)
	}
}

func TestParseForType_SimpleType(t *testing.T) {
	t.Parallel()

	decimalType, err := builtins.NewSimpleType(builtins.TypeNameDecimal)
	if err != nil {
		t.Fatalf("NewSimpleType() error = %v", err)
	}

	typed, err := ParseForType("001.50", builtins.TypeNameDecimal, decimalType)
	if err != nil {
		t.Fatalf("ParseForType() error = %v", err)
	}
	if got, want := typed.String(), "1.5"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestParseForType_UnknownTypeName(t *testing.T) {
	t.Parallel()

	stringType, err := builtins.NewSimpleType(builtins.TypeNameString)
	if err != nil {
		t.Fatalf("NewSimpleType() error = %v", err)
	}

	_, err = ParseForType("value", model.TypeName("unknown"), stringType)
	if err == nil {
		t.Fatal("expected error for unknown parser type")
	}
	if !strings.Contains(err.Error(), "no parser for type unknown") {
		t.Fatalf("error = %q, want substring %q", err.Error(), "no parser for type unknown")
	}
}
