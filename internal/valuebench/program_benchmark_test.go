package valuebench

import (
	"errors"
	"math"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkProgramPrimitiveNoFacets(b *testing.B) {
	p, id := benchmarkProgramPrimitive(b)
	benchmarkProgramValidate(b, p, id, "value", 0)
}

func BenchmarkProgramDecimalBound(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(id, valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveDecimal,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
		Facets: valuepkg.FacetSpec{
			MinInclusive: valuepkg.BoundFacet{LiteralSpec: valuepkg.LiteralSpec{Lexical: "1.5", Type: valuepkg.NoType}, Present: true},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkProgramValidate(b, p, id, "2.00", valuepkg.NeedCanonical)
}

func BenchmarkProgramList(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	item, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(item, valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	id, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(id, valuepkg.TypeSpec{
		Variety:           valuepkg.List,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          item,
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkProgramValidate(b, p, id, "one two three", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
}

func BenchmarkProgramUnion(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	stringID, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(stringID, valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	decimalID, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(decimalID, valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveDecimal,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	unionID, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(unionID, valuepkg.TypeSpec{
		Variety:           valuepkg.Union,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
		Union:             []valuepkg.TypeID{stringID, decimalID},
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkProgramValidate(b, p, unionID, "2", valuepkg.NeedCanonical)
}

func benchmarkProgramPrimitive(b *testing.B) (*valuepkg.Program, valuepkg.TypeID) {
	b.Helper()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(id, valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespacePreserve,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	return p, id
}

func benchmarkProgramValidate(b *testing.B, p *valuepkg.Program, id valuepkg.TypeID, lexical string, needs valuepkg.Needs) {
	b.Helper()
	b.ReportAllocs()
	for range b.N {
		if _, err := p.Validate(id, lexical, valuepkg.Resolver{}, needs, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReentrantQNameConstruction(b *testing.B) {
	childSpec := valuepkg.TypeSpec{Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString, Base: valuepkg.NoType, ListItem: valuepkg.NoType}
	build := func() (*valuepkg.Program, valuepkg.TypeID, valuepkg.TypeID, error) {
		builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
		if err := builder.ReserveCapacity(2); err != nil {
			return nil, valuepkg.NoType, valuepkg.NoType, err
		}
		parent, err := builder.Reserve()
		if err != nil {
			return nil, valuepkg.NoType, valuepkg.NoType, err
		}
		child, err := builder.Reserve()
		if err != nil {
			return nil, valuepkg.NoType, valuepkg.NoType, err
		}
		var callbackErr error
		parentSpec := valuepkg.TypeSpec{
			Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveQName, Base: valuepkg.NoType, ListItem: valuepkg.NoType,
			Facets: valuepkg.FacetSpec{Enumeration: []valuepkg.LiteralSpec{{Type: valuepkg.NoType, Lexical: "p:x", Resolver: valuepkg.Resolver{
				QName: func(string) (valuepkg.ExpandedName, bool) {
					callbackErr = builder.Complete(child, childSpec)
					return valuepkg.ExpandedName{Namespace: "urn:construction", Local: "x"}, callbackErr == nil
				},
			}}}},
		}
		completionErr := builder.Complete(parent, parentSpec)
		if callbackErr != nil {
			return nil, parent, child, callbackErr
		}
		if completionErr != nil {
			return nil, parent, child, completionErr
		}
		program, err := builder.Seal()
		return program, parent, child, err
	}
	check := func(program *valuepkg.Program, parent, child valuepkg.TypeID) {
		resolver := valuepkg.Resolver{QName: func(lexical string) (valuepkg.ExpandedName, bool) {
			switch lexical {
			case "p:x":
				return valuepkg.ExpandedName{Namespace: "urn:construction", Local: "x"}, true
			case "p:y":
				return valuepkg.ExpandedName{Namespace: "urn:construction", Local: "y"}, true
			}
			return valuepkg.ExpandedName{}, false
		}}
		if _, err := program.Validate(parent, "p:x", resolver, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
		_, err := program.Validate(parent, "p:y", resolver, 0, 16<<20, nil)
		if !errors.Is(err, valuepkg.ErrFacet) || err.Error() != "enumeration facet failed" {
			b.Fatalf("Validate(p:y) = %v; want enumeration rejection", err)
		}
		got, err := program.Validate(child, "child", valuepkg.Resolver{}, valuepkg.NeedCanonical, 16<<20, nil)
		if err != nil || got.Type() != child || got.CanonicalText() != "child" {
			b.Fatalf("child validation = %v, %v", got, err)
		}
	}
	program, parent, child, err := build()
	if err != nil {
		b.Fatal(err)
	}
	check(program, parent, child)
	b.ReportAllocs()
	for b.Loop() {
		program, parent, child, err = build()
		if err != nil {
			b.Fatal(err)
		}
	}
	check(program, parent, child)
}

// BenchmarkFacetPolicyRejectedAdmission measures four independent failed completions per operation.
func BenchmarkFacetPolicyRejectedAdmission(b *testing.B) {
	atomic := func(primitive valuepkg.PrimitiveKind, facets valuepkg.FacetSpec) valuepkg.TypeSpec {
		return valuepkg.TypeSpec{
			Variety: valuepkg.Atomic, Primitive: primitive,
			Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
			Base: valuepkg.NoType, ListItem: valuepkg.NoType, Facets: facets,
		}
	}
	cases := []struct {
		spec    valuepkg.TypeSpec
		message string
	}{
		{
			spec:    atomic(valuepkg.PrimitiveDecimal, valuepkg.FacetSpec{Length: valuepkg.CardinalityFacet{Value: 1, Present: true}}),
			message: "length facets require a length-capable type",
		},
		{
			spec:    atomic(valuepkg.PrimitiveString, valuepkg.FacetSpec{MinInclusive: valuepkg.BoundFacet{Lexical: "x", Type: valuepkg.NoType, Present: true}}),
			message: "ordered facets require an ordered atomic type",
		},
		{
			spec:    atomic(valuepkg.PrimitiveString, valuepkg.FacetSpec{TotalDigits: valuepkg.CardinalityFacet{Value: 1, Present: true}}),
			message: "digit facets require atomic decimal type",
		},
		{
			spec: valuepkg.TypeSpec{
				Variety: valuepkg.Union, Primitive: valuepkg.PrimitiveString,
				Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
				Base: valuepkg.NoType, ListItem: valuepkg.NoType,
				Union:  []valuepkg.TypeID{valuepkg.BuiltinType(valuepkg.PrimitiveString)},
				Facets: valuepkg.FacetSpec{Present: valuepkg.FacetWhiteSpace},
			},
			message: "facet is not allowed for simple type",
		},
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, test := range cases {
			builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				b.Fatal(err)
			}
			err = builder.Complete(id, test.spec)
			if !errors.Is(err, valuepkg.ErrFacet) || err.Error() != test.message {
				b.Fatalf("Complete() = %v; want %q", err, test.message)
			}
		}
	}
}

type floatPolicyRow struct {
	id, selected               valuepkg.TypeID
	input, canonical, identity string
	raw                        []byte
	primitive                  valuepkg.PrimitiveKind
	width                      int
	bits                       uint64
}

type floatPolicyFixture struct {
	name    string
	program *valuepkg.Program
	needs   valuepkg.Needs
	rows    []floatPolicyRow
}

func floatPolicyFiniteRows(kind valuepkg.PrimitiveKind) []floatPolicyRow {
	id := valuepkg.BuiltinType(kind)
	rows := []floatPolicyRow{
		{input: "1", canonical: "1.0E0"}, {input: "1.5", canonical: "1.5E0"},
		{input: "0.1", canonical: "1.0E-1"}, {input: "-0.25", canonical: "-2.5E-1"},
		{input: "1e-45", canonical: "1.0E-45"}, {input: "3.4028235e38", canonical: "3.4028235E38"},
	}
	bits := [6]uint64{0x3f800000, 0x3fc00000, 0x3dcccccd, 0xbe800000, 1, 0x7f7fffff}
	width := 32
	if kind == valuepkg.PrimitiveDouble {
		width = 64
		rows[4].input, rows[4].canonical = "5e-324", "5.0E-324"
		rows[5].input, rows[5].canonical = "1.7976931348623157e308", "1.7976931348623157E308"
		bits = [6]uint64{0x3ff0000000000000, 0x3ff8000000000000, 0x3fb999999999999a, 0xbfd0000000000000, 1, 0x7fefffffffffffff}
	}
	for i := range rows {
		rows[i].id, rows[i].selected = id, valuepkg.NoType
		rows[i].primitive, rows[i].width, rows[i].bits = kind, width, bits[i]
	}
	return rows
}

func floatPolicyIdentityRows(kind valuepkg.PrimitiveKind) []floatPolicyRow {
	finite := floatPolicyFiniteRows(kind)
	rows := []floatPolicyRow{finite[5], finite[4]}
	prefix, overflow, underflow := "\x03\x1e", "1e39", "1e-50"
	posInf, negInf, negZero := uint64(0x7f800000), uint64(0xff800000), uint64(0x80000000)
	if kind == valuepkg.PrimitiveDouble {
		prefix, overflow, underflow = "\x04\x1e", "1e9999", "1e-9999"
		posInf, negInf, negZero = 0x7ff0000000000000, 0xfff0000000000000, 0x8000000000000000
	}
	for _, row := range []floatPolicyRow{
		{input: overflow, canonical: "INF", bits: posInf}, {input: "-" + overflow, canonical: "-INF", bits: negInf},
		{input: underflow, canonical: "0.0E0"}, {input: "-" + underflow, canonical: "0.0E0", bits: negZero},
		{input: "0", canonical: "0.0E0"}, {input: "-0", canonical: "0.0E0", bits: negZero},
		{input: "INF", canonical: "INF", bits: posInf}, {input: "-INF", canonical: "-INF", bits: negInf},
		{input: "NaN", canonical: "NaN"}, finite[0],
	} {
		row.id, row.selected = finite[0].id, valuepkg.NoType
		row.primitive, row.width = kind, finite[0].width
		rows = append(rows, row)
	}
	for i := range rows {
		rows[i].identity = prefix + rows[i].canonical
		rows[i].canonical = ""
	}
	return rows
}

func completeFloatPolicyType(tb testing.TB, builder *valuepkg.Builder, spec valuepkg.TypeSpec) valuepkg.TypeID {
	tb.Helper()
	id, err := builder.Reserve()
	if err != nil {
		tb.Fatal(err)
	}
	if err := builder.Complete(id, spec); err != nil {
		tb.Fatal(err)
	}
	return id
}

func floatPolicyFixtures(tb testing.TB) []floatPolicyFixture {
	tb.Helper()
	program, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		tb.Fatal(err)
	}
	var fixtures []floatPolicyFixture
	for _, entry := range []struct {
		name string
		kind valuepkg.PrimitiveKind
	}{
		{"Float32", valuepkg.PrimitiveFloat}, {"Float64", valuepkg.PrimitiveDouble},
	} {
		fixtures = append(fixtures, floatPolicyFixture{entry.name + "Canonical", program, valuepkg.NeedCanonical, floatPolicyFiniteRows(entry.kind)})
		fixtures = append(fixtures, floatPolicyFixture{entry.name + "Identity", program, valuepkg.NeedIdentity, floatPolicyIdentityRows(entry.kind)})
		rows := floatPolicyFiniteRows(entry.kind)
		rows[0].input = "1." + strings.Repeat("0", 1024)
		for i := range rows {
			rows[i].raw = []byte(rows[i].input)
			if len(rows[i].raw) > 1026 {
				tb.Fatal("borrowed float vector exceeds byte bound")
			}
			rows[i].canonical = ""
		}
		fixtures = append(fixtures, floatPolicyFixture{entry.name + "BorrowedUnprojected", program, 0, rows})
	}
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	decimal := completeFloatPolicyType(tb, builder, valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveDecimal, Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
		Facets: valuepkg.FacetSpec{MinInclusive: valuepkg.BoundFacet{Lexical: "2", Type: valuepkg.NoType, Present: true}},
	})
	var lists, unions []floatPolicyRow
	for _, entry := range []struct {
		kind   valuepkg.PrimitiveKind
		prefix string
	}{
		{valuepkg.PrimitiveFloat, "\x03\x1e"}, {valuepkg.PrimitiveDouble, "\x04\x1e"},
	} {
		item := valuepkg.BuiltinType(entry.kind)
		list := completeFloatPolicyType(tb, builder, valuepkg.TypeSpec{Variety: valuepkg.List, Base: valuepkg.NoType, ListItem: item, Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true})
		union := completeFloatPolicyType(tb, builder, valuepkg.TypeSpec{Variety: valuepkg.Union, Base: valuepkg.NoType, ListItem: valuepkg.NoType, Union: []valuepkg.TypeID{decimal, item}, Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true})
		lists = append(lists, floatPolicyRow{id: list, selected: valuepkg.NoType, input: "1 1.5", canonical: "1.0E0 1.5E0", identity: "\x00\x1e7:" + entry.prefix + "1.0E07:" + entry.prefix + "1.5E0"})
		unions = append(unions, floatPolicyRow{id: union, selected: item, input: "1.5", canonical: "1.5E0", identity: entry.prefix + "1.5E0"})
	}
	composed, err := builder.Seal()
	if err != nil {
		tb.Fatal(err)
	}
	fixtures = append(fixtures, floatPolicyFixture{"FloatList", composed, valuepkg.NeedCanonical | valuepkg.NeedIdentity, lists}, floatPolicyFixture{"FloatUnion", composed, valuepkg.NeedCanonical | valuepkg.NeedIdentity, unions})
	return fixtures
}

func checkFloatPolicyResult(tb testing.TB, got valuepkg.Value, row floatPolicyRow) {
	tb.Helper()
	if got.Type() != row.id || got.SelectedType() != row.selected || got.CanonicalText() != row.canonical || got.IdentityKey() != row.identity || got.HasQualifiedNames() || got.IDs() != "" || got.IDRefs() != "" {
		tb.Fatalf("input %q: type=%d selected=%d canonical=%q identity=%q", row.input, got.Type(), got.SelectedType(), got.CanonicalText(), got.IdentityKey())
	}
}

func checkFloatPolicyBits(tb testing.TB, row floatPolicyRow) {
	tb.Helper()
	if row.width == 0 {
		return
	}
	parsed, err := valuepkg.ParseFloatValue(row.primitive, row.input, 0)
	if err != nil {
		tb.Fatal(err)
	}
	if row.input == "NaN" {
		if !math.IsNaN(parsed.Value) {
			tb.Fatal("NaN not preserved")
		}
		return
	}
	bits := math.Float64bits(parsed.Value)
	want := row.bits
	if row.width == 32 {
		if row.bits > math.MaxUint32 {
			tb.Fatal("float32 literal bit oracle exceeds uint32")
			return
		}
		want = math.Float64bits(float64(math.Float32frombits(uint32(row.bits))))
	}
	if bits != want {
		tb.Fatalf("input %q widened bits=%x want=%x", row.input, bits, want)
	}
}

func checkFloatPolicyFixture(tb testing.TB, fixture floatPolicyFixture) {
	tb.Helper()
	if len(fixture.rows) == 0 || len(fixture.rows) > 12 {
		tb.Fatal("invalid fixed batch size")
	}
	var scratch valuepkg.Scratch
	for _, row := range fixture.rows {
		var got valuepkg.Value
		var err error
		if row.raw != nil {
			got, err = fixture.program.ValidateBytes(row.id, row.raw, valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch)
		} else {
			got, err = fixture.program.Validate(row.id, row.input, valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch)
		}
		if err != nil {
			tb.Fatal(err)
		}
		checkFloatPolicyResult(tb, got, row)
		checkFloatPolicyBits(tb, row)
	}
	for _, row := range fixture.rows {
		if _, err := fixture.program.Validate(row.id, "INX", valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch); err == nil {
			tb.Fatal("malformed lexical accepted")
		}
		got, err := fixture.program.Validate(row.id, row.input, valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch)
		if err != nil {
			tb.Fatal(err)
		}
		checkFloatPolicyResult(tb, got, row)
	}
	if fixture.name == "FloatList" {
		if _, err := fixture.program.Validate(fixture.rows[0].id, "1 INX", valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch); err == nil || err.Error() != "invalid float" {
			tb.Fatalf("list member rejection: %v", err)
		}
		got, err := fixture.program.Validate(fixture.rows[0].id, fixture.rows[0].input, valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch)
		if err != nil {
			tb.Fatal(err)
		}
		checkFloatPolicyResult(tb, got, fixture.rows[0])
	}
}

func BenchmarkFloatCanonicalPolicy(b *testing.B) {
	for _, fixture := range floatPolicyFixtures(b) {
		b.Run(fixture.name, func(b *testing.B) {
			checkFloatPolicyFixture(b, fixture)
			var scratch valuepkg.Scratch
			var outputs [12]valuepkg.Value
			b.ReportAllocs()
			for b.Loop() {
				for i, row := range fixture.rows {
					var err error
					if row.raw != nil {
						outputs[i], err = fixture.program.ValidateBytes(row.id, row.raw, valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch)
					} else {
						outputs[i], err = fixture.program.Validate(row.id, row.input, valuepkg.Resolver{}, fixture.needs, 16<<20, &scratch)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			}
			for i, row := range fixture.rows {
				checkFloatPolicyResult(b, outputs[i], row)
			}
			checkFloatPolicyFixture(b, fixture)
		})
	}
}

type admittedOrderedRow struct {
	id             valuepkg.TypeID
	input, message string
}

type admittedOrderedFixture struct {
	name    string
	program *valuepkg.Program
	rows    [6]admittedOrderedRow
}

func admittedOrderedSpec(primitive valuepkg.PrimitiveKind, base valuepkg.TypeID, facets valuepkg.FacetSpec) valuepkg.TypeSpec {
	return valuepkg.TypeSpec{Variety: valuepkg.Atomic, Primitive: primitive,
		Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
		Base: base, ListItem: valuepkg.NoType, Facets: facets}
}

//nolint:revive // Fixedness is facet input data in this fixture.
func admittedOrderedFacet(flag valuepkg.FacetMask, lexical string, fixed bool) valuepkg.FacetSpec {
	bound := valuepkg.BoundFacet{Lexical: lexical, Type: valuepkg.NoType, Present: true}
	var facets valuepkg.FacetSpec
	//nolint:exhaustive // These fixtures use the four ordered facet directions.
	switch flag {
	case valuepkg.FacetMinInclusive:
		facets.MinInclusive = bound
	case valuepkg.FacetMinExclusive:
		facets.MinExclusive = bound
	case valuepkg.FacetMaxInclusive:
		facets.MaxInclusive = bound
	case valuepkg.FacetMaxExclusive:
		facets.MaxExclusive = bound
	}
	if fixed {
		facets.Fixed = flag
	}
	return facets
}

//nolint:thelper // This helper runs inside measured construction without test-stack tracking.
func completeAdmittedOrdered(tb testing.TB, builder *valuepkg.Builder, spec valuepkg.TypeSpec) valuepkg.TypeID {
	id, err := builder.Reserve()
	if err != nil {
		tb.Fatal(err)
	}
	if err := builder.Complete(id, spec); err != nil {
		tb.Fatal(err)
	}
	return id
}

func admittedOrderedFixtures(tb testing.TB) [4]admittedOrderedFixture {
	tb.Helper()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	inclusive := admittedOrderedFacet(valuepkg.FacetMinInclusive, "1.5", false)
	inclusive.MaxInclusive = admittedOrderedFacet(valuepkg.FacetMaxInclusive, "2.5", false).MaxInclusive
	inc := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, valuepkg.NoType, inclusive))
	exclusive := admittedOrderedFacet(valuepkg.FacetMinExclusive, "1.5", false)
	exclusive.MaxExclusive = admittedOrderedFacet(valuepkg.FacetMaxExclusive, "2.5", false).MaxExclusive
	exc := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, valuepkg.NoType, exclusive))
	timeLower := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDateTime, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMinInclusive, "2000-01-01T12:00:00Z", false)))
	timeUpper := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDateTime, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMaxInclusive, "2000-01-01T12:00:00Z", false)))
	durationLower := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDuration, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMinInclusive, "P365D", false)))
	durationUpper := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDuration, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMaxInclusive, "P365D", false)))
	program, err := builder.Seal()
	if err != nil {
		tb.Fatal(err)
	}
	return [4]admittedOrderedFixture{
		{"DecimalInclusive", program, [6]admittedOrderedRow{
			{inc, "1.5", ""}, {inc, "2", ""}, {inc, "2.5", ""},
			{inc, "1.49", "lower bound facet failed"}, {inc, "2.51", "upper bound facet failed"}, {inc, "0", "lower bound facet failed"},
		}},
		{"DecimalExclusive", program, [6]admittedOrderedRow{
			{exc, "1.5", "lower bound facet failed"}, {exc, "2", ""}, {exc, "2.5", "upper bound facet failed"},
			{exc, "1.49", "lower bound facet failed"}, {exc, "2.51", "upper bound facet failed"}, {exc, "2.00", ""},
		}},
		// TimePartial times dateTime's partial order; recurring time is checked outside the loop.
		{"TimePartial", program, [6]admittedOrderedRow{
			{timeLower, "2000-01-01T12:00:00Z", ""}, {timeUpper, "2000-01-01T12:00:00Z", ""},
			{timeLower, "2000-01-01T12:00:00", "lower bound facet failed"}, {timeUpper, "2000-01-01T12:00:00", "upper bound facet failed"},
			{timeLower, "2000-01-01T13:00:00Z", ""}, {timeUpper, "2000-01-01T11:00:00Z", ""},
		}},
		{"DurationPartial", program, [6]admittedOrderedRow{
			{durationLower, "P365D", ""}, {durationUpper, "P365D", ""},
			{durationLower, "P1Y", "lower bound facet failed"}, {durationUpper, "P1Y", "upper bound facet failed"},
			{durationLower, "P366D", ""}, {durationUpper, "P364D", ""},
		}},
	}
}

