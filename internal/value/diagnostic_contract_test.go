package value_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestTerminalFacetDiagnostics(t *testing.T) {
	pattern, err := xsdregex.Compile(`[0-9]+`, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	bound := func(text string) value.BoundFacet {
		return value.BoundFacet{Lexical: text, Type: value.NoType, Present: true}
	}
	for _, test := range []struct {
		name, input, want string
		primitive         value.PrimitiveKind
		facets            value.FacetSpec
		list              bool
		integer           bool
	}{
		{name: "unicode length", input: "é", want: `value "é" has length 1 character; expected exactly 2`, facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 2, Present: true}}},
		{name: "minimum length", input: "a", want: `value "a" has length 1 character; expected at least 2`, facets: value.FacetSpec{MinLength: value.CardinalityFacet{Value: 2, Present: true}}},
		{name: "maximum length", input: "abc", want: `value "abc" has length 3 characters; expected at most 2`, facets: value.FacetSpec{MaxLength: value.CardinalityFacet{Value: 2, Present: true}}},
		{name: "decoded binary length", primitive: value.PrimitiveBase64Binary, input: "YWI=", want: `value "YWI=" has length 2 bytes; expected exactly 1`, facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}}},
		{name: "list length", list: true, input: "1 2 3", want: `value "1 2 3" has length 3 items; expected exactly 2`, facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 2, Present: true}}},
		{name: "total digits", primitive: value.PrimitiveDecimal, input: "004.2500", want: `value "004.2500" has 3 total digits; totalDigits permits at most 2`, facets: value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 2, Present: true}}},
		{name: "fraction digits", primitive: value.PrimitiveDecimal, input: "004.2500", want: `value "004.2500" has 2 fractional digits; fractionDigits permits at most 1`, facets: value.FacetSpec{FractionDigits: value.CardinalityFacet{Value: 1, Present: true}}},
		{name: "pattern", input: "other", want: `value "other" does not satisfy pattern groups ["[0-9]+"]`, facets: value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}}},
		{name: "inclusive minimum", primitive: value.PrimitiveDecimal, input: "1", want: `value "1" does not satisfy minInclusive "2.0"`, facets: value.FacetSpec{MinInclusive: bound("2")}},
		{name: "exclusive minimum", primitive: value.PrimitiveDecimal, input: "2", want: `value "2" does not satisfy minExclusive "2.0"`, facets: value.FacetSpec{MinExclusive: bound("2")}},
		{name: "inclusive maximum", primitive: value.PrimitiveDecimal, input: "3", want: `value "3" does not satisfy maxInclusive "2.0"`, facets: value.FacetSpec{MaxInclusive: bound("2")}},
		{name: "exclusive maximum", primitive: value.PrimitiveDecimal, input: "2", want: `value "2" does not satisfy maxExclusive "2.0"`, facets: value.FacetSpec{MaxExclusive: bound("2")}},
		{name: "raw integer minimum", integer: true, primitive: value.PrimitiveDecimal, input: "9", want: `value "9" does not satisfy minInclusive "10"`, facets: value.FacetSpec{MinInclusive: bound("10")}},
		{name: "raw integer maximum", integer: true, primitive: value.PrimitiveDecimal, input: "11", want: `value "11" does not satisfy maxInclusive "10"`, facets: value.FacetSpec{MaxInclusive: bound("10")}},
		{name: "incomparable dateTime", primitive: value.PrimitiveDateTime, input: "2000-01-01T12:00:00", want: `value "2000-01-01T12:00:00" does not satisfy minInclusive "2000-01-01T12:00:00Z"`, facets: value.FacetSpec{MinInclusive: bound("2000-01-01T12:00:00Z")}},
		{name: "incomparable duration", primitive: value.PrimitiveDuration, input: "P1Y", want: `value "P1Y" does not satisfy minInclusive "P365D"`, facets: value.FacetSpec{MinInclusive: bound("P365D")}},
		{name: "duration components", primitive: value.PrimitiveDuration, input: "PT0S", want: `value "PT0S" does not satisfy minInclusive "P1Y2M3DT4H5M6.7S"`, facets: value.FacetSpec{MinInclusive: bound("P1Y2M3DT4H5M6.7S")}},
		{name: "negative duration", primitive: value.PrimitiveDuration, input: "PT0S", want: `value "PT0S" does not satisfy maxInclusive "-P1Y2M3DT4H5M6.7S"`, facets: value.FacetSpec{MaxInclusive: bound("-P1Y2M3DT4H5M6.7S")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := atomicFacetType(test.primitive, test.facets)
			if test.integer {
				spec.Builtin = value.BuiltinInteger
			}
			if test.primitive == value.PrimitiveString {
				spec.Whitespace = value.WhitespacePreserve
			}
			if test.list {
				spec.Variety, spec.ListItem = value.List, value.BuiltinType(value.PrimitiveString)
				spec.Whitespace = value.WhitespaceCollapse
			}
			program, id := enumerationDiagnosticProgram(t, spec)
			for _, api := range []string{"string", "bytes", "admitted"} {
				for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
					_, validationErr := diagnosticValidate(program, id, test.input, api, needs)
					if !errors.Is(validationErr, value.ErrFacet) {
						t.Fatalf("%s needs=%d: error = %v, want facet failure", api, needs, validationErr)
					}
					if got := validationErr.Error(); got != test.want {
						t.Fatalf("%s needs=%d: diagnostic = %q, want %q", api, needs, got, test.want)
					}
				}
			}
		})
	}
}

