package xsderrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestDiagnosticAccessorsAndLocationAreImmutable(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	original := requireDiagnostic(t, SchemaParse(CodeSchemaXML, "invalid schema XML", cause))
	located := requireDiagnostic(t, WithLocation("schema.xsd", 3, 4, original))
	if located == original {
		t.Fatal("WithLocation() mutated the original diagnostic")
	}
	if original.Path() != "" || original.Line() != 0 || original.Column() != 0 {
		t.Fatalf("original location changed to %q %d:%d", original.Path(), original.Line(), original.Column())
	}
	if located.Category() != CategorySchemaParse || located.Code() != CodeSchemaXML ||
		located.Path() != "schema.xsd" || located.Line() != 3 || located.Column() != 4 ||
		located.Message() != "invalid schema XML" || !errors.Is(located.Cause(), cause) {
		t.Fatalf("located diagnostic = %#v", located)
	}
	if !errors.Is(located, cause) {
		t.Fatal("located diagnostic lost its cause")
	}
}

func TestWithLocationPreservesWrappersAndExistingFields(t *testing.T) {
	t.Parallel()

	original := WithLocation("original.xsd", 1, 2, SchemaCompile(CodeSchemaReference, "missing type"))
	if got := WithLocation("replacement.xsd", 3, 4, original); got != original { //nolint:errorlint // Identity is the no-op contract.
		t.Fatal("WithLocation() replaced an already located diagnostic")
	}
	wrapper := fmt.Errorf("context: %w", original)
	if got := WithLocation("replacement.xsd", 3, 4, wrapper); got != wrapper { //nolint:errorlint // Wrappers are never discarded.
		t.Fatal("WithLocation() replaced a wrapper")
	}
}

func TestErrorsOwnChildrenAndFilterNil(t *testing.T) {
	t.Parallel()

	target := Validation(CodeValidationType, "bad type", nil)
	unsupported := Unsupported(CodeUnsupportedRegex, "unsupported regex", nil)
	input := []error{nil, target, unsupported, nil}
	aggregate, ok := NewErrors(input...).(Errors) //nolint:errorlint // Constructor returns this exact immutable aggregate.
	if !ok {
		t.Fatalf("NewErrors() type = %T, want Errors", NewErrors(input...))
	}
	input[1] = nil
	children := aggregate.Unwrap()
	children[0] = nil
	first := aggregate.At(0)
	if aggregate.Len() != 2 || first != target { //nolint:errorlint // Exact ownership is under test.
		t.Fatalf("aggregate changed through an input/output slice: len=%d first=%v", aggregate.Len(), first)
	}
	if got := aggregate.Error(); got != "2 validation errors: "+target.Error() {
		t.Fatalf("Error() = %q", got)
	}
	if !errors.Is(aggregate, target) || !IsUnsupported(aggregate) {
		t.Fatal("aggregate traversal did not reach its children")
	}
}

func TestNewErrorsCollapsesEmptyAndSingletonInputs(t *testing.T) {
	t.Parallel()

	var typedNil *Error
	if got := NewErrors(nil, typedNil); got != nil {
		t.Fatalf("NewErrors(nil) = %#v", got)
	}
	target := Validation(CodeValidationType, "bad type", nil)
	if got := NewErrors(nil, target, typedNil); got != target { //nolint:errorlint // Singleton identity is the contract.
		t.Fatalf("NewErrors(singleton) = %#v", got)
	}
	if got := Flatten(typedNil); got != nil {
		t.Fatalf("Flatten(typed nil) = %#v", got)
	}
	if IsUnsupported(typedNil) {
		t.Fatal("IsUnsupported(typed nil) = true")
	}
}

func TestTypedNilErrorsAreAbsent(t *testing.T) {
	t.Parallel()

	var typedNil *Errors
	var err error = typedNil
	if got := NewErrors(err); got != nil {
		t.Fatalf("NewErrors(typed nil *Errors) = %T, want nil", got)
	}
	if got := Flatten(err); got != nil {
		t.Fatalf("Flatten(typed nil *Errors) = %#v, want nil", got)
	}
	if IsUnsupported(err) {
		t.Fatal("IsUnsupported(typed nil *Errors) = true")
	}
	if got := WithLocation("schema.xsd", 1, 1, err); got != nil {
		t.Fatalf("WithLocation(typed nil *Errors) = %T, want nil", got)
	}

	diagnostic := Validation(CodeValidationType, "bad type", typedNil)
	if got := diagnostic.Error(); got != "validation.type: bad type" {
		t.Fatalf("diagnostic with typed nil cause = %q, want cause omitted", got)
	}
}

