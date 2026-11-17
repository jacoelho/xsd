package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

// valueRelation is the relation the Datatypes Recommendation and
// docs/spec/INTERPRETATIONS.md declare between the values of two lexical
// forms.
type valueRelation uint8

const (
	// relEqual: the forms denote one value.
	relEqual valueRelation = iota + 1
	// relLess and relGreater: distinct values, the first before or after the
	// second in the order relation.
	relLess
	relGreater
	// relIncomparable: distinct values with no order between them. In an
	// ordered primitive neither satisfies a bound spelled with the other.
	relIncomparable
	// relOrderEqual: distinct values that the order does not separate
	// (Duration equality and order in docs/spec/INTERPRETATIONS.md): each
	// satisfies an inclusive bound spelled with the other and no exclusive one.
	relOrderEqual
)

func (r valueRelation) String() string {
	switch r {
	case relEqual:
		return "equal"
	case relLess:
		return "less"
	case relGreater:
		return "greater"
	case relIncomparable:
		return "incomparable"
	case relOrderEqual:
		return "order-equal but distinct"
	}
	return "unknown"
}

type valueLawPair struct {
	a, b string
	rel  valueRelation
}

// valueLawTable declares relations between lexical forms of one built-in
// type. ordered says the bound facets apply to it and noEnumeration that the
// enumeration facet does not (Datatypes §4.1.5); contextual says its canonical
// text needs a namespace context to be read back.
type valueLawTable struct {
	builtin       string
	pairs         []valueLawPair
	ordered       bool
	noEnumeration bool
	contextual    bool
}

