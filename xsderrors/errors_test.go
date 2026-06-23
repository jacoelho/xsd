package xsderrors

import (
	"fmt"
	"testing"
)

func TestIsUnsupportedRecursesThroughWrappedDiagnostics(t *testing.T) {
	err := SchemaParse(
		CodeSchemaXML,
		1,
		2,
		"invalid schema XML",
		Unsupported(CodeUnsupportedRegex, "unsupported regex"),
	)
	wrapped := fmt.Errorf("outer: %w", err)
	if !IsUnsupported(wrapped) {
		t.Fatalf("IsUnsupported(%v) = false", wrapped)
	}
}

func TestIsUnsupportedRecursesThroughWrappedAggregates(t *testing.T) {
	err := fmt.Errorf("outer: %w", Errors{
		SchemaCompile(CodeSchemaReference, "missing type"),
		SchemaParse(
			CodeSchemaXML,
			1,
			2,
			"invalid schema XML",
			Unsupported(CodeUnsupportedRegex, "unsupported regex"),
		),
	})
	if !IsUnsupported(err) {
		t.Fatalf("IsUnsupported(%v) = false", err)
	}
}

func TestIsUnsupportedIgnoresTypedNilDiagnostics(t *testing.T) {
	var xerr *Error
	if IsUnsupported(xerr) {
		t.Fatal("IsUnsupported(typed nil *Error) = true")
	}
	if IsUnsupported(Errors{xerr}) {
		t.Fatal("IsUnsupported(Errors{typed nil *Error}) = true")
	}
}
