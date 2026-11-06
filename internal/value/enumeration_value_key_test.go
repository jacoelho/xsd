package value

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// valueSpacePair is one XSD 1.0 equality fact: literal and input are equal in
// the value space exactly when equal is set.
type valueSpacePair struct {
	literal, input string
	equal          bool
}

type valueSpaceKind struct {
	name   string
	specs  func(t *testing.T) []TypeSpec // base chain; the last spec receives the enumeration
	filler func(i int) string            // distinct values equal to no pair member
	pairs  []valueSpacePair
}

var valueSpaceResolver = Resolver{
	QName: func(lexical string) (ExpandedName, bool) {
		prefix, local, ok := strings.Cut(lexical, ":")
		if !ok {
			return ExpandedName{Local: lexical}, true
		}
		switch prefix {
		case "a", "b":
			return ExpandedName{Namespace: "urn:x", Local: local}, true
		case "c":
			return ExpandedName{Namespace: "urn:y", Local: local}, true
		}
		return ExpandedName{}, false
	},
	Notation: func(string, string) bool { return true },
}

// builtinRestriction is an atomic restriction of a builtin with the builtin's
// own semantic fields.
func builtinRestriction(local string) func(*testing.T) []TypeSpec {
	return func(t *testing.T) []TypeSpec {
		t.Helper()
		base, ok := BuiltinTypeID(local)
		if !ok {
			t.Fatalf("no builtin %s", local)
		}
		def, ok := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62}).program.typeDef(base)
		if !ok {
			t.Fatalf("no builtin definition %s", local)
		}
		listItem := NoType
		if def.variety == List {
			listItem = def.listItem
		}
		return []TypeSpec{{
			Variety: def.variety, Primitive: def.primitive, Builtin: def.builtin,
			Whitespace: def.whitespace, WhitespacePresent: true,
			Base: base, ListItem: listItem,
		}}
	}
}