var valueLawTables = []valueLawTable{
	{builtin: "string", pairs: []valueLawPair{
		{"a", "a", relEqual},
		{"a", "A", relIncomparable},
		{" a", "a", relIncomparable},
		{"", " ", relIncomparable},
	}},
	{builtin: "boolean", noEnumeration: true, pairs: []valueLawPair{
		{"true", "1", relEqual},
		{"false", "0", relEqual},
		{"true", "false", relIncomparable},
		{"1", "0", relIncomparable},
	}},
	{builtin: "decimal", ordered: true, pairs: []valueLawPair{
		{"1.0", "1", relEqual},
		{"+1", "1", relEqual},
		{"01.50", "1.5", relEqual},
		{"-0", "0", relEqual},
		{"-0.0", "+0", relEqual},
		{".5", "0.50", relEqual},
		{"1", "2", relLess},
		{"-1", "-2", relGreater},
		{"0.1", "0.10000000000000000000000000001", relLess},
		{"-0.1", "0", relLess},
	}},
	{builtin: "integer", ordered: true, pairs: []valueLawPair{
		{"+1", "1", relEqual},
		{"007", "7", relEqual},
		{"-0", "0", relEqual},
		{"9", "10", relLess},
		{"-9", "-10", relGreater},
		{"123456789012345678901234567890", "123456789012345678901234567891", relLess},
	}},
	{builtin: "float", ordered: true, pairs: floatLawPairs("1E38")},
	{builtin: "double", ordered: true, pairs: floatLawPairs("1E308")},
	{builtin: "duration", ordered: true, pairs: []valueLawPair{
		{"P1Y", "P12M", relEqual},
		{"P1D", "PT24H", relEqual},
		{"PT1M", "PT60S", relEqual},
		{"PT1.5S", "PT1.50S", relEqual},
		{"PT0S", "-PT0S", relEqual},
		{"-P1Y", "-P12M", relEqual},
		{"P400Y", "P146097D", relOrderEqual},
		{"-P400Y", "-P146097D", relOrderEqual},
		{"P1M", "P30D", relIncomparable},
		{"P1Y", "P365D", relIncomparable},
		{"P1Y", "P2Y", relLess},
		{"P1M", "P27D", relGreater},
		{"P1M", "P32D", relLess},
		{"-P1D", "PT0S", relLess},
	}},
	{builtin: "dateTime", ordered: true, pairs: []valueLawPair{
		{"2000-01-01T12:00:00Z", "2000-01-01T14:00:00+02:00", relEqual},
		{"2000-01-01T00:00:00+14:00", "1999-12-31T10:00:00Z", relEqual},
		{"2000-01-01T23:00:00-02:00", "2000-01-02T01:00:00Z", relEqual},
		{"2000-01-01T24:00:00Z", "2000-01-02T00:00:00Z", relEqual},
		{"2000-01-01T24:00:00", "2000-01-02T00:00:00", relEqual},
		{"2000-01-01T12:00:00.5Z", "2000-01-01T12:00:00.50Z", relEqual},
		{"2000-01-01T12:00:00Z", "2000-01-01T12:00:00+00:00", relEqual},
		{"2000-01-01T12:00:00Z", "2000-01-01T12:00:01Z", relLess},
		{"2000-01-01T00:00:00Z", "2000-01-01T00:00:00+01:00", relGreater},
		{"2000-01-01T12:00:00", "2000-01-01T12:00:00Z", relIncomparable},
		{"2000-01-01T12:00:00", "2000-01-03T12:00:00Z", relLess},
		{"2000-01-03T12:00:00", "2000-01-01T12:00:00Z", relGreater},
	}},
	{builtin: "time", ordered: true, pairs: []valueLawPair{
		{"00:30:00+14:00", "10:30:00Z", relEqual},
		{"23:00:00-02:00", "01:00:00Z", relEqual},
		{"23:00:00-01:00", "00:00:00Z", relEqual},
		{"12:30:00+02:00", "10:30:00Z", relEqual},
		{"12:00:00Z", "12:00:00+00:00", relEqual},
		{"12:00:00.5Z", "12:00:00.50Z", relEqual},
		{"12:00:00", "12:00:00.000", relEqual},
		{"24:00:00", "00:00:00", relGreater},
		{"24:00:00Z", "00:00:00Z", relGreater},
		{"24:00:00+00:00", "24:00:00Z", relEqual},
		{"24:00:00-00:00", "24:00:00Z", relEqual},
		{"24:00:00+00:00", "00:00:00Z", relGreater},
		{"24:00:00-01:00", "01:00:00Z", relEqual},
		{"24:00:00+01:00", "23:00:00Z", relEqual},
		{"24:00:00-01:00", "24:00:00Z", relLess},
		{"23:59:59", "24:00:00", relLess},
		{"23:59:59.999Z", "24:00:00Z", relLess},
		{"23:59:60Z", "00:00:00Z", relEqual},
		{"23:59:60Z", "24:00:00Z", relLess},
		{"10:00:00Z", "11:00:00Z", relLess},
		{"00:30:00+14:00", "10:00:00Z", relGreater},
		{"23:30:00-02:00", "02:00:00Z", relLess},
		{"12:00:00", "12:00:00Z", relIncomparable},
		{"24:00:00", "24:00:00Z", relIncomparable},
		{"00:00:00", "23:00:00Z", relLess},
		{"23:00:00", "00:00:00Z", relGreater},
	}},
	{builtin: "date", ordered: true, pairs: []valueLawPair{
		{"2000-01-01Z", "2000-01-01+00:00", relEqual},
		{"2000-01-02+14:00", "2000-01-01-10:00", relEqual},
		{"2000-01-01Z", "2000-01-02Z", relLess},
		{"2000-01-01+01:00", "2000-01-01Z", relLess},
		{"2000-01-01-01:00", "2000-01-01Z", relGreater},
		{"2000-01-01", "2000-01-01Z", relIncomparable},
		{"2000-01-01", "2000-01-03Z", relLess},
	}},
	{builtin: "gYearMonth", ordered: true, pairs: []valueLawPair{
		{"2000-01Z", "2000-01+00:00", relEqual},
		{"2000-01Z", "2000-01-05:00", relLess},
		{"2000-01+05:00", "2000-01Z", relLess},
		{"2000-01Z", "2000-02Z", relLess},
		{"2000-01", "2000-01Z", relIncomparable},
		{"2000-01", "2000-03Z", relLess},
	}},
	{builtin: "gYear", ordered: true, pairs: []valueLawPair{
		{"2000Z", "2000-00:00", relEqual},
		{"2000+01:00", "2000Z", relLess},
		{"2000-01:00", "2000Z", relGreater},
		{"1999Z", "2000Z", relLess},
		{"2000", "2000Z", relIncomparable},
		{"1999", "2001Z", relLess},
	}},
	{builtin: "gMonthDay", ordered: true, pairs: []valueLawPair{
		{"--03-01Z", "--03-01+00:00", relEqual},
		{"--03-02+14:00", "--03-01-10:00", relEqual},
		{"--03-01Z", "--03-02Z", relLess},
		{"--03-01+01:00", "--03-01Z", relLess},
		{"--03-01", "--03-01Z", relIncomparable},
		{"--03-01", "--03-05Z", relLess},
	}},
	{builtin: "gDay", ordered: true, pairs: []valueLawPair{
		{"---05Z", "---05+00:00", relEqual},
		{"---06+14:00", "---05-10:00", relEqual},
		{"---05Z", "---06Z", relLess},
		{"---05+01:00", "---05Z", relLess},
		{"---05", "---05Z", relIncomparable},
		{"---05", "---09Z", relLess},
	}},
	{builtin: "gMonth", ordered: true, pairs: []valueLawPair{
		{"--02Z", "--02+00:00", relEqual},
		{"--02Z", "--03Z", relLess},
		{"--02+01:00", "--02Z", relLess},
		{"--02-01:00", "--02Z", relGreater},
		{"--02", "--02Z", relIncomparable},
		{"--02", "--04Z", relLess},
	}},
	{builtin: "hexBinary", pairs: []valueLawPair{
		{"0aff", "0AFF", relEqual},
		{"", "", relEqual},
		{"0a", "0b", relIncomparable},
		{"0a", "0a00", relIncomparable},
	}},
	{builtin: "base64Binary", pairs: []valueLawPair{
		{"AQID", "AQ ID", relEqual},
		{"AQIDBA==", "AQID BA==", relEqual},
		{"AQIDBA==", "A Q I D B A = =", relEqual},
		{"AQID", "AQIE", relIncomparable},
		{"AQ==", "AQI=", relIncomparable},
	}},
	{builtin: "anyURI", pairs: []valueLawPair{
		{"http://example.com/a", "http://example.com/a", relEqual},
		{"http://example.com/a", "http://EXAMPLE.com/a", relIncomparable},
		{"http://example.com/%41", "http://example.com/A", relIncomparable},
		{"a", "./a", relIncomparable},
	}},
	{builtin: "QName", contextual: true, pairs: qualifiedNameLawPairs},
	{builtin: "NOTATION", contextual: true, pairs: qualifiedNameLawPairs},
}

