package valuebench

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

// BenchmarkPaddedValue measures collapse-whitespace values that carry only
// leading and trailing XML whitespace, as indented element content does.
func BenchmarkPaddedValue(b *testing.B) {
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		b.Fatal(err)
	}
	for _, typ := range []paddedValueType{
		{local: "boolean", lexical: "true", invalid: "maybe"},
		{local: "decimal", lexical: "12.50", invalid: "1.2.3"},
		{local: "int", lexical: "42", invalid: "x"},
		// Every collapsed string is a token.
		{local: "token", lexical: "alpha beta"},
		{local: "date", lexical: "2026-10-28", invalid: "2026-13-01"},
		{local: "double", lexical: "1.5E3", invalid: "abc"},
		{local: "NMTOKEN", lexical: "alpha", invalid: "a b"},
	} {
		id, ok := value.BuiltinTypeID(typ.local)
		if !ok {
			b.Fatalf("BuiltinTypeID(%q) missing", typ.local)
		}
		shapes := []struct{ name, text string }{
			{name: "plain", text: typ.lexical},
			{name: "padded", text: padValue(typ.lexical)},
			{name: "indented", text: indentValue(typ.lexical)},
		}
		requirePaddedValues(b, program, id, typ, shapes[1:])
		for _, shape := range shapes {
			raw := []byte(shape.text)
			b.Run(typ.local+"/"+shape.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(raw)))
				for b.Loop() {
					if _, err := program.ValidateBytes(id, raw, value.Resolver{}, 0, 16<<20, nil); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// paddedValueType is one builtin type with a valid lexical form and, when the
// type has one, an invalid lexical form.
type paddedValueType struct{ local, lexical, invalid string }

func padValue(lexical string) string    { return " " + lexical + " " }
func indentValue(lexical string) string { return "\n      " + lexical + "\n    " }

// requirePaddedValues checks that surrounding whitespace is insignificant: a
// padded form has the plain form's canonical text, and a padded invalid value
// fails for the same reason as the plain one.
func requirePaddedValues(b *testing.B, program *value.Program, id value.TypeID, typ paddedValueType, padded []struct{ name, text string }) {
	b.Helper()
	local, invalid := typ.local, typ.invalid
	canonical := func(text string) string {
		got, err := program.ValidateBytes(id, []byte(text), value.Resolver{}, value.NeedCanonical, 16<<20, nil)
		if err != nil {
			b.Fatalf("%s %q: %v", local, text, err)
		}
		return got.CanonicalText()
	}
	want := canonical(typ.lexical)
	for _, shape := range padded {
		if got := canonical(shape.text); got != want {
			b.Fatalf("%s %s canonical = %q, want %q", local, shape.name, got, want)
		}
	}
	if invalid == "" {
		return
	}
	reject := func(text string) error {
		_, err := program.ValidateBytes(id, []byte(text), value.Resolver{}, 0, 16<<20, nil)
		if err == nil {
			b.Fatalf("%s %q accepted, want rejection", local, text)
		}
		return err
	}
	wantErr := reject(invalid)
	for _, text := range []string{padValue(invalid), indentValue(invalid)} {
		err := reject(text)
		if errors.Is(err, value.ErrFacet) != errors.Is(wantErr, value.ErrFacet) || rejectionReason(err) != rejectionReason(wantErr) {
			b.Fatalf("%s %q error = %v, want the same rejection as %v", local, text, err, wantErr)
		}
	}
}

// rejectionReason drops the diagnostic prefix, which quotes the input.
func rejectionReason(err error) string {
	message := err.Error()
	return message[strings.LastIndex(message, ": ")+1:]
}

// BenchmarkListTrailingSpace isolates the list collapse copy: the same items
// with and without one trailing space.
func BenchmarkListTrailingSpace(b *testing.B) {
	const items = 32 << 10
	program, id := benchmarkListProgram(b, value.FacetSpec{})
	exact := strings.TrimSuffix(strings.Repeat("a ", items), " ")
	shapes := []struct{ name, text string }{
		{name: "exact", text: exact},
		{name: "trailing", text: exact + " "},
	}
	for _, shape := range shapes {
		got, err := program.Validate(id, shape.text, value.Resolver{}, value.NeedCanonical, 1<<40, nil)
		if err != nil {
			b.Fatalf("%s: %v", shape.name, err)
		}
		if canonical := got.CanonicalText(); canonical != exact {
			b.Fatalf("%s canonical has %d items, want %d", shape.name, strings.Count(canonical, " ")+1, items)
		}
	}
	for _, shape := range shapes {
		b.Run(shape.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(shape.text)))
			for b.Loop() {
				if _, err := program.Validate(id, shape.text, value.Resolver{}, 0, 1<<40, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
