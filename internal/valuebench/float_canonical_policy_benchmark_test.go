package valuebench

import (
	"math"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

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

func TestFloatCanonicalPolicyFixtures(t *testing.T) {
	t.Parallel()
	for _, fixture := range floatPolicyFixtures(t) {
		t.Run(fixture.name, func(t *testing.T) { t.Parallel(); checkFloatPolicyFixture(t, fixture) })
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