// floatLawPairs are the float and double relations of Datatypes §3.2.4 and
// §3.2.5: one zero, NaN equal to itself and incomparable with every other
// value. large is a finite value of the type above every other form used.
func floatLawPairs(large string) []valueLawPair {
	return []valueLawPair{
		{"NaN", "NaN", relEqual},
		{"-0", "0", relEqual},
		{"-0.0E0", "+0", relEqual},
		{"1", "1.0E0", relEqual},
		{"100", "1E2", relEqual},
		{"INF", "INF", relEqual},
		{"1", "2", relLess},
		{"-1", "-0", relLess},
		{"INF", large, relGreater},
		{"-INF", "-" + large, relLess},
		{"-INF", "INF", relLess},
		{"NaN", "0", relIncomparable},
		{"NaN", "INF", relIncomparable},
		{"NaN", "-INF", relIncomparable},
	}
}

// qualifiedNameLawPairs are read with lawNamespaces: p and q are bound to one
// namespace, r to another, and unprefixed names are in no namespace.
var qualifiedNameLawPairs = []valueLawPair{
	{"p:x", "q:x", relEqual},
	{"p:x", "p:x", relEqual},
	{"x", "x", relEqual},
	{"p:x", "r:x", relIncomparable},
	{"p:x", "p:y", relIncomparable},
	{"p:x", "x", relIncomparable},
}

var lawNamespaces = map[string]string{"p": "urn:one", "q": "urn:one", "r": "urn:two"}

func lawResolver() value.Resolver {
	return value.Resolver{
		QName: func(lexical string) (value.ExpandedName, bool) {
			for i := range len(lexical) {
				if lexical[i] == ':' {
					namespace, ok := lawNamespaces[lexical[:i]]
					return value.ExpandedName{Namespace: namespace, Local: lexical[i+1:]}, ok
				}
			}
			return value.ExpandedName{Local: lexical}, true
		},
		Notation: func(string, string) bool { return true },
	}
}

func lawLimits() value.EvaluationLimits {
	return value.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}
}

func TestValueLawTablesCoverEveryPrimitive(t *testing.T) {
	t.Parallel()
	covered := make(map[value.TypeID]bool)
	for _, table := range valueLawTables {
		id, ok := value.BuiltinTypeID(table.builtin)
		if !ok {
			t.Fatalf("law table names unknown builtin %q", table.builtin)
		}
		covered[id] = true
	}
	primitives := 0
	for kind := value.PrimitiveKind(0); value.BuiltinType(kind) != value.NoType; kind++ {
		primitives++
		if id := value.BuiltinType(kind); !covered[id] {
			name, _ := value.BuiltinTypeName(id)
			t.Errorf("primitive %s has no law table", name)
		}
	}
	if primitives != int(value.PrimitiveNotation)+1 {
		t.Fatalf("visited %d primitives, want %d", primitives, int(value.PrimitiveNotation)+1)
	}
}

// TestValueSpaceLaws checks that every consumer of a value agrees with the
// declared relation of each pair: Value.Equal, the identity key, enumeration
// membership (scanned and indexed), the four bound facets and canonical text.
func TestValueSpaceLaws(t *testing.T) {
	t.Parallel()
	for _, table := range valueLawTables {
		for _, pair := range table.pairs {
			t.Run(table.builtin+"/"+pair.a+" "+pair.rel.String()+" "+pair.b, func(t *testing.T) {
				t.Parallel()
				laws := newValueLaws(t, table)
				a, b, rel := pair.a, pair.b, pair.rel
				if rel == relGreater {
					a, b, rel = b, a, relLess
				}
				laws.equality(a, b, rel)
				laws.enumeration(a, b, rel)
				laws.enumeration(b, a, rel)
				laws.canonicalRoundTrip(a)
				laws.canonicalRoundTrip(b)
				if !table.ordered {
					return
				}
				switch rel {
				case relEqual, relOrderEqual:
					laws.bounds(a, b, boundsAccept{minInclusive: true, maxInclusive: true})
					laws.bounds(b, a, boundsAccept{minInclusive: true, maxInclusive: true})
				case relLess:
					laws.bounds(b, a, boundsAccept{maxInclusive: true, maxExclusive: true})
					laws.bounds(a, b, boundsAccept{minInclusive: true, minExclusive: true})
				case relIncomparable:
					laws.bounds(a, b, boundsAccept{})
					laws.bounds(b, a, boundsAccept{})
				case relGreater:
					t.Fatal("relGreater is normalized to relLess")
				}
			})
		}
	}
}