func TestUnionRejectedMemberDiagnosticsAreTerminal(t *testing.T) {
	pattern, err := xsdregex.Compile(`[0-9]+`, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	integer, ok := value.BuiltinTypeID("integer")
	if !ok {
		t.Fatal("missing integer builtin")
	}
	boundedInteger := atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{MinInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: "10", Type: value.NoType}, Present: true}})
	boundedInteger.Builtin = value.BuiltinInteger
	for _, test := range []struct {
		name, input, firstAccepted string
		first                      value.TypeSpec
	}{
		{name: "enumeration", input: "waiting", firstAccepted: "ready", first: atomicFacetType(value.PrimitiveString, value.FacetSpec{Enumeration: []value.LiteralSpec{{Lexical: "ready", Type: value.NoType}}})},
		{name: "pattern", input: "other", firstAccepted: "123", first: atomicFacetType(value.PrimitiveString, value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}})},
		{name: "bound", input: "9", firstAccepted: "10", first: boundedInteger},
		{name: "list item", input: "1 other", firstAccepted: "1 2", first: value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: integer, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true}},
		{name: "list length", input: "1 2 3", firstAccepted: "1 2", first: value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: integer, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 2, Present: true}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			first := completeCanonicalProjectionType(t, builder, test.first)
			fallback := value.BuiltinType(value.PrimitiveString)
			union := completeCanonicalProjectionType(t, builder, value.TypeSpec{Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{first, fallback}})
			program, sealErr := builder.Seal()
			if sealErr != nil {
				t.Fatal(sealErr)
			}
			for _, api := range []string{"string", "bytes", "admitted"} {
				for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
					got, validationErr := diagnosticValidate(program, union, test.input, api, needs)
					canonical := ""
					if needs != 0 {
						canonical = test.input
					}
					if validationErr != nil || got.Type() != union || got.SelectedType() != fallback || got.CanonicalText() != canonical || got.HasIdentity() != (needs != 0) {
						t.Fatalf("%s needs=%d fallback result = %#v, %v", api, needs, got, validationErr)
					}
				}
				got, validationErr := diagnosticValidate(program, union, test.firstAccepted, api, 0)
				if validationErr != nil || got.SelectedType() != first {
					t.Fatalf("%s first-member result = %#v, %v", api, got, validationErr)
				}
			}
		})
	}
}

