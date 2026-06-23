package runtime

import "testing"

func TestValidateTemporalLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
		kind    PrimitiveKind
	}{
		{name: "dateTime leap second", kind: PrimitiveDateTime, input: "2026-05-18T23:59:60Z"},
		{name: "dateTime hour twenty four", kind: PrimitiveDateTime, input: "2026-05-18T24:00:00Z"},
		{name: "dateTime zero fraction at hour twenty four", kind: PrimitiveDateTime, input: "-0001-12-31T24:00:00.0"},
		{name: "dateTime expanded year timezone", kind: PrimitiveDateTime, input: "10000-01-01T00:00:00+14:00"},
		{name: "dateTime rejects missing separator", kind: PrimitiveDateTime, input: "2026-05-18 00:00:00", wantErr: "invalid dateTime"},
		{name: "dateTime rejects nonzero fraction at hour twenty four", kind: PrimitiveDateTime, input: "2026-05-18T24:00:00.1", wantErr: "invalid dateTime"},
		{name: "dateTime rejects zero year", kind: PrimitiveDateTime, input: "0000-01-01T00:00:00", wantErr: dateErrInvalidDateTime},
		{name: "dateTime rejects non leap day", kind: PrimitiveDateTime, input: "1900-02-29T00:00:00", wantErr: dateErrInvalidDateTime},
		{name: "dateTime rejects timezone above max", kind: PrimitiveDateTime, input: "2026-05-18T00:00:00+14:01", wantErr: "invalid dateTime"},
		{name: "time plain", kind: PrimitiveTime, input: "00:00:00"},
		{name: "time leap second", kind: PrimitiveTime, input: "23:59:60Z"},
		{name: "time hour twenty four", kind: PrimitiveTime, input: "24:00:00"},
		{name: "time zero fraction at hour twenty four", kind: PrimitiveTime, input: "24:00:00.0"},
		{name: "time negative max timezone", kind: PrimitiveTime, input: "12:34:56.789-14:00"},
		{name: "time rejects nonzero fraction at hour twenty four", kind: PrimitiveTime, input: "24:00:00.1", wantErr: "invalid time"},
		{name: "time rejects non terminal leap second", kind: PrimitiveTime, input: "23:58:60", wantErr: "invalid time"},
		{name: "time rejects timezone above max", kind: PrimitiveTime, input: "12:00:00+14:01", wantErr: dateErrInvalidTimezone},
		{name: "gYearMonth timezone", kind: PrimitiveGYearMonth, input: "2000-01Z"},
		{name: "gYearMonth negative year", kind: PrimitiveGYearMonth, input: "-0001-12+14:00"},
		{name: "gYearMonth rejects zero year", kind: PrimitiveGYearMonth, input: "0000-01", wantErr: dateErrInvalidDateTime},
		{name: "gYearMonth rejects month", kind: PrimitiveGYearMonth, input: "2000-13", wantErr: "invalid gYearMonth"},
		{name: "gYear expanded", kind: PrimitiveGYear, input: "10000Z"},
		{name: "gYear rejects leading plus", kind: PrimitiveGYear, input: "+2000", wantErr: dateErrInvalidDateTime},
		{name: "gYear rejects expanded leading zero", kind: PrimitiveGYear, input: "02000", wantErr: dateErrInvalidDateTime},
		{name: "gMonthDay leap day", kind: PrimitiveGMonthDay, input: "--02-29"},
		{name: "gMonthDay rejects impossible day", kind: PrimitiveGMonthDay, input: "--02-30", wantErr: "invalid gMonthDay"},
		{name: "gDay max", kind: PrimitiveGDay, input: "---31-14:00"},
		{name: "gDay rejects day", kind: PrimitiveGDay, input: "---32", wantErr: "invalid gDay"},
		{name: "gMonth max", kind: PrimitiveGMonth, input: "--12Z"},
		{name: "gMonth rejects month", kind: PrimitiveGMonth, input: "--13", wantErr: "invalid gMonth"},
		{name: "unsupported primitive", kind: PrimitiveString, input: "raw", wantErr: "invalid temporal primitive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bytesErr := ValidateTemporalLexical(tt.kind, []byte(tt.input))
			if got := errorMessage(bytesErr); got != tt.wantErr {
				t.Fatalf("ValidateTemporalLexical() error = %q, want %q", got, tt.wantErr)
			}
			stringErr := ValidateTemporalLexical(tt.kind, tt.input)
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateTemporalLexical string error for %q = %v, bytes error = %v", tt.input, stringErr, bytesErr)
			}
		})
	}
}