// boundsAccept says which single bound facets, spelled with one form, accept
// the value of another.
type boundsAccept struct {
	minInclusive, maxInclusive, minExclusive, maxExclusive bool
}

type valueLaws struct {
	t       *testing.T
	table   valueLawTable
	builtin value.TypeID
}

func newValueLaws(t *testing.T, table valueLawTable) valueLaws {
	t.Helper()
	builtin, ok := value.BuiltinTypeID(table.builtin)
	if !ok {
		t.Fatalf("unknown builtin %q", table.builtin)
	}
	return valueLaws{t: t, table: table, builtin: builtin}
}

// restricted returns a sealed program holding one restriction of the table's
// builtin by facets. A facet set the builder rejects fails the test: every law
// builds types the declared relations make valid.
func (l valueLaws) restricted(description string, facets value.FacetSpec) (*value.Program, value.TypeID) {
	l.t.Helper()
	builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	base, ok := builder.TypeView(l.builtin)
	if !ok {
		l.t.Fatalf("builtin %s has no type view", l.table.builtin)
	}
	id, err := builder.Reserve()
	if err != nil {
		l.t.Fatal(err)
	}
	spec := value.TypeSpec{
		Variety: value.Atomic, Primitive: base.Primitive, Builtin: base.Builtin,
		Whitespace: base.Whitespace, Base: l.builtin, ListItem: value.NoType, Facets: facets,
	}
	if completeErr := builder.Complete(id, spec); completeErr != nil {
		l.t.Fatalf("%s restricted by %s: Complete = %v", l.table.builtin, description, completeErr)
	}
	program, err := builder.Seal()
	if err != nil {
		l.t.Fatal(err)
	}
	return program, id
}

func (l valueLaws) value(lexical string) value.Value {
	l.t.Helper()
	program, err := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62}).Seal()
	if err != nil {
		l.t.Fatal(err)
	}
	v, err := program.Validate(l.builtin, lexical, lawResolver(), value.NeedCanonical|value.NeedIdentity, lawLimits(), nil)
	if err != nil {
		l.t.Fatalf("%s value %q: %v", l.table.builtin, lexical, err)
	}
	return v
}

// accepts reports whether the type admits lexical. Only a facet failure is a
// rejection; any other error fails the test.
func (l valueLaws) accepts(program *value.Program, id value.TypeID, lexical string) bool {
	l.t.Helper()
	_, err := program.Validate(id, lexical, lawResolver(), 0, lawLimits(), nil)
	if err != nil && !errors.Is(err, value.ErrFacet) {
		l.t.Fatalf("%s value %q: %v, want nil or ErrFacet", l.table.builtin, lexical, err)
	}
	return err == nil
}

// equality checks Value.Equal in both directions and the identity keys
// against whether rel is equality.
func (l valueLaws) equality(a, b string, rel valueRelation) {
	l.t.Helper()
	equal := rel == relEqual
	left, right := l.value(a), l.value(b)
	if got := left.Equal(right); got != equal {
		l.t.Errorf("Value.Equal(%q, %q) = %v, want %v", a, b, got, equal)
	}
	if got := right.Equal(left); got != equal {
		l.t.Errorf("Value.Equal(%q, %q) = %v, want %v", b, a, got, equal)
	}
	if got := left.IdentityKey() == right.IdentityKey(); got != equal {
		l.t.Errorf("identity keys of %q and %q equal = %v, want %v (%q, %q)", a, b, got, equal, left.IdentityKey(), right.IdentityKey())
	}
}

// enumeration checks that lexical is a member of an enumeration of literal
// alone exactly when rel is equality, both scanned and, repeated to the
// indexed group size, looked up by key.
func (l valueLaws) enumeration(literal, lexical string, rel valueRelation) {
	l.t.Helper()
	if l.table.noEnumeration {
		return
	}
	member := rel == relEqual
	for _, shape := range []struct {
		name   string
		copies int
	}{{"scanned", 1}, {"indexed", 8}} {
		literals := make([]value.LiteralSpec, shape.copies)
		for i := range literals {
			literals[i] = value.LiteralSpec{Lexical: literal, Type: value.NoType, Resolver: lawResolver()}
		}
		program, id := l.restricted("enumeration "+literal, value.FacetSpec{Enumeration: literals})
		if got := l.accepts(program, id, lexical); got != member {
			l.t.Errorf("%s enumeration {%q} accepts %q = %v, want %v", shape.name, literal, lexical, got, member)
		}
		if !l.accepts(program, id, literal) {
			l.t.Errorf("%s enumeration {%q} rejects its own literal", shape.name, literal)
		}
	}
}