func TestTerminalLexicalAndUnionDiagnostics(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	integer, _ := value.BuiltinTypeID("integer")
	union := completeCanonicalProjectionType(t, builder, value.TypeSpec{Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{value.BuiltinType(value.PrimitiveBoolean), integer}})
	list := completeCanonicalProjectionType(t, builder, value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: integer, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true})
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id          value.TypeID
		input, want string
	}{
		{integer, "1.1", `value "1.1" is not valid for xs:integer: invalid integer`},
		{union, "other", `value "other" does not match any union member; expected one of [xs:boolean, xs:integer]`},
		{list, "1 other", `value "1 other" is not a valid list of xs:integer: invalid integer`},
	} {
		for _, api := range []string{"string", "bytes", "admitted"} {
			_, validationErr := diagnosticValidate(program, test.id, test.input, api, 0)
			if validationErr == nil || validationErr.Error() != test.want {
				t.Fatalf("%s input=%q: error = %v, want %q", api, test.input, validationErr, test.want)
			}
		}
	}
	_, limitErr := program.Validate(union, strings.Repeat("x", 50), value.Resolver{}, 0, 1, nil)
	if !errors.Is(limitErr, value.ErrLimit) {
		t.Fatalf("work error = %v, want terminal limit", limitErr)
	}
}

func TestConstrainedUnionDiagnostic(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	member := func(literal string) value.TypeID {
		return completeCanonicalProjectionType(t, builder, atomicFacetType(value.PrimitiveString, value.FacetSpec{Enumeration: []value.LiteralSpec{{Lexical: literal, Type: value.NoType}}}))
	}
	ready, waiting := member("ready"), member("waiting")
	union := completeCanonicalProjectionType(t, builder, value.TypeSpec{Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{ready, waiting}})
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"string", "bytes", "admitted"} {
		for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
			_, validationErr := diagnosticValidate(program, union, "bogus", api, needs)
			if !errors.Is(validationErr, value.ErrFacet) {
				t.Fatalf("%s needs=%d: error = %v, want facet failure", api, needs, validationErr)
			}
			for _, fact := range []string{`value "bogus"`, `does not match any union member`, `enumeration groups ["ready"]`, `enumeration groups ["waiting"]`} {
				if !strings.Contains(validationErr.Error(), fact) {
					t.Errorf("%s needs=%d: diagnostic %q lacks %q", api, needs, validationErr, fact)
				}
			}
			for input, selected := range map[string]value.TypeID{"ready": ready, "waiting": waiting} {
				got, controlErr := diagnosticValidate(program, union, input, api, needs)
				if controlErr != nil || got.SelectedType() != selected {
					t.Fatalf("%s needs=%d input=%q: result=%#v error=%v", api, needs, input, got, controlErr)
				}
			}
		}
	}
}

func TestListItemFacetDiagnostic(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	item := completeCanonicalProjectionType(t, builder, atomicFacetType(value.PrimitiveString, value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}}))
	list := completeCanonicalProjectionType(t, builder, value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: item, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 2, Present: true}}})
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"string", "bytes", "admitted"} {
		for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
			_, validationErr := diagnosticValidate(program, list, "a bb", api, needs)
			if !errors.Is(validationErr, value.ErrFacet) {
				t.Fatalf("%s needs=%d: error=%v, want facet failure", api, needs, validationErr)
			}
			message := validationErr.Error()
			for _, fact := range []string{`value "a bb"`, `not a valid list of`, `length exactly 1`} {
				if !strings.Contains(message, fact) {
					t.Errorf("%s needs=%d: diagnostic %q lacks %q", api, needs, message, fact)
				}
			}
			if strings.Contains(message, "has length") || strings.Contains(message, "expected exactly 2") {
				t.Errorf("%s needs=%d: diagnostic attributes item failure to list length: %q", api, needs, message)
			}
			if _, controlErr := diagnosticValidate(program, list, "a b", api, needs); controlErr != nil {
				t.Fatalf("%s needs=%d: valid list: %v", api, needs, controlErr)
			}
		}
	}
}

func diagnosticValidate(program *value.Program, id value.TypeID, input, api string, needs value.Needs) (value.Value, error) {
	switch api {
	case "bytes":
		return program.ValidateBytes(id, []byte(input), value.Resolver{}, needs, 16<<20, nil)
	case "admitted":
		return program.ValidateAdmitted(id, input, value.Resolver{}, needs, 16<<20, nil)
	default:
		return program.Validate(id, input, value.Resolver{}, needs, 16<<20, nil)
	}
}