func checkAdmittedOrderedResult(tb testing.TB, got valuepkg.Value, id valuepkg.TypeID) {
	tb.Helper()
	if got.Type() != id || got.SelectedType() != valuepkg.NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
		tb.Fatalf("unprojected typed result = %#v", got)
	}
}

func checkAdmittedOrderedRow(tb testing.TB, program *valuepkg.Program, row admittedOrderedRow, scratch *valuepkg.Scratch) {
	tb.Helper()
	got, err := program.Validate(row.id, row.input, valuepkg.Resolver{}, 0, 16<<20, scratch)
	checkAdmittedOrderedOutcome(tb, got, err, row)
}

func checkAdmittedOrderedOutcome(tb testing.TB, got valuepkg.Value, err error, row admittedOrderedRow) {
	tb.Helper()
	if row.message != "" {
		if err == nil || err.Error() != row.message || !errors.Is(err, valuepkg.ErrFacet) {
			tb.Fatalf("Validate(%q) = %v, want ErrFacet with %q", row.input, err, row.message)
		}
		return
	}
	if err != nil {
		tb.Fatalf("Validate(%q) = %v", row.input, err)
	}
	checkAdmittedOrderedResult(tb, got, row.id)
}

func checkAdmittedOrderedFixture(tb testing.TB, fixture admittedOrderedFixture, scratch *valuepkg.Scratch) {
	tb.Helper()
	var recovery admittedOrderedRow
	for _, row := range fixture.rows {
		if row.message == "" {
			recovery = row
			break
		}
	}
	for _, row := range fixture.rows {
		checkAdmittedOrderedRow(tb, fixture.program, row, scratch)
		if row.message != "" {
			checkAdmittedOrderedRow(tb, fixture.program, recovery, scratch)
		}
	}
}

