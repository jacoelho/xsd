package xsderrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorsIgnoreNilChildren(t *testing.T) {
	target := Validation(CodeValidationType, 1, 2, "/root", "bad type")
	unsupported := Unsupported(CodeUnsupportedRegex, "unsupported regex")
	tests := []struct {
		name         string
		errors       Errors
		wantText     string
		wantChildren []error
	}{
		{name: "empty", errors: Errors{}, wantText: nilErrorString},
		{name: "nil", errors: Errors{nil}, wantText: nilErrorString},
		{name: "mixed singleton", errors: Errors{nil, target, nil}, wantText: target.Error(), wantChildren: []error{target}},
		{
			name:         "mixed aggregate",
			errors:       Errors{nil, target, nil, unsupported},
			wantText:     "2 validation errors: " + target.Error(),
			wantChildren: []error{target, unsupported},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.errors.Error(); got != test.wantText {
				t.Fatalf("Error() = %q, want %q", got, test.wantText)
			}
			children := test.errors.Unwrap()
			if len(children) != len(test.wantChildren) {
				t.Fatalf("Unwrap() = %#v, want %#v", children, test.wantChildren)
			}
			for i := range children {
				if children[i] != test.wantChildren[i] { //nolint:errorlint // Exact child identity is the contract under test.
					t.Fatalf("Unwrap()[%d] = %#v, want %#v", i, children[i], test.wantChildren[i])
				}
			}
			if errors.Is(test.errors, target) != (len(test.wantChildren) > 0) {
				t.Fatalf("errors.Is(target) = %v", errors.Is(test.errors, target))
			}
		})
	}
	if !IsUnsupported(Errors{nil, unsupported, nil}) {
		t.Fatal("IsUnsupported() ignored non-nil child among nil children")
	}
}

func TestErrorsUnwrapDoesNotAllocateWithoutNilChildren(t *testing.T) {
	children := Errors{
		Validation(CodeValidationType, 1, 2, "/root", "bad type"),
		Unsupported(CodeUnsupportedRegex, "unsupported regex"),
	}
	unwrapped := children.Unwrap()
	if len(unwrapped) != len(children) || &unwrapped[0] != &children[0] {
		t.Fatal("Unwrap() copied a nil-free error list")
	}
}

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

func TestLocationDecoratorsPreserveNestedErrorTrees(t *testing.T) {
	child := SchemaCompile(CodeSchemaReference, "missing type")
	sibling := Validation(CodeValidationType, 1, 2, "/root", "bad type")
	wrapped := fmt.Errorf("context: %w", child)
	if got := WithPath("schema.xsd", wrapped); got != wrapped { //nolint:errorlint // Require wrapper identity, not only chain membership.
		t.Fatalf("WithPath(wrapped) = %v, want original wrapper", got)
	}
	if got := WithSchemaCompileLocation("schema.xsd", 3, 4, wrapped); got != wrapped { //nolint:errorlint // Require wrapper identity.
		t.Fatalf("WithSchemaCompileLocation(wrapped) = %v, want original wrapper", got)
	}

	multiple := Errors{child, sibling}
	for name, got := range map[string]error{
		"path":     WithPath("schema.xsd", multiple),
		"location": WithSchemaCompileLocation("schema.xsd", 3, 4, multiple),
	} {
		preserved, ok := got.(Errors)                                                       //nolint:errorlint // Verify the top-level aggregate is unchanged.
		if !ok || len(preserved) != 2 || preserved[0] != child || preserved[1] != sibling { //nolint:errorlint // Require child identity.
			t.Fatalf("%s decorator = %#v, want original two-error aggregate", name, got)
		}
	}
}

func TestLocationDecoratorsHandleTypedNilDiagnostics(t *testing.T) {
	var xerr *Error
	var err error = xerr
	if got := WithPath("schema.xsd", err); !isTypedNilDiagnostic(got) {
		t.Fatalf("WithPath(typed nil) = %#v, want typed nil *Error", got)
	}
	if got := WithSchemaCompileLocation("schema.xsd", 1, 2, err); !isTypedNilDiagnostic(got) {
		t.Fatalf("WithSchemaCompileLocation(typed nil) = %#v, want typed nil *Error", got)
	}
}

func TestLocationDecoratorsCloneDirectDiagnostics(t *testing.T) {
	original := requireDiagnostic(t, SchemaCompile(CodeSchemaReference, "missing type"))
	withPath := requireDiagnostic(t, WithPath("schema.xsd", original))
	if withPath == original || withPath.Path != "schema.xsd" || original.Path != "" {
		t.Fatalf("WithPath() = %#v, original %#v", withPath, original)
	}
	withLocation := requireDiagnostic(t, WithSchemaCompileLocation("schema.xsd", 3, 4, original))
	if withLocation == original || withLocation.Path != "schema.xsd" || withLocation.Line != 3 || withLocation.Column != 4 {
		t.Fatalf("WithSchemaCompileLocation() = %#v, original %#v", withLocation, original)
	}
}

func TestLocationDecoratorsPreserveIneligibleDirectDiagnostics(t *testing.T) {
	alreadyLocated := requireDiagnostic(t, SchemaCompileAt("original.xsd", 1, 2, CodeSchemaReference, "missing type"))
	alreadyPathed := requireDiagnostic(t, Validation(CodeValidationType, 1, 2, "/root", "bad type"))
	nonCompile := requireDiagnostic(t, Validation(CodeValidationType, 0, 0, "", "bad type"))
	for name, test := range map[string]struct {
		original error
		got      error
	}{
		"compile location": {alreadyLocated, WithSchemaCompileLocation("replacement.xsd", 3, 4, alreadyLocated)},
		"path":             {alreadyPathed, WithPath("replacement.xsd", alreadyPathed)},
		"category":         {nonCompile, WithSchemaCompileLocation("schema.xsd", 3, 4, nonCompile)},
	} {
		if test.got != test.original { //nolint:errorlint // Ineligible decorators must preserve exact identity.
			t.Fatalf("%s decorator replaced ineligible diagnostic: got %#v, want %#v", name, test.got, test.original)
		}
	}
}

func requireDiagnostic(t *testing.T, err error) *Error {
	t.Helper()
	xerr, ok := err.(*Error) //nolint:errorlint // Test helper requires the direct public diagnostic type.
	if ok && xerr != nil {
		return xerr
	}
	t.Fatalf("error type = %T, want non-nil *Error", err)
	return nil
}

func isTypedNilDiagnostic(err error) bool {
	xerr, ok := err.(*Error) //nolint:errorlint // Verify the exact top-level typed-nil result.
	return ok && xerr == nil
}
