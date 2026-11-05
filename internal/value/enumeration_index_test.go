package value

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// enumerationSubject is one restriction shape carrying an enumeration. raw
// reports whether ValidateBytes takes the raw string-enumeration plan, so the
// same program exercises both the raw and the typed membership paths.
type enumerationSubject struct {
	name string
	spec func(literals []LiteralSpec) TypeSpec
	raw  bool
}

func enumerationSubjects() []enumerationSubject {
	derived := func(base TypeID, primitive PrimitiveKind, whitespace WhitespaceMode) func([]LiteralSpec) TypeSpec {
		return func(literals []LiteralSpec) TypeSpec {
			return TypeSpec{
				Variety: Atomic, Primitive: primitive, Whitespace: whitespace,
				Base: base, ListItem: NoType,
				Facets: FacetSpec{Enumeration: literals},
			}
		}
	}
	anyURI, _ := BuiltinTypeID("anyURI")
	return []enumerationSubject{
		{name: "string_preserve", spec: testPreservedStringEnumeration, raw: true},
		{name: "token", spec: derived(builtinToken, PrimitiveString, WhitespaceCollapse)},
		{name: "normalizedString", spec: derived(builtinNormalizedString, PrimitiveString, WhitespaceReplace)},
		{name: "anyURI", spec: derived(anyURI, PrimitiveAnyURI, WhitespaceCollapse)},
		{name: "string_maxlen", spec: func(literals []LiteralSpec) TypeSpec {
			spec := testPreservedStringEnumeration(literals)
			spec.Facets.MaxLength = CardinalityFacet{Value: 64, Present: true}
			return spec
		}},
	}
}

func testPreservedStringEnumeration(literals []LiteralSpec) TypeSpec {
	return TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: literals},
	}
}

// testEnumerationChain completes specs in order, each restricting the one
// before it, and seals the program.
func testEnumerationChain(t *testing.T, specs ...TypeSpec) (*Program, []TypeID) {
	t.Helper()
	builder := NewBuilder(BuilderOptions{})
	ids := make([]TypeID, len(specs))
	for i, spec := range specs {
		id, err := builder.Reserve()
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		if i > 0 {
			spec.Base = ids[i-1]
		}
		if err := builder.Complete(id, spec); err != nil {
			t.Fatalf("Complete(type %d) error = %v", i, err)
		}
		ids[i] = id
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	return program, ids
}

func requireRawEnumerationPlan(t *testing.T, program *Program, ids ...TypeID) {
	t.Helper()
	for _, id := range ids {
		if typ, ok := program.typeDef(id); !ok || typ.facets.raw.kind != rawPlanStringEnumeration {
			t.Fatalf("type %d raw plan = %v, want string enumeration", id, typ.facets.raw.kind)
		}
	}
}

// enumerationValidators are the three public entry points; for a type with
// the raw plan, ValidateBytes takes it and the others take the typed path.
var enumerationValidators = []struct {
	name     string
	validate func(p *Program, id TypeID, input string) error
}{
	{name: "string", validate: func(p *Program, id TypeID, input string) error {
		_, err := p.Validate(id, input, Resolver{}, 0, 16<<20, nil)
		return err
	}},
	{name: "admitted", validate: func(p *Program, id TypeID, input string) error {
		_, err := p.ValidateAdmitted(id, input, Resolver{}, 0, 16<<20, nil)
		return err
	}},
	{name: "bytes", validate: func(p *Program, id TypeID, input string) error {
		_, err := p.ValidateBytes(id, []byte(input), Resolver{}, 0, 16<<20, nil)
		return err
	}},
}

// requireEnumerationMatch checks that every entry point accepts input.
func requireEnumerationMatch(t *testing.T, program *Program, id TypeID, input string) {
	t.Helper()
	for _, validator := range enumerationValidators {
		if err := validator.validate(program, id, input); err != nil {
			t.Fatalf("%s(%q) error = %v, want match", validator.name, input, err)
		}
	}
}

// requireEnumerationRejection checks that every entry point rejects input
// with the enumeration facet failure.
func requireEnumerationRejection(t *testing.T, program *Program, id TypeID, input string) {
	t.Helper()
	for _, validator := range enumerationValidators {
		if err := validator.validate(program, id, input); !errors.Is(err, ErrFacet) {
			t.Fatalf("%s(%q) error = %v, want ErrFacet", validator.name, input, err)
		}
	}
}

type enumerationMembership struct {
	input string
	valid bool
}

func requireEnumerationCases(t *testing.T, program *Program, id TypeID, cases []enumerationMembership) {
	t.Helper()
	for _, c := range cases {
		if c.valid {
			requireEnumerationMatch(t, program, id, c.input)
		} else {
			requireEnumerationRejection(t, program, id, c.input)
		}
	}
}

func TestEnumerationMembershipAcrossSizes(t *testing.T) {
	t.Parallel()
	sizes := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 32, 1024}
	for _, subject := range enumerationSubjects() {
		for _, n := range sizes {
			t.Run(fmt.Sprintf("%s/n_%d", subject.name, n), func(t *testing.T) {
				t.Parallel()
				// Descending, with the first literal repeated last.
				values := make([]string, 0, n+1)
				for i := n - 1; i >= 0; i-- {
					values = append(values, fmt.Sprintf("v%04d", i))
				}
				values = append(values, values[0])
				program, ids := testEnumerationChain(t, subject.spec(testStringLiteralSpecs(values)))
				id := ids[0]
				if subject.raw {
					requireRawEnumerationPlan(t, program, id)
				}
				checked := values[:n]
				if n > 12 {
					checked = []string{values[0], values[n/2], values[n-1]}
					for i := 1; i < n; i += 37 {
						checked = append(checked, values[i])
					}
				}
				for _, literal := range checked {
					requireEnumerationMatch(t, program, id, literal)
					requireEnumerationRejection(t, program, id, literal+"x")
					requireEnumerationRejection(t, program, id, literal[:len(literal)-1])
					requireEnumerationRejection(t, program, id, strings.ToUpper(literal))
				}
				requireEnumerationRejection(t, program, id, "")
			})
		}
	}
}

