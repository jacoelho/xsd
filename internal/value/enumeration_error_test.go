package value_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/value"
)

func TestEnumerationDiagnostic(t *testing.T) {
	for _, test := range []struct {
		name      string
		primitive value.PrimitiveKind
		literals  []string
		input     string
		want      string
		list      bool
	}{
		{name: "string", literals: []string{"onLoad", "onRequest"}, input: "bogus", want: `value "bogus" is not one of ["onLoad", "onRequest"]`},
		{name: "empty input", literals: []string{"ok"}, want: `value "" is not one of ["ok"]`},
		{name: "empty literal", literals: []string{""}, input: "bad", want: `value "bad" is not one of [""]`},
		{name: "escaping", literals: []string{"a\"\\\t"}, input: "b\"\\\n", want: `value "b\"\\\n" is not one of ["a\"\\\t"]`},
		{name: "numeric lexical input", primitive: value.PrimitiveDecimal, literals: []string{"1"}, input: "+02.00", want: `value "+02.00" is not one of ["1.0"]`},
		{name: "list", list: true, literals: []string{"1 2"}, input: "1 3", want: `value "1 3" is not one of ["1 2"]`},
		{name: "QName", primitive: value.PrimitiveQName, literals: []string{"p:a"}, input: "p:b", want: `value "p:b" is not one of ["{urn:test}a"]`},
		{name: "at byte limit", literals: []string{"ok"}, input: strings.Repeat("a", 128), want: `value "` + strings.Repeat("a", 128) + `" is not one of ["ok"]`},
		{name: "UTF-8 cutoff", literals: []string{"ok"}, input: strings.Repeat("a", 127) + "éx", want: `value "` + strings.Repeat("a", 127) + `"... is not one of ["ok"]`},
		{name: "long literal", literals: []string{strings.Repeat("z", 129)}, input: "bad", want: `value "bad" is not one of ["` + strings.Repeat("z", 128) + `"...]`},
		{name: "eight alternatives", literals: []string{"a", "b", "c", "d", "e", "f", "g", "h"}, input: "bad", want: `value "bad" is not one of ["a", "b", "c", "d", "e", "f", "g", "h"]`},
		{name: "nine alternatives", literals: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}, input: "bad", want: `value "bad" is not one of ["a", "b", "c", "d", "e", "f", "g", "h", ...]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := value.Resolver{QName: func(lexical string) (value.ExpandedName, bool) {
				return value.ExpandedName{Namespace: "urn:test", Local: strings.TrimPrefix(lexical, "p:")}, true
			}}
			spec := atomicFacetType(test.primitive, value.FacetSpec{})
			if test.primitive == value.PrimitiveString {
				spec.Whitespace = value.WhitespacePreserve
			}
			if test.list {
				spec.Variety, spec.ListItem = value.List, value.BuiltinType(value.PrimitiveString)
				spec.Whitespace = value.WhitespaceCollapse
			}
			for _, literal := range test.literals {
				spec.Facets.Enumeration = append(spec.Facets.Enumeration, value.LiteralSpec{Lexical: literal, Type: value.NoType, Resolver: resolver})
			}
			program, id := enumerationDiagnosticProgram(t, spec)
			for _, api := range []string{"string", "bytes", "admitted"} {
				t.Run(api, func(t *testing.T) {
					var err error
					switch api {
					case "string":
						_, err = program.Validate(id, test.input, resolver, 0, 16<<20, nil)
					case "bytes":
						_, err = program.ValidateBytes(id, []byte(test.input), resolver, 0, 16<<20, nil)
					case "admitted":
						_, err = program.ValidateAdmitted(id, test.input, resolver, 0, 16<<20, nil)
					}
					if !errors.Is(err, value.ErrFacet) {
						t.Fatalf("validation error = %v, want facet failure", err)
					}
					if got := err.Error(); got != test.want || !utf8.ValidString(got) {
						t.Fatalf("diagnostic = %q, want %q", got, test.want)
					}
				})
			}
		})
	}
}

func TestEnumerationDiagnosticOwnsBorrowedInput(t *testing.T) {
	spec := atomicFacetType(value.PrimitiveString, value.FacetSpec{Enumeration: []value.LiteralSpec{{Lexical: "ok", Type: value.NoType}}})
	spec.Whitespace = value.WhitespacePreserve
	program, id := enumerationDiagnosticProgram(t, spec)
	raw := []byte("bad")
	var scratch value.Scratch
	_, err := program.ValidateBytes(id, raw, value.Resolver{}, 0, 16<<20, &scratch)
	if !errors.Is(err, value.ErrFacet) {
		t.Fatalf("validation error = %v, want facet failure", err)
	}
	copy(raw, "new")
	if _, validErr := program.ValidateBytes(id, []byte("ok"), value.Resolver{}, 0, 16<<20, &scratch); validErr != nil {
		t.Fatal(validErr)
	}
	if got, want := err.Error(), `value "bad" is not one of ["ok"]`; got != want {
		t.Fatalf("retained diagnostic = %q, want %q", got, want)
	}
}

func enumerationDiagnosticProgram(t *testing.T, spec value.TypeSpec) (*value.Program, value.TypeID) {
	t.Helper()
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if completionErr := builder.Complete(id, spec); completionErr != nil {
		t.Fatal(completionErr)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	return program, id
}
