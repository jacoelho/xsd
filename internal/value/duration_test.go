package value

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

func durationForTest(t *testing.T, text string) DurationValue {
	t.Helper()
	d, err := ParseDurationValue(text)
	if err != nil {
		t.Fatalf("ParseDurationValue(%q): %v", text, err)
	}
	return d
}

func TestDurationLexicalSpace(t *testing.T) {
	valid := []string{
		"P1Y2M3DT4H5M6.7S", "-PT0S", "P3D", "PT0.000S",
		"PT9223372036854775808S", "P768614336404564650Y8M",
		"P106751991167300DT15H30M8S", "P9223372036854775808D",
		"P00000000000000000000000000000000000000000000000000001Y",
	}
	invalid := []string{
		"", "P", "PT", "P1DT", "P1YTT1S", "+P1D", "P-1D", "P1.5Y",
		"PT1.0H", "PT1.0M", "P1H", "P1Y2Y", "P1D2M", "PT1.S",
		"PT.5S", "P1S", "PT1Y", "P1Dgarbage", "P١D", " P1D", "P1D ",
	}
	for _, input := range valid {
		if err := ValidateDurationLexical(input); err != nil {
			t.Errorf("valid duration %q: %v", input, err)
		}
		if err := ValidateDurationLexical([]byte(input)); err != nil {
			t.Errorf("valid byte duration %q: %v", input, err)
		}
	}
	for _, input := range invalid {
		if err := ValidateDurationLexical(input); err == nil {
			t.Errorf("accepted invalid duration %q", input)
		}
		if err := ValidateDurationLexical([]byte(input)); err == nil {
			t.Errorf("accepted invalid byte duration %q", input)
		}
	}
}