func valueSpaceKinds(t *testing.T) []valueSpaceKind {
	t.Helper()
	decimal, _ := BuiltinTypeID("decimal")
	dateTime, _ := BuiltinTypeID("dateTime")
	listOf := func(item TypeID) func(*testing.T) []TypeSpec {
		return func(*testing.T) []TypeSpec {
			return []TypeSpec{{
				Variety: List, ListItem: item, Base: NoType,
				Whitespace: WhitespaceCollapse, WhitespacePresent: true,
			}}
		}
	}
	return []valueSpaceKind{
		{name: "decimal", specs: builtinRestriction("decimal"), filler: func(i int) string { return fmt.Sprintf("900%d", i) }, pairs: []valueSpacePair{
			{"1.0", "1", true}, {"+1", "1", true}, {"01", "1", true}, {"0", "-0", true}, {"0.0", "-0.00", true},
			{"1.5", "1.50", true}, {"1", "1.01", false}, {"-1", "1", false}, {"10", "1", false}, {"0.1", "0.01", false},
			{"1234567890123456789012345678901234567890", "1234567890123456789012345678901234567890.0", true},
			{"1234567890123456789012345678901234567890", "1234567890123456789012345678901234567891", false},
		}},
		{name: "integer", specs: builtinRestriction("integer"), filler: func(i int) string { return fmt.Sprintf("900%d", i) }, pairs: []valueSpacePair{
			{"01", "1", true}, {"-0", "0", true}, {"+5", "5", true}, {"5", "-5", false},
		}},
		{name: "float", specs: builtinRestriction("float"), filler: func(i int) string { return fmt.Sprintf("90%d", i) }, pairs: []valueSpacePair{
			{"0", "-0", true}, {"NaN", "NaN", true}, {"INF", "INF", true}, {"INF", "-INF", false},
			{"1e0", "1.0", true}, {"0.1", "0.10000000149", true}, {"1", "1.0000001", false}, {"NaN", "INF", false},
		}},
		{name: "double", specs: builtinRestriction("double"), filler: func(i int) string { return fmt.Sprintf("90%d", i) }, pairs: []valueSpacePair{
			{"0", "-0", true}, {"NaN", "NaN", true}, {"1E2", "100", true}, {"0.1", "0.10000000149", false},
		}},
		{name: "duration", specs: builtinRestriction("duration"), filler: func(i int) string { return fmt.Sprintf("P90%dY", i) }, pairs: []valueSpacePair{
			{"P1Y", "P12M", true}, {"PT60S", "PT1M", true}, {"P1D", "PT24H", true}, {"P1M", "P30D", false},
			{"P0D", "PT0S", true}, {"-P0D", "P0D", true}, {"PT1.0S", "PT1S", true}, {"PT1.5S", "PT1.50S", true},
			{"-PT1.5S", "PT1.5S", false}, {"-PT0.5S", "PT0.5S", false}, {"-PT0.0S", "PT0S", true},
			{"P99999999999999999999Y", "P99999999999999999999Y", true},
			{"P99999999999999999999Y", "P99999999999999999998Y", false},
			{"-P99999999999999999999Y", "P99999999999999999999Y", false},
		}},
		{name: "dateTime", specs: builtinRestriction("dateTime"), filler: func(i int) string { return fmt.Sprintf("1900-01-%02dT00:00:00Z", i+1) }, pairs: []valueSpacePair{
			{"2000-01-01T00:00:00Z", "2000-01-01T00:00:00+00:00", true},
			{"2000-01-01T01:00:00+01:00", "2000-01-01T00:00:00Z", true},
			{"2000-01-01T00:00:00-00:00", "2000-01-01T00:00:00Z", true},
			{"2000-01-01T00:00:00", "2000-01-01T00:00:00Z", false},
			{"1999-12-31T24:00:00", "2000-01-01T00:00:00", true},
			{"2000-01-01T00:00:00.0", "2000-01-01T00:00:00", true},
			{"2000-01-01T00:00:00.5Z", "2000-01-01T00:00:00.50Z", true},
			{"2000-01-01T00:00:00Z", "2000-01-01T00:00:01Z", false},
			{"-0001-01-01T00:00:00", "-0001-01-01T00:00:00", true},
			{"0001-01-01T00:00:00", "-0001-01-01T00:00:00", false},
			{"10000-01-01T00:00:00", "10000-01-01T00:00:00", true},
		}},
		{name: "date", specs: builtinRestriction("date"), filler: func(i int) string { return fmt.Sprintf("1900-01-%02d", i+1) }, pairs: []valueSpacePair{
			{"2000-01-01Z", "2000-01-01+00:00", true}, {"2000-01-02+14:00", "2000-01-01-10:00", true},
			{"2000-01-01", "2000-01-01Z", false}, {"2000-01-01", "2000-01-02", false},
		}},
		{name: "time", specs: builtinRestriction("time"), filler: func(i int) string { return fmt.Sprintf("05:%02d:07", i) }, pairs: []valueSpacePair{
			{"00:00:00Z", "01:00:00+01:00", true}, {"24:00:00", "00:00:00", true}, {"12:00:00.000", "12:00:00", true},
			{"00:30:00+01:00", "23:30:00Z", true}, {"00:00:00", "00:00:00Z", false}, {"12:00:00.1", "12:00:00.01", false},
		}},
		{name: "gYear", specs: builtinRestriction("gYear"), filler: func(i int) string { return fmt.Sprintf("19%02d", i) }, pairs: []valueSpacePair{
			{"2000Z", "2000+00:00", true}, {"2000+01:00", "2000Z", false}, {"2000", "2000Z", false}, {"-2000", "2000", false},
		}},
		{name: "gYearMonth", specs: builtinRestriction("gYearMonth"), filler: func(i int) string { return fmt.Sprintf("1800-%02d", i+1) }, pairs: []valueSpacePair{
			{"2000-01Z", "2000-01+00:00", true}, {"2000-01", "2000-02", false},
		}},
		{name: "gMonthDay", specs: builtinRestriction("gMonthDay"), filler: func(i int) string { return fmt.Sprintf("--02-%02d", i+1) }, pairs: []valueSpacePair{
			{"--01-01Z", "--01-01+00:00", true}, {"--01-02+14:00", "--01-01-10:00", true}, {"--01-01", "--01-01Z", false},
		}},
		{name: "gDay", specs: builtinRestriction("gDay"), filler: func(i int) string { return fmt.Sprintf("---1%d", i) }, pairs: []valueSpacePair{
			{"---01Z", "---01+00:00", true}, {"---01+01:00", "---01Z", false}, {"---01", "---02", false},
		}},
		{name: "gMonth", specs: builtinRestriction("gMonth"), filler: func(i int) string { return fmt.Sprintf("--0%d", i+1) }, pairs: []valueSpacePair{
			{"--11Z", "--11+00:00", true}, {"--11", "--12", false},
		}},
		{name: "hexBinary", specs: builtinRestriction("hexBinary"), filler: func(i int) string { return fmt.Sprintf("F%dF0", i) }, pairs: []valueSpacePair{
			{"0aFF", "0AFF", true}, {"0aff", "0AFF", true}, {"0A", "0B", false}, {"", "00", false},
		}},
		{name: "base64Binary", specs: builtinRestriction("base64Binary"), filler: func(i int) string { return "QUJ" + "QRSTUVWXYZabcdef"[i:i+1] }, pairs: []valueSpacePair{
			{"QUJD", "QU JD", true}, {"QUJD", " QUJD\n", true}, {"YQ==", "Y Q = =", true}, {"QUJD", "QUJE", false},
		}},
		{name: "QName", specs: builtinRestriction("QName"), filler: func(i int) string { return fmt.Sprintf("a:f%d", i) }, pairs: []valueSpacePair{
			{"a:n", "b:n", true}, {"a:n", "c:n", false}, {"n", "a:n", false}, {"a:n", "a:m", false},
		}},
		{name: "NOTATION", specs: builtinRestriction("NOTATION"), filler: func(i int) string { return fmt.Sprintf("a:f%d", i) }, pairs: []valueSpacePair{
			{"a:n", "b:n", true}, {"a:n", "c:n", false},
		}},
		{name: "decimal list", specs: listOf(decimal), filler: func(i int) string { return fmt.Sprintf("9%d 9", i) }, pairs: []valueSpacePair{
			{"1 23", "1 23", true}, {"1 23", "12 3", false}, {"1 2", "1 2 3", false}, {"1.0 2", " 1  2.00 ", true}, {"1", "1 1", false},
		}},
		{name: "dateTime list", specs: listOf(dateTime), filler: func(i int) string { return fmt.Sprintf("1900-01-%02dT00:00:00Z", i+1) }, pairs: []valueSpacePair{
			{"2000-01-01T00:00:00Z", "2000-01-01T01:00:00+01:00", true},
			{"2000-01-01T00:00:00Z 2000-01-01T00:00:00", "2000-01-01T00:00:00Z 2000-01-01T00:00:00Z", false},
		}},
		{name: "NMTOKENS", specs: builtinRestriction("NMTOKENS"), filler: func(i int) string { return fmt.Sprintf("f%d", i) }, pairs: []valueSpacePair{
			{"a b", "a  b", true}, {"ab", "a b", false},
		}},
		// Distinct kinds whose value fields coincide: only the kind separates
		// their keys.
		{name: "temporal union", specs: func(*testing.T) []TypeSpec {
			gYear, _ := BuiltinTypeID("gYear")
			date, _ := BuiltinTypeID("date")
			return []TypeSpec{{
				Variety: Union, Primitive: PrimitiveString, Union: []TypeID{dateTime, date, gYear},
				Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType,
			}}
		}, filler: func(i int) string { return fmt.Sprintf("19%02d", i) }, pairs: []valueSpacePair{
			{"2000-01-01T00:00:00", "2000-01-01", false}, {"2000-01-01", "2000", false}, {"2000-01-01Z", "2000-01-01+00:00", true},
		}},
		// Mixed with decimal, base64 groups are value-keyed rather than
		// text-keyed.
		{name: "base64 union", specs: func(*testing.T) []TypeSpec {
			base64, _ := BuiltinTypeID("base64Binary")
			return []TypeSpec{{
				Variety: Union, Primitive: PrimitiveString, Union: []TypeID{decimal, base64},
				Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType,
			}}
		}, filler: func(i int) string { return fmt.Sprintf("%d.5", i) }, pairs: []valueSpacePair{
			{"AQID", "AQ ID", true}, {"AQID", "AQIE", false}, {"1.0", "1", true}, {"QUJD", "QUJ D", true},
		}},
		{name: "union", specs: func(*testing.T) []TypeSpec {
			return []TypeSpec{{
				Variety: Union, Primitive: PrimitiveString, Union: []TypeID{dateTime, decimal, builtinString},
				Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType,
			}}
		}, filler: func(i int) string { return fmt.Sprintf("s%d", i) }, pairs: []valueSpacePair{
			{"2000.0", "2000", true}, {"2000-01-01T00:00:00Z", "2000-01-01T01:00:00+01:00", true},
			{"2000-01-01T00:00:00Z", "2000-01-01T00:00:00", false}, {"x", "x", true}, {"2000.0", "2000.0x", false},
			{"1", "01", true}, {"1", "x1", false},
		}},
	}
}