// bounds checks lexical against each bound facet spelled with bound.
func (l valueLaws) bounds(bound, lexical string, want boundsAccept) {
	l.t.Helper()
	literal := value.BoundFacet{Lexical: bound, Type: value.NoType, Present: true}
	for _, facet := range []struct {
		name  string
		spec  value.FacetSpec
		wants bool
	}{
		{"minInclusive", value.FacetSpec{MinInclusive: literal}, want.minInclusive},
		{"maxInclusive", value.FacetSpec{MaxInclusive: literal}, want.maxInclusive},
		{"minExclusive", value.FacetSpec{MinExclusive: literal}, want.minExclusive},
		{"maxExclusive", value.FacetSpec{MaxExclusive: literal}, want.maxExclusive},
	} {
		program, id := l.restricted(facet.name+" "+bound, facet.spec)
		if got := l.accepts(program, id, lexical); got != facet.wants {
			l.t.Errorf("%s %q accepts %q = %v, want %v", facet.name, bound, lexical, got, facet.wants)
		}
	}
	if want.minInclusive && want.maxInclusive {
		program, id := l.restricted("minInclusive and maxInclusive "+bound, value.FacetSpec{MinInclusive: literal, MaxInclusive: literal})
		if !l.accepts(program, id, lexical) {
			l.t.Errorf("minInclusive %q with maxInclusive %q rejects %q, want accepted", bound, bound, lexical)
		}
	}
}

// canonicalRoundTrip checks that the canonical text of lexical denotes the
// value of lexical, for types whose canonical text needs no context.
func (l valueLaws) canonicalRoundTrip(lexical string) {
	l.t.Helper()
	if l.table.contextual {
		return
	}
	original := l.value(lexical)
	canonical := original.CanonicalText()
	if again := l.value(canonical); !original.Equal(again) {
		l.t.Errorf("canonical text %q of %q denotes another value (identity %q, want %q)", canonical, lexical, again.IdentityKey(), original.IdentityKey())
	}
}

// valueLawGrids holds, for each law table, valid lexical forms built from
// boundary components. TestValueSpaceGridLaws relates every pair of one grid
// by laws that need no declared relation.
var valueLawGrids = map[string][]string{
	"string":       {"", " ", "a", "A", " a", "a ", "ab", "a b", "é"},
	"boolean":      {"true", "false", "1", "0"},
	"decimal":      cross([]string{"", "+", "-"}, []string{"0", "1", "01", "10"}, []string{"", ".0", ".5", ".50", ".05"}),
	"integer":      cross([]string{"", "+", "-"}, []string{"0", "00", "1", "01", "9", "10", "100", "123456789012345678901234567890"}),
	"float":        floatGrid("1E38"),
	"double":       floatGrid("1E308"),
	"duration":     cross([]string{"", "-"}, durationGridForms),
	"dateTime":     cross([]string{"1999-12-31", "2000-01-01", "2000-02-29", "2000-12-31", "2001-01-01"}, []string{"T00:00:00", "T12:00:00", "T12:00:00.5", "T23:59:59", "T24:00:00"}, []string{"", "Z", "+00:00", "+02:00", "+14:00", "-14:00"}),
	"time":         timeGrid(),
	"date":         cross([]string{"1999-12-31", "2000-01-01", "2000-01-02", "2000-02-29", "2000-03-01", "2000-12-31", "2001-01-01"}, gridTimezones),
	"gYearMonth":   cross([]string{"1999-12", "2000-01", "2000-02", "2000-12", "2001-01"}, gridTimezones),
	"gYear":        cross([]string{"-0001", "0001", "1999", "2000", "2001"}, gridTimezones),
	"gMonthDay":    cross([]string{"--01-01", "--01-02", "--02-28", "--02-29", "--03-01", "--12-31"}, gridTimezones),
	"gDay":         cross([]string{"---01", "---02", "---15", "---28", "---31"}, gridTimezones),
	"gMonth":       cross([]string{"--01", "--02", "--06", "--12"}, gridTimezones),
	"hexBinary":    {"", "00", "0a", "0A", "ff", "FF", "fF", "0aff", "0AFF", "0a00"},
	"base64Binary": {"", "AA==", "AQ==", "AQI=", "AQID", "AQ ID", "A Q I D", "AQIDBA==", "AQID BA==", "AQIDBA= ="},
	"anyURI":       {"", "a", "A", "./a", "http://example.com/a", "http://EXAMPLE.com/a", "http://example.com/%41", "http://example.com/A"},
	"QName":        {"p:x", "q:x", "r:x", "p:y", "x", "y"},
	"NOTATION":     {"p:x", "q:x", "r:x", "p:y", "x", "y"},
}

var gridTimezones = []string{"", "Z", "+00:00", "-00:00", "+02:00", "-02:00", "+14:00", "-14:00"}