// enumerationFillers pads a group past every candidate index threshold with
// literals no case below uses as input.
func enumerationFillers(prefix string) []string {
	fillers := make([]string, 9)
	for i := range fillers {
		fillers[i] = fmt.Sprintf("%s%02d", prefix, i)
	}
	return fillers
}

func TestEnumerationStringFamilyKeys(t *testing.T) {
	t.Parallel()
	subjects := map[string]enumerationSubject{}
	for _, subject := range enumerationSubjects() {
		subjects[subject.name] = subject
	}
	longPrefix := strings.Repeat("p", 129)
	var long []string
	for c := 'a'; c <= 'i'; c++ {
		long = append(long, longPrefix+string(c))
	}
	type membership = enumerationMembership
	for _, test := range []struct {
		name     string
		subject  string
		literals []string
		cases    []membership
	}{
		{
			name: "preserved bytes", subject: "string_preserve",
			literals: append([]string{"", "é", "ab", "a"}, enumerationFillers("f")...),
			cases: []membership{
				{"", true}, {"é", true}, {"ab", true}, {"a", true},
				{" ", false}, {"é", false}, {"abc", false}, {"A", false}, {"\t", false},
			},
		},
		{
			name: "preserved space", subject: "string_preserve",
			literals: append([]string{" "}, enumerationFillers("f")...),
			cases:    []membership{{" ", true}, {"", false}},
		},
		{
			name: "collapsed", subject: "token",
			literals: append([]string{"", "  pad \t ded "}, enumerationFillers("f")...),
			cases: []membership{
				{"", true}, {" ", true}, {"pad  ded", true}, {" pad ded ", true}, {"pad dedx", false},
			},
		},
		{
			name: "replaced", subject: "normalizedString",
			literals: append([]string{"a\tb"}, enumerationFillers("f")...),
			cases:    []membership{{"a\tb", true}, {"a b", true}, {"a  b", false}},
		},
		{
			name: "URI", subject: "anyURI",
			literals: append([]string{"http://x/a"}, enumerationFillers("http://x/f")...),
			cases:    []membership{{"http://x/a", true}, {" http://x/a ", true}, {"http://x/A", false}},
		},
		{
			name: "long shared prefix", subject: "token",
			literals: long,
			cases:    []membership{{long[0], true}, {long[8], true}, {longPrefix + "Z", false}, {longPrefix, false}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			subject := subjects[test.subject]
			program, ids := testEnumerationChain(t, subject.spec(testStringLiteralSpecs(test.literals)))
			if subject.raw {
				requireRawEnumerationPlan(t, program, ids[0])
			}
			requireEnumerationCases(t, program, ids[0], test.cases)
		})
	}
}