// TestEnumerationValueSpaceIndex checks that membership follows XSD value-space
// equality on both sides of the index threshold, for every placement of the
// literal, with and without the value's canonical projections. The partner
// group places an unequal literal before the input's own, so a key that merged
// distinct values would hide the input's literal.
func TestEnumerationValueSpaceIndex(t *testing.T) {
	t.Parallel()
	needs := []struct {
		name  string
		needs Needs
	}{{"none", 0}, {"canonical", NeedCanonical | NeedIdentity}}
	sizes := []struct {
		name    string
		n       int
		indexed bool
	}{
		{"scanned", enumerationIndexMinLiterals - 1, false},
		{"indexed", enumerationIndexMinLiterals + 1, true},
	}
	for _, kind := range valueSpaceKinds(t) {
		for _, size := range sizes {
			t.Run(kind.name+"/"+size.name, func(t *testing.T) {
				t.Parallel()
				fillers := make([]string, size.n-1)
				for i := range fillers {
					fillers[i] = kind.filler(i)
				}
				for _, pair := range kind.pairs {
					groups := map[string]struct {
						literals []string
						inputs   map[string]bool
					}{
						"first":   {append([]string{pair.literal}, fillers...), map[string]bool{pair.literal: true, pair.input: pair.equal}},
						"middle":  {splice(fillers, len(fillers)/2, pair.literal), map[string]bool{pair.literal: true, pair.input: pair.equal}},
						"last":    {append(append([]string(nil), fillers...), pair.literal), map[string]bool{pair.literal: true, pair.input: pair.equal}},
						"partner": {append(append([]string{pair.literal}, fillers[1:]...), pair.input), map[string]bool{pair.literal: true, pair.input: true}},
					}
					for placement, group := range groups {
						program, id := valueSpaceEnumeration(t, kind, group.literals)
						if got := enumerationGroupIndexed(t, program, id); got != size.indexed {
							t.Fatalf("group of %d literals indexed = %v, want %v", size.n, got, size.indexed)
						}
						for input, want := range group.inputs {
							for _, need := range needs {
								for _, validator := range valueSpaceValidators {
									err := validator.validate(program, id, input, need.needs)
									switch {
									case want && err != nil:
										t.Errorf("%s %q: %s(%q) needs=%s error = %v, want match", placement, group.literals, validator.name, input, need.name, err)
									case !want && !errors.Is(err, ErrFacet):
										t.Errorf("%s %q: %s(%q) needs=%s error = %v, want ErrFacet", placement, group.literals, validator.name, input, need.name, err)
									}
								}
							}
						}
					}
				}
			})
		}
	}
}