var durationGridForms = []string{
	"P0D", "PT0S", "P1Y", "P12M", "P1M", "P28D", "P30D", "P31D", "P1D", "PT24H", "PT1440M", "PT86400S",
	"PT1M", "PT60S", "PT1.5S", "PT1.50S", "P400Y", "P4800M", "P146097D", "P1Y1D", "P365D", "P366D", "P1Y2M3DT4H5M6.7S",
}

func floatGrid(large string) []string {
	return []string{
		"0", "-0", "+0", "0.0", "-0.0E0", "1", "1.0", "1E0", "1e0", "10E-1", "-1", "1E1", "10", "1.5", "15E-1",
		large, "-" + large, "INF", "-INF", "NaN",
	}
}

// timeGrid crosses clocks, fractions and timezones. The clocks are the
// boundary subset of hours 00, 01, 10, 12, 23 and 24 with minutes and seconds
// 00, 30 and 59: every pair is validated against six types, so the full cross
// of about 1100 forms would take several seconds. The hour 24 admits only a
// zero fraction (Datatypes §3.2.8.1).
func timeGrid() []string {
	clocks := []string{
		"00:00:00", "00:00:30", "00:30:00", "00:59:59", "01:00:00", "10:30:00", "12:00:00", "12:30:30",
		"23:00:00", "23:30:00", "23:59:59", "23:59:60", "24:00:00",
	}
	var forms []string
	for _, clock := range clocks {
		for _, fraction := range []string{"", ".0", ".5"} {
			if clock == "24:00:00" && fraction == ".5" {
				continue
			}
			for _, zone := range gridTimezones {
				forms = append(forms, clock+fraction+zone)
			}
		}
	}
	return forms
}

func cross(parts ...[]string) []string {
	forms := []string{""}
	for _, part := range parts {
		next := make([]string, 0, len(forms)*len(part))
		for _, prefix := range forms {
			for _, suffix := range part {
				next = append(next, prefix+suffix)
			}
		}
		forms = next
	}
	return forms
}

// orderEqualDistinct reports whether the declared exception to "values the
// order ranks equal are equal" covers a pair: two durations whose month
// counts differ by a whole number of 400-year Gregorian cycles. Only then can
// the sums at the four reference dateTimes all coincide while the values
// differ (Duration equality and order in docs/spec/INTERPRETATIONS.md). No
// other primitive has an exception.
func orderEqualDistinct(builtin, a, b string) bool {
	if builtin != "duration" {
		return false
	}
	const monthsPerGregorianCycle = 400 * 12
	difference := durationMonths(a) - durationMonths(b)
	return difference != 0 && difference%monthsPerGregorianCycle == 0
}

// durationMonths reads the signed year and month count of a duration form.
func durationMonths(lexical string) int {
	sign := 1
	if lexical[0] == '-' {
		sign, lexical = -1, lexical[1:]
	}
	months, digits := 0, 0
	for i := 1; i < len(lexical) && lexical[i] != 'T'; i++ {
		switch c := lexical[i]; {
		case c >= '0' && c <= '9':
			digits = digits*10 + int(c-'0')
		case c == 'Y':
			months, digits = months+12*digits, 0
		case c == 'M':
			months, digits = months+digits, 0
		default:
			digits = 0
		}
	}
	return sign * months
}

// valueLawKnownGaps lists grid pairs excluded from a law, each with the
// reason the gap is open. An entry whose pair satisfies the law fails the
// test, so the list cannot outlive its gaps.
var valueLawKnownGaps = map[valueLawGap]string{}

type valueLawGap struct{ builtin, law, a, b string }

// observedOrder is how one form meets the four bound facets spelled with
// another.
type observedOrder uint8

const (
	meetsMinInclusive observedOrder = 1 << iota
	meetsMaxInclusive
	meetsMinExclusive
	meetsMaxExclusive
)

const (
	observedLess         = meetsMaxInclusive | meetsMaxExclusive
	observedGreater      = meetsMinInclusive | meetsMinExclusive
	observedOrderEqual   = meetsMinInclusive | meetsMaxInclusive
	observedIncomparable = observedOrder(0)
)

// observedRelations names the facet combinations that are one relation.
var observedRelations = map[observedOrder]string{
	observedLess:         "less",
	observedGreater:      "greater",
	observedOrderEqual:   "order-equal",
	observedIncomparable: "incomparable",
}

func (o observedOrder) String() string {
	if name, ok := observedRelations[o]; ok {
		return name
	}
	names := ""
	for i, name := range []string{"minInclusive", "maxInclusive", "minExclusive", "maxExclusive"} {
		if o&(1<<i) != 0 {
			names += " " + name
		}
	}
	return "inconsistent (satisfies" + names + ")"
}

func (o observedOrder) inverse() observedOrder {
	if o == observedLess {
		return observedGreater
	}
	if o == observedGreater {
		return observedLess
	}
	return o
}

func TestValueLawGridsCoverEveryTable(t *testing.T) {
	t.Parallel()
	for _, table := range valueLawTables {
		if len(valueLawGrids[table.builtin]) < 2 {
			t.Errorf("%s has no grid", table.builtin)
		}
	}
	if len(valueLawGrids) != len(valueLawTables) {
		t.Errorf("%d grids for %d law tables", len(valueLawGrids), len(valueLawTables))
	}
}

