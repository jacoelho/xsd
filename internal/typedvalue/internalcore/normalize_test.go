package internalcore

import (
	"strings"
	"testing"
)

func TestNormalizeValueRejectsNilType(t *testing.T) {
	t.Parallel()

	_, err := NormalizeValue("x", nil, NormalizeOps{
		IsNilType:         func(any) bool { return true },
		IsBuiltinType:     func(any) bool { return false },
		TypeNameLocal:     func(any) string { return "" },
		PrimitiveType:     func(any) any { return nil },
		WhiteSpaceMode:    func(any) int { return 0 },
		ApplyWhiteSpace:   func(lexical string, _ int) string { return lexical },
		TrimXMLWhitespace: func(lexical string) string { return lexical },
	})
	if err == nil {
		t.Fatalf("expected error for nil type")
	}
}

func TestNormalizeValueTemporalTrimsAfterWhitespace(t *testing.T) {
	t.Parallel()

	type fakeType struct {
		builtin bool
		local   string
	}
	typ := fakeType{builtin: true, local: "dateTime"}

	got, err := NormalizeValue("  2001-01-01T00:00:00  ", typ, NormalizeOps{
		IsNilType:      func(any) bool { return false },
		IsBuiltinType:  func(v any) bool { return v.(fakeType).builtin },
		TypeNameLocal:  func(v any) string { return v.(fakeType).local },
		PrimitiveType:  func(any) any { return nil },
		WhiteSpaceMode: func(any) int { return 0 },
		ApplyWhiteSpace: func(lexical string, _ int) string {
			return strings.ReplaceAll(lexical, "\t", " ")
		},
		TrimXMLWhitespace: strings.TrimSpace,
	})
	if err != nil {
		t.Fatalf("NormalizeValue() error = %v", err)
	}
	if got != "2001-01-01T00:00:00" {
		t.Fatalf("NormalizeValue() = %q, want %q", got, "2001-01-01T00:00:00")
	}
}

func TestNormalizeValueNonTemporalKeepsOuterWhitespace(t *testing.T) {
	t.Parallel()

	type fakeType struct {
		builtin bool
		local   string
	}
	typ := fakeType{builtin: true, local: "string"}

	got, err := NormalizeValue("  a  ", typ, NormalizeOps{
		IsNilType:      func(any) bool { return false },
		IsBuiltinType:  func(v any) bool { return v.(fakeType).builtin },
		TypeNameLocal:  func(v any) string { return v.(fakeType).local },
		PrimitiveType:  func(any) any { return nil },
		WhiteSpaceMode: func(any) int { return 0 },
		ApplyWhiteSpace: func(lexical string, _ int) string {
			return lexical
		},
		TrimXMLWhitespace: strings.TrimSpace,
	})
	if err != nil {
		t.Fatalf("NormalizeValue() error = %v", err)
	}
	if got != "  a  " {
		t.Fatalf("NormalizeValue() = %q, want %q", got, "  a  ")
	}
}
