package value_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestValidateAdmittedNormalizedReturnsWhitespaceSpelling(t *testing.T) {
	t.Parallel()

	program, types := normalizedResultProgram(t)
	resolver := value.Resolver{QName: func(lexical string) (value.ExpandedName, bool) {
		return value.ExpandedName{Namespace: "urn:values", Local: "item"}, lexical == "p:item"
	}}
	for _, test := range []struct {
		name       string
		typ        value.TypeID
		lexical    string
		normalized string
		canonical  string
	}{
		{name: "preserve atom", typ: normalizedBuiltin(t, "string"), lexical: " \talpha\r\nbeta  ", normalized: " \talpha\r\nbeta  ", canonical: " \talpha\r\nbeta  "},
		{name: "replace atom", typ: normalizedBuiltin(t, "normalizedString"), lexical: " \talpha\r\nbeta  ", normalized: "  alpha  beta  ", canonical: "  alpha  beta  "},
		{name: "integer keeps sign and zeroes", typ: normalizedBuiltin(t, "integer"), lexical: " \t+001\r\n ", normalized: "+001", canonical: "1"},
		{name: "integer list keeps item spellings", typ: types.list, lexical: " \t+001\r\n002  ", normalized: "+001 002", canonical: "1 2"},
		{name: "nested union integer after boolean fails", typ: types.union, lexical: " \t+001\r\n ", normalized: "+001", canonical: "1"},
		{name: "nested union string after inner union fails", typ: types.union, lexical: " \tword\r\n  ", normalized: " \tword\r\n  ", canonical: " \tword\r\n  "},
		{name: "document ID", typ: normalizedBuiltin(t, "ID"), lexical: " \talpha\r\n ", normalized: "alpha", canonical: "alpha"},
		{name: "document IDREF list", typ: normalizedBuiltin(t, "IDREFS"), lexical: " \talpha\r\nbeta  ", normalized: "alpha beta", canonical: "alpha beta"},
		{name: "qualified name spelling", typ: normalizedBuiltin(t, "QName"), lexical: " \tp:item\r\n ", normalized: "p:item", canonical: "{urn:values}item"},
	} {
		for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
			t.Run(fmt.Sprintf("%s/needs=%d", test.name, needs), func(t *testing.T) {
				t.Parallel()
				got, normalized, err := program.ValidateAdmittedNormalized(test.typ, test.lexical, resolver, needs, 16<<20, nil)
				if err != nil {
					t.Fatalf("ValidateAdmittedNormalized(%q) error = %v", test.lexical, err)
				}
				if normalized != test.normalized {
					t.Fatalf("normalized = %q, want source whitespace spelling %q", normalized, test.normalized)
				}
				wantCanonical := ""
				if needs.Has(value.NeedCanonical) {
					wantCanonical = test.canonical
				}
				if got.CanonicalText() != wantCanonical {
					t.Fatalf("canonical = %q, want %q", got.CanonicalText(), wantCanonical)
				}
				old, err := program.ValidateAdmitted(test.typ, test.lexical, resolver, needs, 16<<20, nil)
				if err != nil {
					t.Fatalf("ValidateAdmitted(%q) error = %v", test.lexical, err)
				}
				assertNormalizedValueParity(t, got, old)
			})
		}
	}
}

func TestValidateAdmittedNormalizedReturnsNoSpellingOnFailure(t *testing.T) {
	t.Parallel()

	program, types := normalizedResultProgram(t)
	for _, test := range []struct {
		name    string
		program *value.Program
		typ     value.TypeID
		lexical string
		work    uint64
		wantErr error
	}{
		{name: "lexical", program: program, typ: normalizedBuiltin(t, "integer"), lexical: "  invalid  ", work: 16 << 20},
		{name: "atomic facet", program: program, typ: types.short, lexical: "  aa  ", work: 16 << 20, wantErr: value.ErrFacet},
		{name: "outer facet after numeric member succeeds", program: program, typ: types.patternUnion, lexical: "  +001  ", work: 16 << 20, wantErr: value.ErrFacet},
		{name: "work limit", program: program, typ: types.union, lexical: "  +001  ", work: 1, wantErr: value.ErrLimit},
		{name: "zero work limit", program: program, typ: types.union, lexical: "  +001  ", wantErr: value.ErrMetadata},
		{name: "missing type", program: program, typ: value.NoType, lexical: "  text  ", work: 16 << 20, wantErr: value.ErrMetadata},
		{name: "nil program", lexical: "  text  ", work: 16 << 20, wantErr: value.ErrMetadata},
		{name: "unsealed program", program: &value.Program{}, lexical: "  text  ", work: 16 << 20, wantErr: value.ErrMetadata},
	} {
		for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
			t.Run(fmt.Sprintf("%s/needs=%d", test.name, needs), func(t *testing.T) {
				t.Parallel()
				_, normalized, err := test.program.ValidateAdmittedNormalized(test.typ, test.lexical, value.Resolver{}, needs, test.work, nil)
				if err == nil || test.wantErr != nil && !errors.Is(err, test.wantErr) {
					t.Fatalf("ValidateAdmittedNormalized() error = %v, want rejection matching %v", err, test.wantErr)
				}
				if normalized != "" {
					t.Fatalf("failed validation returned normalized spelling %q", normalized)
				}
			})
		}
	}
}

