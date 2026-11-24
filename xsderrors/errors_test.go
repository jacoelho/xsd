package xsderrors

import (
	"errors"
	"fmt"
	"slices"
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

	original := WithLocation("original.xsd", 1, 2, SchemaCompile(CodeSchemaReference, "missing type", nil))
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

func TestIsConclusiveValidationTruthTable(t *testing.T) {
	t.Parallel()

	semantic := Validation(CodeValidationFacet, "invalid value", nil)
	semanticWithPlainCause := Validation(CodeValidationFacet, "invalid value", errors.New("lexical failure"))
	limit := Validation(CodeValidationLimit, "validation limit", nil)
	unsupported := Unsupported(CodeUnsupportedDTD, "DTD", nil)
	custom := customDiagnosticError{diagnostic: requireDiagnostic(t, semantic)}
	var typedNil *Errors
	var typedNilError error = typedNil
	empty := Errors{}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil},
		{name: "empty aggregate", err: empty},
		{name: "semantic", err: semantic, want: true},
		{name: "semantic with plain cause", err: semanticWithPlainCause, want: true},
		{name: "wrapped semantic", err: fmt.Errorf("document: %w", semantic), want: true},
		{name: "wrapped empty aggregate", err: fmt.Errorf("empty: %w", empty)},
		{name: "semantic and wrapped empty aggregate", err: errors.Join(semantic, fmt.Errorf("empty: %w", empty)), want: true},
		{name: "semantic with structured limit cause", err: Validation(CodeValidationFacet, "invalid value", limit)},
		{name: "semantic with joined plain causes", err: Validation(CodeValidationFacet, "invalid value", errors.Join(errors.New("first"), errors.New("second")))},
		{name: "semantic with joined semantic causes", err: Validation(CodeValidationFacet, "invalid value", errors.Join(semantic, semanticWithPlainCause)), want: true},
		{name: "semantic with wrapped empty aggregate cause", err: Validation(CodeValidationFacet, "invalid value", fmt.Errorf("empty: %w", empty)), want: true},
		{name: "semantic with empty and semantic aggregate cause", err: Validation(CodeValidationFacet, "invalid value", errors.Join(fmt.Errorf("empty: %w", empty), semantic)), want: true},
		{name: "two semantic branches", err: errors.Join(semantic, semanticWithPlainCause), want: true},
		{name: "semantic and plain branch", err: errors.Join(semantic, errors.New("resource failure"))},
		{name: "semantic and unsupported branch", err: errors.Join(semantic, unsupported)},
		{name: "wrapped joined semantic branches", err: fmt.Errorf("assessment: %w", errors.Join(semantic, semanticWithPlainCause)), want: true},
		{name: "custom As only", err: custom},
		{name: "custom As only beneath semantic", err: Validation(CodeValidationFacet, "invalid value", custom)},
		{name: "typed nil aggregate", err: typedNilError},
		{name: "semantic with typed nil aggregate", err: Validation(CodeValidationFacet, "invalid value", typedNilError), want: true},
		{name: "semantic element", err: Validation(CodeValidationElement, "invalid", nil), want: true},
		{name: "semantic attribute", err: Validation(CodeValidationAttribute, "invalid", nil), want: true},
		{name: "semantic text", err: Validation(CodeValidationText, "invalid", nil), want: true},
		{name: "semantic type", err: Validation(CodeValidationType, "invalid", nil), want: true},
		{name: "semantic content", err: Validation(CodeValidationContent, "invalid", nil), want: true},
		{name: "semantic nil", err: Validation(CodeValidationNil, "invalid", nil), want: true},
		{name: "semantic identity", err: Validation(CodeValidationIdentity, "invalid", nil), want: true},
		{name: "unknown", err: errors.New("unknown failure")},
		{name: "semantic collection", err: NewErrors(semantic, semanticWithPlainCause), want: true},
		{name: "semantic and limit collection", err: NewErrors(semantic, limit)},
		{name: "semantic root with nil cause", err: Validation(CodeValidationRoot, "missing root", nil), want: true},
		{name: "XML diagnostic", err: Validation(CodeValidationXML, "malformed XML", nil)},
		{name: "read diagnostic", err: Validation(CodeValidationRead, "read instance document", nil)},
		{name: "option diagnostic", err: Validation(CodeValidationOption, "invalid option", nil)},
		{name: "session diagnostic", err: Validation(CodeValidationSession, "session busy", nil)},
		{name: "limit diagnostic", err: limit},
		{name: "unsupported diagnostic", err: unsupported},
		{name: "internal diagnostic", err: InternalInvariant("broken invariant", nil)},
		{name: "schema diagnostic", err: SchemaCompile(CodeSchemaReference, "missing type", nil)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := IsConclusiveValidation(test.err); got != test.want {
				t.Fatalf("IsConclusiveValidation(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

func TestDiagnosticCatalogAdmitsEachCodeOnlyInItsCategories(t *testing.T) {
	t.Parallel()

	categories := []Category{
		CategorySchemaParse,
		CategorySchemaCompile,
		CategoryUnsupported,
		CategoryValidation,
		CategoryFormat,
		CategoryInternal,
		"unknown",
	}
	parseAndCompile := []Category{CategorySchemaParse, CategorySchemaCompile}
	catalog := map[Code][]Category{
		"schema.read":                 parseAndCompile,
		"schema.xml":                  {CategorySchemaParse},
		"schema.root":                 parseAndCompile,
		"schema.duplicate":            {CategorySchemaCompile},
		"schema.reference":            {CategorySchemaCompile},
		"schema.facet":                {CategorySchemaCompile},
		"schema.occurrence":           {CategorySchemaCompile},
		"schema.content_model":        {CategorySchemaCompile},
		"schema.no_sources":           {CategorySchemaCompile},
		"schema.invalid_attribute":    {CategorySchemaCompile},
		"schema.identity":             {CategorySchemaCompile},
		"schema.limit":                parseAndCompile,
		"unsupported.dtd":             {CategoryUnsupported},
		"unsupported.external_entity": {CategoryUnsupported},
		"unsupported.entity":          {CategoryUnsupported},
		"unsupported.non_utf8":        {CategoryUnsupported},
		"unsupported.xs_redefine":     {CategoryUnsupported},
		"unsupported.regex":           {CategoryUnsupported},
		"unsupported.xml_version":     {CategoryUnsupported},
		"unsupported.xsd_1_1":         {CategoryUnsupported},
		"validation.xml":              {CategoryValidation},
		"validation.read":             {CategoryValidation},
		"validation.root":             {CategoryValidation},
		"validation.element":          {CategoryValidation},
		"validation.attribute":        {CategoryValidation},
		"validation.text":             {CategoryValidation},
		"validation.type":             {CategoryValidation},
		"validation.facet":            {CategoryValidation},
		"validation.content":          {CategoryValidation},
		"validation.nil":              {CategoryValidation},
		"validation.identity":         {CategoryValidation},
		"validation.option":           {CategoryValidation},
		"validation.session":          {CategoryValidation},
		"validation.limit":            {CategoryValidation},
		"format.xml":                  {CategoryFormat},
		"format.option":               {CategoryFormat},
		"format.limit":                {CategoryFormat},
		"internal.invariant":          {CategoryInternal},
		"unknown.code":                nil,
		"":                            nil,
	}
	for code, members := range catalog {
		for _, category := range categories {
			want := slices.Contains(members, category)
			if got := ValidCategoryCode(category, code); got != want {
				t.Errorf("ValidCategoryCode(%q, %q) = %v, want %v", category, code, got, want)
			}
		}
	}
}

type causeError struct{ detail string }

func (e *causeError) Error() string { return e.detail }

func TestEveryConstructorKeepsCategoryCodeMessageAndCause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		build    func(msg string, cause error) error
		category Category
		code     Code
	}{
		{"SchemaParse", func(m string, c error) error { return SchemaParse(CodeSchemaXML, m, c) }, CategorySchemaParse, "schema.xml"},
		{"SchemaCompile", func(m string, c error) error { return SchemaCompile(CodeSchemaFacet, m, c) }, CategorySchemaCompile, "schema.facet"},
		{"Unsupported", func(m string, c error) error { return Unsupported(CodeUnsupportedRegex, m, c) }, CategoryUnsupported, "unsupported.regex"},
		{"Validation", func(m string, c error) error { return Validation(CodeValidationType, m, c) }, CategoryValidation, "validation.type"},
		{"Format", func(m string, c error) error { return Format(CodeFormatXML, m, c) }, CategoryFormat, "format.xml"},
		{"InternalInvariant", InternalInvariant, CategoryInternal, "internal.invariant"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			sentinel := errors.New("sentinel")
			cause := fmt.Errorf("reading: %w", &causeError{detail: "typed cause"})
			joined := errors.Join(sentinel, cause)
			err := test.build("what failed", joined)
			diagnostic := requireDiagnostic(t, err)
			if diagnostic.Category() != test.category || diagnostic.Code() != test.code || diagnostic.Message() != "what failed" {
				t.Fatalf("%s() = %s/%s %q, want %s/%s %q", test.name,
					diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), test.category, test.code, "what failed")
			}
			if want := string(test.code) + ": what failed: sentinel\nreading: typed cause"; err.Error() != want {
				t.Fatalf("%s().Error() = %q, want %q", test.name, err.Error(), want)
			}
			if !errors.Is(err, sentinel) {
				t.Fatalf("errors.Is(%s(), sentinel) = false", test.name)
			}
			typed, ok := errors.AsType[*causeError](err)
			if !ok || typed.detail != "typed cause" {
				t.Fatalf("errors.AsType[*causeError](%s()) = %v, %v", test.name, typed, ok)
			}
			if without := requireDiagnostic(t, test.build("what failed", nil)); without.Cause() != nil || without.Unwrap() != nil {
				t.Fatalf("%s() without a cause unwraps to %v", test.name, without.Unwrap())
			}
		})
	}
}