// TestValueSpaceGridLaws relates every pair of each grid through Value.Equal,
// identity keys, enumeration membership and the bound facets, and checks the
// laws that hold whatever the values are: equality is reflexive and symmetric
// and decides identity keys and enumeration membership; the order is
// consistent, antisymmetric and transitive; equal values are not separated by
// the order and meet every third value alike; values the order ranks equal
// are equal, except for orderEqualDistinct; canonical text denotes its value.
func TestValueSpaceGridLaws(t *testing.T) {
	t.Parallel()
	for _, table := range valueLawTables {
		t.Run(table.builtin, func(t *testing.T) {
			t.Parallel()
			grid := newValueLawGrid(t, table)
			grid.checkEquality()
			grid.checkCanonical()
			if table.ordered {
				grid.checkOrder()
			}
			grid.reportStaleGaps()
		})
	}
}

type valueLawGrid struct {
	t        *testing.T
	table    valueLawTable
	forms    []string
	values   []value.Value
	program  *value.Program
	scanned  []value.TypeID
	indexed  []value.TypeID
	bounds   [][4]value.TypeID
	failures map[string]int
	gapsHit  map[valueLawGap]bool
}

// maxGridFailuresPerLaw bounds the failures printed for one law; the count of
// the rest is reported once.
const maxGridFailuresPerLaw = 12