func splice(values []string, at int, value string) []string {
	out := append([]string(nil), values[:at]...)
	out = append(out, value)
	return append(out, values[at:]...)
}

func valueSpaceEnumeration(t *testing.T, kind valueSpaceKind, literals []string) (*Program, TypeID) {
	t.Helper()
	specs := kind.specs(t)
	restricted := specs[len(specs)-1]
	if len(specs) == 1 && restricted.Variety != Atomic {
		// A list or union declared here is the base; the enumeration restricts it.
		specs = append(specs, restricted)
	}
	restricted = specs[len(specs)-1]
	restricted.Facets.Enumeration = make([]LiteralSpec, len(literals))
	for i, literal := range literals {
		restricted.Facets.Enumeration[i] = LiteralSpec{Lexical: literal, Type: NoType, Resolver: valueSpaceResolver}
	}
	specs[len(specs)-1] = restricted
	program, ids := testEnumerationChain(t, specs...)
	return program, ids[len(ids)-1]
}

// enumerationGroupIndexed reports whether id's innermost enumeration group has
// an index.
func enumerationGroupIndexed(t *testing.T, program *Program, id TypeID) bool {
	t.Helper()
	typ, ok := program.typeDef(id)
	if !ok || len(typ.facets.enumGroups) == 0 {
		t.Fatalf("type %d has no enumeration group", id)
	}
	return typ.facets.enumGroups[len(typ.facets.enumGroups)-1].scheme != enumerationScan
}