func TestMismatchedConstructorKeepsMessageAndCauseInInternalInvariant(t *testing.T) {
	t.Parallel()

	cause := errors.New("reader failed")
	tests := []struct {
		name        string
		err         error
		wantMessage string
	}{
		{
			name:        "with message",
			err:         Validation(CodeFormatXML, "caller message", cause),
			wantMessage: `diagnostic code "format.xml" does not belong to category "validation": caller message`,
		},
		{
			name:        "without message",
			err:         Format(CodeSchemaXML, "", cause),
			wantMessage: `diagnostic code "schema.xml" does not belong to category "format"`,
		},
		{
			name:        "schema-parse-only code in compile",
			err:         SchemaCompile(CodeSchemaXML, "caller message", cause),
			wantMessage: `diagnostic code "schema.xml" does not belong to category "schema_compile": caller message`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			diagnostic := requireDiagnostic(t, test.err)
			if diagnostic.Category() != CategoryInternal || diagnostic.Code() != CodeInternalInvariant {
				t.Fatalf("mismatched pair produced %s/%s, want internal/internal.invariant", diagnostic.Category(), diagnostic.Code())
			}
			if diagnostic.Message() != test.wantMessage {
				t.Fatalf("Message() = %q, want %q", diagnostic.Message(), test.wantMessage)
			}
			if !errors.Is(test.err, cause) {
				t.Fatal("mismatched pair dropped its cause")
			}
		})
	}
}