func TestValidateAdmittedNormalizedReusesScratchAfterFailure(t *testing.T) {
	t.Parallel()

	program, types := normalizedResultProgram(t)
	for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
		t.Run(fmt.Sprintf("needs=%d", needs), func(t *testing.T) {
			t.Parallel()
			var scratch value.Scratch
			for _, step := range []struct {
				typ        value.TypeID
				lexical    string
				normalized string
				wantErr    error
			}{
				{typ: types.patternUnion, lexical: "ABC", normalized: "ABC"},
				{typ: types.patternUnion, lexical: " \t+001  ", wantErr: value.ErrFacet},
				{typ: types.union, lexical: " \tword\n  ", normalized: " \tword\n  "},
			} {
				got, normalized, err := program.ValidateAdmittedNormalized(step.typ, step.lexical, value.Resolver{}, needs, 16<<20, &scratch)
				if step.wantErr != nil {
					if !errors.Is(err, step.wantErr) || normalized != "" {
						t.Fatalf("failed step returned normalized %q, error %v; want empty spelling and %v", normalized, err, step.wantErr)
					}
					continue
				}
				if err != nil || normalized != step.normalized {
					t.Fatalf("successful step returned normalized %q, error %v; want %q", normalized, err, step.normalized)
				}
				old, err := program.ValidateAdmitted(step.typ, step.lexical, value.Resolver{}, needs, 16<<20, &scratch)
				if err != nil {
					t.Fatalf("ValidateAdmitted() after successful step error = %v", err)
				}
				assertNormalizedValueParity(t, got, old)
			}
		})
	}
}

type normalizedResultTypes struct {
	list         value.TypeID
	union        value.TypeID
	patternUnion value.TypeID
	short        value.TypeID
}

func normalizedResultProgram(t *testing.T) (*value.Program, normalizedResultTypes) {
	t.Helper()
	builder := value.NewBuilder(value.BuilderOptions{})
	integer := normalizedBuiltin(t, "integer")
	inner := completeNormalizedType(t, builder, value.TypeSpec{
		Variety: value.Union, Base: value.NoType, ListItem: value.NoType,
		Union: []value.TypeID{normalizedBuiltin(t, "boolean"), integer},
	})
	outerSpec := value.TypeSpec{
		Variety: value.Union, Base: value.NoType, ListItem: value.NoType,
		Union: []value.TypeID{inner, normalizedBuiltin(t, "string")},
	}
	types := normalizedResultTypes{
		union: completeNormalizedType(t, builder, outerSpec),
		list: completeNormalizedType(t, builder, value.TypeSpec{
			Variety: value.List, Base: value.NoType, ListItem: integer,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		}),
		short: completeNormalizedType(t, builder, atomicFacetType(value.PrimitiveString, value.FacetSpec{
			MaxLength: value.CardinalityFacet{Value: 1, Present: true},
		})),
	}
	pattern, err := xsdregex.Compile("[A-Z]+", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	outerSpec.Facets = value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}}
	types.patternUnion = completeNormalizedType(t, builder, outerSpec)
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	return program, types
}

func completeNormalizedType(t *testing.T, builder *value.Builder, spec value.TypeSpec) value.TypeID {
	t.Helper()
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Complete(id, spec); err != nil {
		t.Fatal(err)
	}
	return id
}

func normalizedBuiltin(t *testing.T, name string) value.TypeID {
	t.Helper()
	id, ok := value.BuiltinTypeID(name)
	if !ok {
		t.Fatalf("missing built-in type %q", name)
	}
	return id
}

func assertNormalizedValueParity(t *testing.T, got, old value.Value) {
	t.Helper()
	if got.Type() != old.Type() || got.SelectedType() != old.SelectedType() ||
		got.CanonicalText() != old.CanonicalText() || got.IdentityKey() != old.IdentityKey() ||
		got.IDs() != old.IDs() || got.IDRefs() != old.IDRefs() || got.HasQualifiedNames() != old.HasQualifiedNames() {
		t.Fatalf("normalized API changed value projections: got %+v, old API %+v", got, old)
	}
}