func BenchmarkAdmittedOrderedBounds(b *testing.B) {
	for _, fixture := range admittedOrderedFixtures(b) {
		b.Run(fixture.name, func(b *testing.B) {
			var scratch valuepkg.Scratch
			checkAdmittedOrderedFixture(b, fixture, &scratch)
			var outputs [6]valuepkg.Value
			var outcomes [6]error
			b.ReportAllocs()
			for b.Loop() {
				for i, row := range fixture.rows {
					outputs[i], outcomes[i] = fixture.program.Validate(row.id, row.input, valuepkg.Resolver{}, 0, 16<<20, &scratch)
					if (outcomes[i] != nil) != (row.message != "") {
						b.Fatalf("Validate(%q) unexpected error presence: %v", row.input, outcomes[i])
					}
				}
			}
			for i, row := range fixture.rows {
				checkAdmittedOrderedOutcome(b, outputs[i], outcomes[i], row)
			}
			checkAdmittedOrderedFixture(b, fixture, &scratch)
		})
	}
}

//nolint:thelper // This helper runs inside measured construction without test-stack tracking.
func checkAdmittedOrderedCompletionError(tb testing.TB, err error, message string) {
	if err == nil || err.Error() != message || !errors.Is(err, valuepkg.ErrFacet) {
		tb.Fatalf("Complete() = %v, want ErrFacet with %q", err, message)
	}
}

