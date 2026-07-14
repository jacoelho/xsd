package runtime

import "testing"

func TestValidateDurationLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "full", input: "P1Y2M3DT4H5M6.7S"},
		{name: "negative zero seconds", input: "-PT0S"},
		{name: "days only", input: "P3D"},
		{name: "max seconds", input: "PT9223372036854775807S"},
		{name: "max months", input: "P768614336404564650Y7M"},
		{name: "max day time", input: "P106751991167300DT15H30M7S"},
		{name: "rejects no parts", input: "P", wantErr: "invalid duration"},
		{name: "rejects empty time", input: "PT", wantErr: "invalid duration"},
		{name: "rejects fractional years", input: "P1.5Y", wantErr: "invalid duration"},
		{name: "rejects fractional hours", input: "PT1.0H", wantErr: "invalid duration"},
		{name: "rejects fractional minutes", input: "PT1.0M", wantErr: "invalid duration"},
		{name: "rejects time unit before T", input: "P1H", wantErr: "invalid duration"},
		{name: "rejects repeated field", input: "P1Y2Y", wantErr: "invalid duration"},
		{name: "rejects out of order date fields", input: "P1D2M", wantErr: "invalid duration"},
		{name: "rejects missing fraction digits", input: "PT1.S", wantErr: "invalid duration"},
		{name: "rejects seconds overflow", input: "PT9223372036854775808S", wantErr: "invalid duration"},
		{name: "rejects month overflow", input: "P768614336404564650Y8M", wantErr: "invalid duration"},
		{name: "rejects day time overflow", input: "P106751991167300DT15H30M8S", wantErr: "invalid duration"},
		{name: "rejects unsigned field overflow", input: "P9223372036854775808D", wantErr: "invalid duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bytesErr := ValidateDurationLexical([]byte(tt.input))
			if got := errorMessage(bytesErr); got != tt.wantErr {
				t.Fatalf("ValidateDurationLexical() error = %q, want %q", got, tt.wantErr)
			}
			stringErr := ValidateDurationLexical(tt.input)
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateDurationLexical string error for %q = %v, bytes error = %v", tt.input, stringErr, bytesErr)
			}
		})
	}
}

func TestDurationValueEquality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a    string
		b    string
		want bool
	}{
		{a: "P1Y", b: "P12M", want: true},
		{a: "PT1.0S", b: "PT1S", want: true},
		{a: "-PT0.0S", b: "PT0S", want: true},
		{a: "-PT0.5S", b: "PT0.5S", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()

			a := mustParseDurationValue(t, tt.a)
			b := mustParseDurationValue(t, tt.b)
			if got := EqualDurationValues(a, b); got != tt.want {
				t.Fatalf("EqualDurationValues(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestDurationIdentityCanonicalMatchesValueEquality(t *testing.T) {
	t.Parallel()

	equal := [][2]string{
		{"P1Y", "P12M"},
		{"PT1S", "PT1.0S"},
		{"-PT0.0S", "PT0S"},
	}
	for _, pair := range equal {
		a := mustParseDurationValue(t, pair[0])
		b := mustParseDurationValue(t, pair[1])
		if got, want := durationIdentityCanonical(a), durationIdentityCanonical(b); got != want {
			t.Fatalf("durationIdentityCanonical(%q) = %q, want %q for equal %q", pair[0], got, want, pair[1])
		}
	}
	a := durationIdentityCanonical(mustParseDurationValue(t, "P1Y"))
	b := durationIdentityCanonical(mustParseDurationValue(t, "P365D"))
	if a == b {
		t.Fatalf("durationIdentityCanonical(P1Y) = durationIdentityCanonical(P365D) = %q", a)
	}
}

func TestCompareDurationValuesMatchesXSDExamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a    string
		b    string
		want OrderedFacetRelation
	}{
		{a: "P1Y", b: "P364D", want: OrderedFacetGreater},
		{a: "P1Y", b: "P365D", want: OrderedFacetIncomparable},
		{a: "P1Y", b: "P366D", want: OrderedFacetIncomparable},
		{a: "P1Y", b: "P367D", want: OrderedFacetLess},
		{a: "P1M", b: "P27D", want: OrderedFacetGreater},
		{a: "P1M", b: "P28D", want: OrderedFacetIncomparable},
		{a: "P1M", b: "P30D", want: OrderedFacetIncomparable},
		{a: "P1M", b: "P32D", want: OrderedFacetLess},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()

			a := mustParseDurationValue(t, tt.a)
			b := mustParseDurationValue(t, tt.b)
			if got := CompareDurationValues(a, b); got != tt.want {
				t.Fatalf("CompareDurationValues(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			if got := CompareDurationValues(b, a); got != reverseOrderedFacetRelation(tt.want) {
				t.Fatalf("CompareDurationValues(%q, %q) = %v, want %v", tt.b, tt.a, got, reverseOrderedFacetRelation(tt.want))
			}
		})
	}
}

func TestCompareDurationValuesHandlesFractionalSecondsExactly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a    string
		b    string
		want OrderedFacetRelation
	}{
		{a: "PT0.10000000000000000001S", b: "PT0.10000000000000000002S", want: OrderedFacetLess},
		{a: "-PT0.4S", b: "-PT0.5S", want: OrderedFacetGreater},
		{a: "-PT0.6S", b: "-PT0.5S", want: OrderedFacetLess},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()

			if got := CompareDurationValues(mustParseDurationValue(t, tt.a), mustParseDurationValue(t, tt.b)); got != tt.want {
				t.Fatalf("CompareDurationValues(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareDurationValuesUsesBoundedDateArithmetic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a    string
		b    string
		want OrderedFacetRelation
	}{
		{a: "P1000000000D", b: "P1M", want: OrderedFacetGreater},
		{a: "PT9223372036854775807S", b: "P1M", want: OrderedFacetGreater},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()

			if got := CompareDurationValues(mustParseDurationValue(t, tt.a), mustParseDurationValue(t, tt.b)); got != tt.want {
				t.Fatalf("CompareDurationValues(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestAddDurationToPointHandlesNegativeFractionalSecond(t *testing.T) {
	t.Parallel()

	d := mustParseDurationValue(t, "-PT0.5S")
	got, ok := addDurationToPoint(xsdDateTimePoint{year: xsdYear{digits: "1903"}, month: 3, day: 1}, d)
	if !ok {
		t.Fatal("addDurationToPoint() failed")
	}
	want := xsdDateTimePoint{year: xsdYear{digits: "1903"}, month: 2, day: 28, second: 86399, frac: "5"}
	if got != want {
		t.Fatalf("addDurationToPoint() = %+v, want %+v", got, want)
	}
}

func mustParseDurationValue(t *testing.T, s string) DurationValue {
	t.Helper()

	value, err := ParseDurationValue(s)
	if err != nil {
		t.Fatalf("ParseDurationValue(%q) error = %v", s, err)
	}
	return value
}

func reverseOrderedFacetRelation(r OrderedFacetRelation) OrderedFacetRelation {
	switch r {
	case OrderedFacetLess:
		return OrderedFacetGreater
	case OrderedFacetGreater:
		return OrderedFacetLess
	default:
		return r
	}
}