var valueSpaceValidators = []struct {
	name     string
	validate func(p *Program, id TypeID, input string, needs Needs) error
}{
	{name: "Validate", validate: func(p *Program, id TypeID, input string, needs Needs) error {
		_, err := p.Validate(id, input, valueSpaceResolver, needs, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
		return err
	}},
	{name: "ValidateAdmitted", validate: func(p *Program, id TypeID, input string, needs Needs) error {
		_, err := p.ValidateAdmitted(id, input, valueSpaceResolver, needs, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
		return err
	}},
	{name: "ValidateBytes", validate: func(p *Program, id TypeID, input string, needs Needs) error {
		_, err := p.ValidateBytes(id, []byte(input), valueSpaceResolver, needs, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
		return err
	}},
}

// TestEnumerationValueKeyLookupAllocatesNothingForLongKeys pins a value-keyed
// hit to zero allocations when the probe key exceeds the runtime's 32-byte
// conversion buffer but fits enumerationKeyBuffer.
func TestEnumerationValueKeyLookupAllocatesNothingForLongKeys(t *testing.T) {
	const namespace = "urn:example:enumeration:long-namespace:0123456"
	resolver := Resolver{QName: func(lexical string) (ExpandedName, bool) {
		prefix, local, ok := strings.Cut(lexical, ":")
		if !ok || prefix != "a" {
			return ExpandedName{}, false
		}
		return ExpandedName{Namespace: namespace, Local: local}, true
	}}
	specs := builtinRestriction("QName")(t)
	literals := make([]LiteralSpec, enumerationIndexMinLiterals+1)
	for i := range literals {
		literals[i] = LiteralSpec{Lexical: fmt.Sprintf("a:l%d", i), Type: NoType, Resolver: resolver}
	}
	specs[0].Facets.Enumeration = literals
	program, ids := testEnumerationChain(t, specs...)
	id := ids[len(ids)-1]
	if !enumerationGroupIndexed(t, program, id) {
		t.Fatal("QName enumeration group is not indexed")
	}
	if _, err := program.Validate(id, "a:l3", resolver, 0, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil); err != nil {
		t.Fatalf("Validate(a:l3) error = %v", err)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if _, err := program.Validate(id, "a:l3", resolver, 0, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: sequentialPatternTotal}, nil); err != nil {
			t.Fatal(err)
		}
	}); allocs != 0 {
		t.Fatalf("long-key enumeration hit allocations = %v, want 0", allocs)
	}
}