func newValueLawGrid(t *testing.T, table valueLawTable) *valueLawGrid {
	t.Helper()
	g := &valueLawGrid{t: t, table: table, forms: valueLawGrids[table.builtin], failures: map[string]int{}, gapsHit: map[valueLawGap]bool{}}
	builtin, ok := value.BuiltinTypeID(table.builtin)
	if !ok {
		t.Fatalf("unknown builtin %q", table.builtin)
	}
	builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	base, ok := builder.TypeView(builtin)
	if !ok {
		t.Fatalf("builtin %s has no type view", table.builtin)
	}
	restrict := func(description string, facets value.FacetSpec) value.TypeID {
		id, err := builder.Reserve()
		if err != nil {
			t.Fatal(err)
		}
		spec := value.TypeSpec{
			Variety: value.Atomic, Primitive: base.Primitive, Builtin: base.Builtin,
			Whitespace: base.Whitespace, Base: builtin, ListItem: value.NoType, Facets: facets,
		}
		if err := builder.Complete(id, spec); err != nil {
			t.Fatalf("%s restricted by %s: Complete = %v", table.builtin, description, err)
		}
		return id
	}
	for _, form := range g.forms {
		if !table.noEnumeration {
			literal := value.LiteralSpec{Lexical: form, Type: value.NoType, Resolver: lawResolver()}
			g.scanned = append(g.scanned, restrict("enumeration "+form, value.FacetSpec{Enumeration: []value.LiteralSpec{literal}}))
			repeated := make([]value.LiteralSpec, 8)
			for i := range repeated {
				repeated[i] = literal
			}
			g.indexed = append(g.indexed, restrict("indexed enumeration "+form, value.FacetSpec{Enumeration: repeated}))
		}
		if table.ordered {
			bound := value.BoundFacet{Lexical: form, Type: value.NoType, Present: true}
			g.bounds = append(g.bounds, [4]value.TypeID{
				restrict("minInclusive "+form, value.FacetSpec{MinInclusive: bound}),
				restrict("maxInclusive "+form, value.FacetSpec{MaxInclusive: bound}),
				restrict("minExclusive "+form, value.FacetSpec{MinExclusive: bound}),
				restrict("maxExclusive "+form, value.FacetSpec{MaxExclusive: bound}),
			})
		}
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	g.program = program
	limits := lawLimits()
	for _, form := range g.forms {
		v, err := program.Validate(builtin, form, lawResolver(), value.NeedCanonical|value.NeedIdentity, limits, nil)
		if err != nil {
			t.Fatalf("%s grid form %q: %v", table.builtin, form, err)
		}
		g.values = append(g.values, v)
	}
	return g
}

// failf reports a broken law for the pair unless a known gap covers it.
func (g *valueLawGrid) failf(law string, i, j int, format string, args ...any) {
	g.t.Helper()
	gap := valueLawGap{builtin: g.table.builtin, law: law, a: g.forms[i], b: g.forms[j]}
	if _, known := valueLawKnownGaps[gap]; known {
		g.gapsHit[gap] = true
		return
	}
	g.failures[law]++
	if g.failures[law] <= maxGridFailuresPerLaw {
		g.t.Errorf("%s %q, %q: %s: "+format, append([]any{g.table.builtin, g.forms[i], g.forms[j], law}, args...)...)
	}
}

func (g *valueLawGrid) reportStaleGaps() {
	g.t.Helper()
	for law, count := range g.failures {
		if count > maxGridFailuresPerLaw {
			g.t.Errorf("%s: %s: %d more pairs fail", g.table.builtin, law, count-maxGridFailuresPerLaw)
		}
	}
	for gap, reason := range valueLawKnownGaps {
		if gap.builtin == g.table.builtin && !g.gapsHit[gap] {
			g.t.Errorf("%s %q, %q: known gap in %s no longer fails (%s)", gap.builtin, gap.a, gap.b, gap.law, reason)
		}
	}
}

func (g *valueLawGrid) accepts(id value.TypeID, form string) bool {
	g.t.Helper()
	_, err := g.program.Validate(id, form, lawResolver(), 0, lawLimits(), nil)
	if err != nil && !errors.Is(err, value.ErrFacet) {
		g.t.Fatalf("%s value %q: %v, want nil or ErrFacet", g.table.builtin, form, err)
	}
	return err == nil
}

func (g *valueLawGrid) checkEquality() {
	g.t.Helper()
	for i := range g.forms {
		for j := range g.forms {
			equal := g.values[i].Equal(g.values[j])
			if i == j && !equal {
				g.failf("equality is reflexive", i, j, "Value.Equal = false")
			}
			if reverse := g.values[j].Equal(g.values[i]); reverse != equal {
				g.failf("equality is symmetric", i, j, "Value.Equal = %v, reversed = %v", equal, reverse)
			}
			if sameKey := g.values[i].IdentityKey() == g.values[j].IdentityKey(); sameKey != equal {
				g.failf("identity keys agree with equality", i, j, "Value.Equal = %v, identity keys equal = %v", equal, sameKey)
			}
			if g.table.noEnumeration {
				continue
			}
			if member := g.accepts(g.scanned[i], g.forms[j]); member != equal {
				g.failf("scanned enumeration agrees with equality", i, j, "Value.Equal = %v, enumeration of the first accepts the second = %v", equal, member)
			}
			if member := g.accepts(g.indexed[i], g.forms[j]); member != equal {
				g.failf("indexed enumeration agrees with equality", i, j, "Value.Equal = %v, enumeration of the first accepts the second = %v", equal, member)
			}
		}
	}
}

func (g *valueLawGrid) checkCanonical() {
	g.t.Helper()
	if g.table.contextual {
		return
	}
	builtin, _ := value.BuiltinTypeID(g.table.builtin)
	for i, form := range g.forms {
		canonical := g.values[i].CanonicalText()
		again, err := g.program.Validate(builtin, canonical, lawResolver(), value.NeedIdentity, lawLimits(), nil)
		if err != nil {
			g.t.Errorf("%s %q: canonical text %q is not a value: %v", g.table.builtin, form, canonical, err)
			continue
		}
		if !g.values[i].Equal(again) {
			g.failf("canonical text denotes its value", i, i, "canonical text %q has identity %q, want %q", canonical, again.IdentityKey(), g.values[i].IdentityKey())
		}
	}
}

func (g *valueLawGrid) checkOrder() {
	g.t.Helper()
	n := len(g.forms)
	order := make([][]observedOrder, n)
	for i := range order {
		order[i] = make([]observedOrder, n)
		for j := range order[i] {
			for facet, id := range g.bounds[j] {
				if g.accepts(id, g.forms[i]) {
					order[i][j] |= 1 << facet
				}
			}
		}
	}
	for i := range n {
		for j := range n {
			observed, reverse := order[i][j], order[j][i]
			equal := g.values[i].Equal(g.values[j])
			if _, relation := observedRelations[observed]; !relation {
				g.failf("bound facets agree on one relation", i, j, "first against bounds of the second: %v", observed)
				continue
			}
			if reverse != observed.inverse() {
				g.failf("order is antisymmetric", i, j, "first against second: %v, second against first: %v", observed, reverse)
			}
			if equal && observed != observedOrderEqual {
				g.failf("equal values are not separated by the order", i, j, "Value.Equal = true, first against second: %v, second against first: %v", observed, reverse)
			}
			if !equal && observed == observedOrderEqual && !orderEqualDistinct(g.table.builtin, g.forms[i], g.forms[j]) {
				g.failf("values the order ranks equal are equal", i, j, "Value.Equal = false, first against second: %v, second against first: %v", observed, reverse)
			}
			if !equal {
				continue
			}
			for k := range n {
				if order[i][k] != order[j][k] || order[k][i] != order[k][j] {
					g.failf("equal values meet a third value alike", i, j, "against %q: %v and %v; %q against them: %v and %v",
						g.forms[k], order[i][k], order[j][k], g.forms[k], order[k][i], order[k][j])
					break
				}
			}
		}
	}
	for i := range n {
		for j := range n {
			if order[i][j] != observedLess {
				continue
			}
			for k := range n {
				if order[j][k] == observedLess && order[i][k] != observedLess {
					g.failf("order is transitive", i, k, "both sides of %q, yet first against second: %v, second against first: %v", g.forms[j], order[i][k], order[k][i])
				}
			}
		}
	}
}