func TestEnumerationMixedKindUnionGroups(t *testing.T) {
	t.Parallel()
	nmtokens, _ := BuiltinTypeID("NMTOKENS")
	union := func(members ...TypeID) TypeSpec {
		return TypeSpec{
			Variety: Union, Primitive: PrimitiveString, Union: members,
			Whitespace: WhitespaceCollapse, WhitespacePresent: true,
			Base: NoType, ListItem: NoType,
		}
	}
	type membership = enumerationMembership
	booleanCases := []membership{{"1", true}, {"true", true}, {"s3", true}, {"TRUE", false}, {"tru", false}}
	listCases := []membership{{"ab cd", true}, {"ab   cd", true}, {"ab", false}, {"!1", true}}
	// All literals select the string member, so these groups are indexed and
	// receive values of another kind or a list. Both the kind guard and the
	// equality check reject them, so only removing both makes these cases fail.
	booleanValueCases := []membership{{"true", false}, {"1", false}, {"tru", true}, {"", true}, {"s3", true}}
	listValueCases := []membership{{"ab cd", false}, {"ab", false}, {"", true}, {"!3", true}}
	strings8 := func(format string) []string {
		values := make([]string, 8)
		for i := range values {
			values[i] = fmt.Sprintf(format, i+1)
		}
		return values
	}
	for _, test := range []struct {
		name     string
		members  []TypeID
		literals []string
		cases    []membership
	}{
		{name: "boolean last", members: []TypeID{builtinBoolean, builtinString}, literals: append(strings8("s%d"), "true"), cases: booleanCases},
		{name: "boolean first", members: []TypeID{builtinBoolean, builtinString}, literals: append([]string{"true"}, strings8("s%d")...), cases: booleanCases},
		{name: "list last", members: []TypeID{nmtokens, builtinString}, literals: append(strings8("!%d"), "ab cd"), cases: listCases},
		{name: "list first", members: []TypeID{nmtokens, builtinString}, literals: append([]string{"ab cd"}, strings8("!%d")...), cases: listCases},
		{name: "boolean value", members: []TypeID{builtinBoolean, builtinString}, literals: append([]string{"", "tru"}, strings8("s%d")...), cases: booleanValueCases},
		{name: "list value", members: []TypeID{nmtokens, builtinString}, literals: append([]string{""}, strings8("!%d")...), cases: listValueCases},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			restricted := union(test.members...)
			restricted.Facets.Enumeration = testStringLiteralSpecs(test.literals)
			program, ids := testEnumerationChain(t, union(test.members...), restricted)
			requireEnumerationCases(t, program, ids[1], test.cases)
		})
	}
}

func TestEnumerationChainGroupsEachEnforced(t *testing.T) {
	t.Parallel()
	names := func(indexes ...int) []string {
		values := make([]string, len(indexes))
		for i, index := range indexes {
			values[i] = fmt.Sprintf("x%02d", index)
		}
		return values
	}
	span := func(from, to int) []int {
		var indexes []int
		for i := from; ; {
			indexes = append(indexes, i)
			if i == to {
				return indexes
			}
			if from < to {
				i++
			} else {
				i--
			}
		}
	}
	// A starts x11, x12 so its first eight differ from C's.
	a := names(append([]int{11, 12}, span(10, 1)...)...)
	b := names(span(3, 12)...)
	c := names(span(12, 4)...)
	program, ids := testEnumerationChain(t,
		testPreservedStringEnumeration(testStringLiteralSpecs(a)),
		testPreservedStringEnumeration(testStringLiteralSpecs(b)),
		testPreservedStringEnumeration(testStringLiteralSpecs(c)),
	)
	requireRawEnumerationPlan(t, program, ids...)
	leaf := ids[2]
	for _, input := range []string{"x04", "x12"} {
		requireEnumerationMatch(t, program, leaf, input)
	}
	for _, test := range []struct{ input, want string }{
		{"x01", `value "x01" is not one of ["x03", "x04", "x05", "x06", "x07", "x08", "x09", "x10", ...]`},
		{"x03", `value "x03" is not one of ["x12", "x11", "x10", "x09", "x08", "x07", "x06", "x05", ...]`},
		{"x13", `value "x13" is not one of ["x11", "x12", "x10", "x09", "x08", "x07", "x06", "x05", ...]`},
	} {
		for _, validator := range enumerationValidators {
			err := validator.validate(program, leaf, test.input)
			if !errors.Is(err, ErrFacet) || err.Error() != test.want {
				t.Fatalf("%s(%q) error = %v, want %s", validator.name, test.input, err, test.want)
			}
		}
	}

	t.Run("derivation", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name     string
			literals []string
			valid    bool
		}{
			{name: "inside every base group", literals: names(span(3, 11)...), valid: true},
			{name: "outside the nearest base group", literals: names(append([]int{2}, span(4, 11)...)...)},
		} {
			builder := NewBuilder(BuilderOptions{})
			base := NoType
			for _, literals := range [][]string{a, b} {
				id, err := builder.Reserve()
				if err != nil {
					t.Fatalf("Reserve() error = %v", err)
				}
				spec := testPreservedStringEnumeration(testStringLiteralSpecs(literals))
				spec.Base = base
				if err := builder.Complete(id, spec); err != nil {
					t.Fatal(err)
				}
				base = id
			}
			id, err := builder.Reserve()
			if err != nil {
				t.Fatalf("Reserve() error = %v", err)
			}
			spec := testPreservedStringEnumeration(testStringLiteralSpecs(test.literals))
			spec.Base = base
			if err := builder.Complete(id, spec); (err == nil) != test.valid {
				t.Fatalf("%s: Complete() error = %v, valid = %t", test.name, err, test.valid)
			}
		}
	})
}