func TestTemporalValueCanonicalText(t *testing.T) {
	t.Parallel()

	date, err := ParseDateValue("2020-01-01+14:00")
	if err != nil {
		t.Fatalf("ParseDateValue() error = %v", err)
	}
	if got, want := date.CanonicalText(), "2019-12-31-10:00"; got != want {
		t.Fatalf("DateValue.CanonicalText() = %q, want %q", got, want)
	}

	dateTime, err := ParseDateTimeValue("2020-01-01T24:00:00Z")
	if err != nil {
		t.Fatalf("ParseDateTimeValue() error = %v", err)
	}
	if got, want := dateTime.CanonicalText(), "2020-01-02T00:00:00Z"; got != want {
		t.Fatalf("DateTimeValue.CanonicalText() = %q, want %q", got, want)
	}

	tm, err := ParseTimeValue("24:00:00")
	if err != nil {
		t.Fatalf("ParseTimeValue() error = %v", err)
	}
	if got, want := tm.CanonicalText(), "00:00:00"; got != want {
		t.Fatalf("TimeValue.CanonicalText() = %q, want %q", got, want)
	}
}

func TestTemporalValueEquality(t *testing.T) {
	t.Parallel()

	a, err := ParseDateTimeValue("1998-12-31T23:59:60Z")
	if err != nil {
		t.Fatalf("ParseDateTimeValue(a) error = %v", err)
	}
	b, err := ParseDateTimeValue("1999-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("ParseDateTimeValue(b) error = %v", err)
	}
	if !EqualTemporalValues(a.Temporal(), b.Temporal()) {
		t.Fatal("EqualTemporalValues() = false, want true")
	}

	ta, err := ParseTimeValue("10:30:00Z")
	if err != nil {
		t.Fatalf("ParseTimeValue(a) error = %v", err)
	}
	tb, err := ParseTimeValue("00:30:00-10:00")
	if err != nil {
		t.Fatalf("ParseTimeValue(b) error = %v", err)
	}
	if !EqualTimeValues(ta, tb) {
		t.Fatal("EqualTimeValues() = false, want true")
	}
}

func TestTemporalValuePartialOrderTimezoneAbsence(t *testing.T) {
	t.Parallel()

	dateAbsent, err := ParseDateValue("2020-01-01")
	if err != nil {
		t.Fatalf("ParseDateValue(absent) error = %v", err)
	}
	dateZ, err := ParseDateValue("2020-01-01Z")
	if err != nil {
		t.Fatalf("ParseDateValue(z) error = %v", err)
	}
	if got := CompareTemporalValues(dateAbsent.Temporal(), dateZ.Temporal()); got != OrderedFacetIncomparable {
		t.Fatalf("CompareTemporalValues(date absent, z) = %v, want incomparable", got)
	}

	dateTimeAbsent, err := ParseDateTimeValue("2020-01-01T12:00:00")
	if err != nil {
		t.Fatalf("ParseDateTimeValue(absent) error = %v", err)
	}
	dateTimeZ, err := ParseDateTimeValue("2020-01-01T12:00:00Z")
	if err != nil {
		t.Fatalf("ParseDateTimeValue(z) error = %v", err)
	}
	if got := CompareTemporalValues(dateTimeAbsent.Temporal(), dateTimeZ.Temporal()); got != OrderedFacetIncomparable {
		t.Fatalf("CompareTemporalValues(dateTime absent, z) = %v, want incomparable", got)
	}

	timeAbsent, err := ParseTimeRawValue("12:00:00")
	if err != nil {
		t.Fatalf("ParseTimeRawValue(absent) error = %v", err)
	}
	timeZ, err := ParseTimeRawValue("12:00:00Z")
	if err != nil {
		t.Fatalf("ParseTimeRawValue(z) error = %v", err)
	}
	if got := CompareTimePartial(timeAbsent, timeZ); got != OrderedFacetIncomparable {
		t.Fatalf("CompareTimePartial(absent, z) = %v, want incomparable", got)
	}
}