func TestDurationEqualityAndIdentity(t *testing.T) {
	for _, pair := range [][2]string{
		{"P1Y", "P12M"}, {"PT1S", "PT1.0000S"}, {"-PT0.000S", "P0Y"},
		{"P100000000000000000000Y", "P1200000000000000000000M"},
		{"-P100000000000000000000D", "-PT8640000000000000000000000S"},
		{"PT9223372036854775808S", "PT9223372036854775808.000S"},
	} {
		a, b := durationForTest(t, pair[0]), durationForTest(t, pair[1])
		if !EqualDurationValues(a, b) {
			t.Errorf("durations differ: %s and %s", pair[0], pair[1])
		}
		if durationIdentityCanonical(a) != durationIdentityCanonical(b) {
			t.Errorf("identity keys differ: %s and %s", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{{"P1Y", "P365D"}, {"-PT0.5S", "PT0.5S"}} {
		a, b := durationForTest(t, pair[0]), durationForTest(t, pair[1])
		if EqualDurationValues(a, b) || durationIdentityCanonical(a) == durationIdentityCanonical(b) {
			t.Errorf("distinct durations conflated: %s and %s", pair[0], pair[1])
		}
	}
}

func TestDurationPartialOrder(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want OrderedFacetRelation
	}{
		{"P1Y", "P364D", OrderedFacetGreater},
		{"P1Y", "P365D", OrderedFacetIncomparable},
		{"P1Y", "P366D", OrderedFacetIncomparable},
		{"P1Y", "P367D", OrderedFacetLess},
		{"P1M", "P27D", OrderedFacetGreater},
		{"P1M", "P28D", OrderedFacetIncomparable},
		{"P1M", "P30D", OrderedFacetIncomparable},
		{"P1M", "P32D", OrderedFacetLess},
		{"PT0.10000000000000000001S", "PT0.10000000000000000002S", OrderedFacetLess},
		{"-PT0.4S", "-PT0.5S", OrderedFacetGreater},
		{"P400Y", "P146097D", OrderedFacetEqual},
		{"P400000000000000000000Y", "P146097000000000000000000D", OrderedFacetEqual},
		{"-P400000000000000000000Y", "-P146097000000000000000000D", OrderedFacetEqual},
		{"P400000000000000000000YT0.1S", "P146097000000000000000000D", OrderedFacetGreater},
		{"-P400000000000000000000YT0.1S", "-P146097000000000000000000D", OrderedFacetLess},
	} {
		a, b := durationForTest(t, tc.a), durationForTest(t, tc.b)
		if got := CompareDurationValues(a, b); got != tc.want {
			t.Errorf("CompareDurationValues(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestDurationArbitraryMagnitude(t *testing.T) {
	// Leading zeroes and large components remain linear in input length;
	// Gregorian arithmetic must not collapse an admitted value to incomparable.
	zeroes := strings.Repeat("0", 100_000)
	a := durationForTest(t, "P400"+zeroes+"Y")
	b := durationForTest(t, "P146097"+zeroes+"D")
	// A complete 400-year Gregorian cycle always has exactly 146097 days.
	if got := CompareDurationValues(a, b); got != OrderedFacetEqual {
		t.Fatalf("large Gregorian cycle order = %v, want equal", got)
	}
	zero := durationForTest(t, "PT"+strings.Repeat("0", 100_000)+"S")
	if !EqualDurationValues(zero, DurationValue{}) {
		t.Fatal("large lexical zero differs from zero duration")
	}
}

func TestDurationArithmeticAgainstExactIntegers(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 29)) //nolint:gosec // Reproducible arithmetic test values, not security-sensitive randomness.
	for range 1000 {
		a := new(big.Int).SetUint64(rng.Uint64())
		b := new(big.Int).SetUint64(rng.Uint64())
		a.Lsh(a, uint(rng.IntN(128)))
		b.Lsh(b, uint(rng.IntN(128)))
		x, y := newDurationInteger(a.String()), newDurationInteger(b.String())
		if rng.IntN(2) == 0 {
			a.Neg(a)
			x = x.neg()
		}
		if rng.IntN(2) == 0 {
			b.Neg(b)
			y = y.neg()
		}
		if got, want := x.add(y).text(), new(big.Int).Add(a, b).String(); got != want {
			t.Fatalf("duration addition = %s, want %s", got, want)
		}
		if got, want := x.mul(146097).text(), new(big.Int).Mul(a, big.NewInt(146097)).String(); got != want {
			t.Fatalf("calendar multiplication = %s, want %s", got, want)
		}
		q, r := x.divFloor(4800)
		wantQ, wantR := new(big.Int), new(big.Int)
		wantQ.DivMod(a, big.NewInt(4800), wantR)
		if q.text() != wantQ.String() || r != wantR.Int64() {
			t.Fatalf("calendar division = %s remainder %d, want %s remainder %s", q.text(), r, wantQ, wantR)
		}
	}
}

func TestDurationReferenceOrderingAgainstCalendar(t *testing.T) {
	rng := rand.New(rand.NewPCG(37, 41)) //nolint:gosec // Reproducible calendar test values, not security-sensitive randomness.
	refs := []time.Time{
		time.Date(1696, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(1697, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1903, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(1903, 7, 1, 0, 0, 0, 0, time.UTC),
	}
	for range 1000 {
		am, bm := rng.IntN(100_000)-50_000, rng.IntN(100_000)-50_000
		ad, bd := rng.IntN(1000), rng.IntN(1000)
		a, b := calendarDuration(t, am, ad), calendarDuration(t, bm, bd)
		var expected int
		want := OrderedFacetEqual
		for i, ref := range refs {
			x := ref.AddDate(0, am, signedDurationDays(am, ad))
			y := ref.AddDate(0, bm, signedDurationDays(bm, bd))
			n := x.Compare(y)
			if i > 0 && n != expected {
				want = OrderedFacetIncomparable
				break
			}
			expected, want = n, orderedFacetRelationFromInt(n)
		}
		if got := CompareDurationValues(a, b); got != want {
			t.Fatalf("calendar order for months %d/%d days %d/%d = %v, want %v", am, bm, ad, bd, got, want)
		}
	}
}

func signedDurationDays(months, days int) int {
	if months < 0 {
		return -days
	}
	return days
}

func calendarDuration(t *testing.T, months, days int) DurationValue {
	t.Helper()
	sign := ""
	if months < 0 {
		sign, months = "-", -months
	}
	return durationForTest(t, fmt.Sprintf("%sP%dM%dD", sign, months, days))
}

func BenchmarkDurationParse(b *testing.B) {
	for _, input := range []string{"P1Y2M3DT4H5M6.7S", "P400000000000000000000Y", "PT0.5S"} {
		b.Run(input, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := ParseDurationValue(input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