//nolint:revive,thelper // Widening selects the untimed opponent; test-stack tracking stays outside measurement.
func buildAdmittedOrderedBatch(tb testing.TB, widening bool) ([4]*valuepkg.Program, [4]valuepkg.TypeID) {
	var programs [4]*valuepkg.Program
	var leaves [4]valuepkg.TypeID
	for i, flag := range [4]valuepkg.FacetMask{valuepkg.FacetMinInclusive, valuepkg.FacetMinExclusive, valuepkg.FacetMaxInclusive, valuepkg.FacetMaxExclusive} {
		older, middle, changed, equivalent := "1", "2", "3", "2.0"
		message := "fixed ordered facet cannot change"
		if flag == valuepkg.FacetMaxInclusive || flag == valuepkg.FacetMaxExclusive {
			older, middle, changed, equivalent = "10", "9", "8", "9.00"
			if widening {
				changed, message = "11", "upper ordered facet cannot loosen base restriction"
			}
		} else if widening {
			changed, message = "0", "lower ordered facet cannot loosen base restriction"
		}
		builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
		base := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, valuepkg.NoType, admittedOrderedFacet(flag, older, false)))
		fixed := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, base, admittedOrderedFacet(flag, middle, true)))
		omitted := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, fixed, valuepkg.FacetSpec{}))
		leaf, err := builder.Reserve()
		if err != nil {
			tb.Fatal(err)
		}
		err = builder.Complete(leaf, admittedOrderedSpec(valuepkg.PrimitiveDecimal, omitted, admittedOrderedFacet(flag, changed, false)))
		checkAdmittedOrderedCompletionError(tb, err, message)
		if retryErr := builder.Complete(leaf, admittedOrderedSpec(valuepkg.PrimitiveDecimal, omitted, admittedOrderedFacet(flag, equivalent, false))); retryErr != nil {
			tb.Fatal(retryErr)
		}
		program, err := builder.Seal()
		if err != nil {
			tb.Fatal(err)
		}
		programs[i], leaves[i] = program, leaf
	}
	return programs, leaves
}