func TestIsUnsupportedFollowsDiagnosticCausesExceptInternalInvariants(t *testing.T) {
	t.Parallel()

	unsupported := Unsupported(CodeUnsupportedRegex, "unsupported regex", nil)
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unsupported", unsupported, true},
		{"schema parse", SchemaParse(CodeSchemaXML, "bad XML", nil), false},
		{"schema compile", SchemaCompile(CodeSchemaFacet, "bad facet", nil), false},
		{"validation", Validation(CodeValidationType, "bad type", nil), false},
		{"format", Format(CodeFormatXML, "bad XML", nil), false},
		{"internal invariant", InternalInvariant("broken", nil), false},
		{"plain cause", SchemaCompile(CodeSchemaFacet, "bad facet", errors.New("plain")), false},
		{"schema parse with unsupported cause", fmt.Errorf("outer: %w", SchemaParse(CodeSchemaXML, "bad XML", unsupported)), true},
		{"schema compile with unsupported cause", SchemaCompile(CodeSchemaFacet, "bad facet", unsupported), true},
		{"internal invariant with unsupported cause", InternalInvariant("broken", unsupported), false},
		{"mismatched pair with unsupported cause", Unsupported(CodeSchemaXML, "wrong category", unsupported), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := IsUnsupported(test.err); got != test.want {
				t.Fatalf("IsUnsupported(%v) = %v, want %v", test.err, got, test.want)
			}
		})
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