func TestTypedNilErrorsAreSkippedThroughWrappersAndJoins(t *testing.T) {
	t.Parallel()

	var typedNil *Errors
	var typedNilError error = typedNil
	unsupported := Unsupported(CodeUnsupportedRegex, "unsupported regex", nil)
	wrappedNil := fmt.Errorf("wrapped: %w", typedNilError)
	if IsUnsupported(wrappedNil) {
		t.Fatal("IsUnsupported(wrapper around typed nil *Errors) = true")
	}
	if flat := Flatten(wrappedNil); len(flat) != 1 || flat[0] != wrappedNil { //nolint:errorlint // A non-diagnostic wrapper remains a leaf.
		t.Fatalf("Flatten(wrapper around typed nil *Errors) = %#v, want the wrapper leaf", flat)
	}

	for i, err := range []error{
		errors.Join(typedNilError, unsupported),
		fmt.Errorf("outer: %w", errors.Join(typedNilError, unsupported)),
	} {
		if !IsUnsupported(err) {
			t.Errorf("IsUnsupported(case %d) = false, want true", i)
		}
		flat := Flatten(err)
		if len(flat) != 1 || flat[0] != unsupported { //nolint:errorlint // Projection preserves the non-empty public diagnostic.
			t.Errorf("Flatten(case %d) = %#v, want the unsupported diagnostic only", i, flat)
		}
	}
}

type customDiagnosticError struct {
	diagnostic *Error
}

func (customDiagnosticError) Error() string { return "custom diagnostic wrapper" }

func (e customDiagnosticError) As(target any) bool {
	diagnostic, ok := target.(**Error)
	if !ok {
		return false
	}
	*diagnostic = e.diagnostic
	return true
}

func TestIsUnsupportedPreservesCustomAs(t *testing.T) {
	t.Parallel()

	diagnostic, ok := errors.AsType[*Error](Unsupported(CodeUnsupportedRegex, "unsupported regex", nil))
	if !ok {
		t.Fatal("Unsupported() did not return a diagnostic")
	}
	if !IsUnsupported(customDiagnosticError{diagnostic: diagnostic}) {
		t.Fatal("IsUnsupported(custom As wrapper) = false, want true")
	}
}

func TestFlattenUsesCanonicalAggregateProjection(t *testing.T) {
	t.Parallel()

	first := Validation(CodeValidationType, "bad type", nil)
	second := Unsupported(CodeUnsupportedRegex, "unsupported regex", nil)
	aggregate := NewErrors(first, second)
	flat := Flatten(fmt.Errorf("outer: %w", aggregate))
	if len(flat) != 2 || flat[0] != first || flat[1] != second { //nolint:errorlint // Child identity and order are the projection contract.
		t.Fatalf("Flatten() = %#v", flat)
	}
}

func TestFlattenTraversesEveryJoinedAggregateInOrder(t *testing.T) {
	t.Parallel()

	first := Validation(CodeValidationType, "first", nil)
	second := Validation(CodeValidationType, "second", nil)
	third := Validation(CodeValidationType, "third", nil)
	fourth := Validation(CodeValidationType, "fourth", nil)
	leaf := errors.New("leaf")
	err := errors.Join(
		NewErrors(first, second),
		fmt.Errorf("wrapped aggregate: %w", NewErrors(third, fourth)),
		leaf,
	)
	flat := Flatten(err)
	want := []error{first, second, third, fourth, leaf}
	if len(flat) != len(want) {
		t.Fatalf("Flatten() = %#v, want %#v", flat, want)
	}
	for i := range want {
		if flat[i] != want[i] { //nolint:errorlint // Exact identity and order are the projection contract.
			t.Fatalf("Flatten()[%d] = %#v, want %#v", i, flat[i], want[i])
		}
	}
	flat[0] = nil
	if got := Flatten(err); got[0] != first { //nolint:errorlint // Returned storage must be owned.
		t.Fatal("Flatten() returned aliased storage")
	}
}

func TestFlattenKeepsPublicDiagnosticAsLeaf(t *testing.T) {
	t.Parallel()

	child := NewErrors(errors.New("first"), errors.New("second"))
	diagnostic := Validation(CodeValidationType, "invalid type", child)
	flat := Flatten(diagnostic)
	if len(flat) != 1 || flat[0] != diagnostic { //nolint:errorlint // Public diagnostic context is terminal.
		t.Fatalf("Flatten() = %#v, want diagnostic leaf", flat)
	}
}

func TestDiagnosticCatalogRejectsMismatchedCategoryAndCode(t *testing.T) {
	t.Parallel()

	if !ValidCategoryCode(CategoryFormat, CodeFormatLimit) || ValidCategoryCode(CategoryValidation, CodeFormatLimit) {
		t.Fatal("ValidCategoryCode() returned an invalid catalog result")
	}
	diagnostic := requireDiagnostic(t, Validation(CodeSchemaXML, "wrong category", nil))
	if diagnostic.Category() != CategoryInternal || diagnostic.Code() != CodeInternalInvariant {
		t.Fatalf("invalid constructor pair produced %s/%s", diagnostic.Category(), diagnostic.Code())
	}
}

func TestIsUnsupportedRecursesThroughDiagnosticCause(t *testing.T) {
	t.Parallel()

	err := SchemaParse(
		CodeSchemaXML,
		"invalid schema XML",
		Unsupported(CodeUnsupportedRegex, "unsupported regex", nil),
	)
	if !IsUnsupported(fmt.Errorf("outer: %w", err)) {
		t.Fatalf("IsUnsupported(%v) = false", err)
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