func checkAdmittedOrderedBatch(tb testing.TB, programs [4]*valuepkg.Program, ids [4]valuepkg.TypeID) {
	tb.Helper()
	for i, program := range programs {
		var scratch valuepkg.Scratch
		rows := [3]admittedOrderedRow{{ids[i], "2", ""}, {ids[i], "1", "lower bound facet failed"}, {ids[i], "3", ""}}
		switch i {
		case 1:
			rows = [3]admittedOrderedRow{{ids[i], "2", "lower bound facet failed"}, {ids[i], "3", ""}, {ids[i], "8", ""}}
		case 2:
			rows = [3]admittedOrderedRow{{ids[i], "9", ""}, {ids[i], "10", "upper bound facet failed"}, {ids[i], "8", ""}}
		case 3:
			rows = [3]admittedOrderedRow{{ids[i], "9", "upper bound facet failed"}, {ids[i], "3", ""}, {ids[i], "8", ""}}
		}
		for _, row := range rows {
			checkAdmittedOrderedRow(tb, program, row, &scratch)
			if row.message != "" {
				checkAdmittedOrderedRow(tb, program, rows[2], &scratch)
			}
		}
	}
}

func BenchmarkAdmittedOrderedBoundConstruction(b *testing.B) {
	programs, ids := buildAdmittedOrderedBatch(b, true)
	checkAdmittedOrderedBatch(b, programs, ids)
	programs, ids = buildAdmittedOrderedBatch(b, false)
	checkAdmittedOrderedBatch(b, programs, ids)
	b.ReportAllocs()
	for b.Loop() {
		programs, ids = buildAdmittedOrderedBatch(b, false)
	}
	checkAdmittedOrderedBatch(b, programs, ids)
}
