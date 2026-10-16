package value

import "testing"

func TestTemporalLexicalAdmission(t *testing.T) {
	tests := []struct {
		kind          PrimitiveKind
		lexical, want string
	}{
		{PrimitiveDate, "2000-02-29", ""},
		{PrimitiveDate, "1900-02-29", "invalid date/time"},
		{PrimitiveDate, "2024-00-01", "invalid date/time"},
		{PrimitiveDate, "2024-13-01", "invalid date/time"},
		{PrimitiveDate, "2024-04-31", "invalid date/time"},
		{PrimitiveDate, "2024-01-00", "invalid date/time"},
		{PrimitiveDate, "-0001-02-29", ""},
		{PrimitiveDate, "-0401-02-29", ""},
		{PrimitiveDate, "-0101-02-29", "invalid date/time"},
		{PrimitiveDate, "-0400-02-29", "invalid date/time"},
		{PrimitiveGYearMonth, "2024-01", ""},
		{PrimitiveGYearMonth, "2024-12", ""},
		{PrimitiveGYearMonth, "2024-00", "invalid gYearMonth"},
		{PrimitiveGYearMonth, "2024-13", "invalid gYearMonth"},
		{PrimitiveGYearMonth, "2024/02", "invalid gYearMonth"},
		{PrimitiveGMonthDay, "--02-29", ""},
		{PrimitiveGMonthDay, "--02-30", "invalid gMonthDay"},
		{PrimitiveGMonthDay, "--04-31", "invalid gMonthDay"},
		{PrimitiveGMonthDay, "--01-00", "invalid gMonthDay"},
		{PrimitiveGMonthDay, "--00-01", "invalid gMonthDay"},
		{PrimitiveGMonthDay, "--13-01", "invalid gMonthDay"},
		{PrimitiveGMonthDay, "-02-29", "invalid gMonthDay"},
		{PrimitiveGMonthDay, "--02/29", "invalid gMonthDay"},
		{PrimitiveGDay, "---01", ""},
		{PrimitiveGDay, "---31", ""},
		{PrimitiveGDay, "---00", "invalid gDay"},
		{PrimitiveGDay, "---32", "invalid gDay"},
		{PrimitiveGDay, "--01", "invalid gDay"},
		{PrimitiveGMonth, "--01", ""},
		{PrimitiveGMonth, "--12", ""},
		{PrimitiveGMonth, "--00", "invalid gMonth"},
		{PrimitiveGMonth, "--13", "invalid gMonth"},
		{PrimitiveGMonth, "-01", "invalid gMonth"},
		{PrimitiveGMonth, "--02--", "invalid timezone"},
		{PrimitiveGYearMonth, "0000-13+15:00", "invalid date/time"},
		{PrimitiveGYearMonth, "2024-13+15:00", "invalid gYearMonth"},
		{PrimitiveGMonthDay, "--02-30+15:00", "invalid gMonthDay"},
		{PrimitiveGDay, "---32+15:00", "invalid gDay"},
		{PrimitiveGMonth, "--13+15:00", "invalid gMonth"},
		{PrimitiveTime, "24:00:00", ""},
		{PrimitiveTime, "24:00:00.000", ""},
		{PrimitiveTime, "24:00:00.001", "invalid time"},
		{PrimitiveTime, "24:00:01", "invalid time"},
		{PrimitiveTime, "24:01:00", "invalid time"},
		{PrimitiveTime, "23:59:60", ""},
		{PrimitiveTime, "23:59:60.125", ""},
		{PrimitiveTime, "12:00:60", "invalid time"},
		{PrimitiveTime, "12:00:00.", "invalid time"},
		{PrimitiveTime, "25:00:00+15:00", "invalid timezone"},
		{PrimitiveTime, "25:00:00.", "invalid time"},
		{PrimitiveDateTime, "2024-02-29T25:00:00+15:00", "invalid dateTime"},
		{PrimitiveDateTime, "2024-02-29T25:00:00.", "invalid dateTime"},
		{PrimitiveDateTime, "1900-02-29T25:00:00+15:00", "invalid date/time"},
		{PrimitiveDateTime, "2024-02-29 12:00:00", "invalid dateTime"},
	}
	seeds := []struct {
		kind                         PrimitiveKind
		lexical, invalidTZ, trailing string
	}{
		{PrimitiveDate, "2024-02-29", "invalid timezone", "invalid date"},
		{PrimitiveDateTime, "2024-02-29T12:34:56", "invalid dateTime", "invalid dateTime"},
		{PrimitiveTime, "12:34:56", "invalid timezone", "invalid time"},
		{PrimitiveGYearMonth, "2024-02", "invalid timezone", "invalid gYearMonth"},
		{PrimitiveGYear, "2024", "invalid timezone", "invalid gYear"},
		{PrimitiveGMonthDay, "--02-29", "invalid timezone", "invalid gMonthDay"},
		{PrimitiveGDay, "---31", "invalid timezone", "invalid gDay"},
		{PrimitiveGMonth, "--12", "invalid timezone", "invalid gMonth"},
	}
	for _, seed := range seeds {
		for _, suffix := range []string{"", "Z", "+00:00", "-00:00", "+14:00", "-14:00"} {
			tests = append(tests, struct {
				kind          PrimitiveKind
				lexical, want string
			}{seed.kind, seed.lexical + suffix, ""})
		}
		for _, suffix := range []string{"+14:01", "-14:01", "+15:00", "+00:60", "+00", "+00:"} {
			tests = append(tests, struct {
				kind          PrimitiveKind
				lexical, want string
			}{seed.kind, seed.lexical + suffix, seed.invalidTZ})
		}
		tests = append(tests, struct {
			kind          PrimitiveKind
			lexical, want string
		}{seed.kind, seed.lexical + "Zx", seed.trailing})
	}
	for _, suffix := range []struct {
		kind PrimitiveKind
		tail string
	}{
		{PrimitiveDate, "-02-28"}, {PrimitiveDateTime, "-02-28T12:00:00"}, {PrimitiveGYearMonth, "-02"}, {PrimitiveGYear, ""},
	} {
		for _, year := range []string{"0000", "-0000", "+2024", "202", "02024"} {
			tests = append(tests, struct {
				kind          PrimitiveKind
				lexical, want string
			}{suffix.kind, year + suffix.tail, "invalid date/time"})
		}
		tests = append(tests, struct {
			kind          PrimitiveKind
			lexical, want string
		}{suffix.kind, "123456789012345678901234567890" + suffix.tail, ""})
	}
	for _, tc := range tests {
		t.Run(tc.lexical, func(t *testing.T) {
			raw := []byte(tc.lexical)
			assertTemporalError(t, ValidateTemporalLexical(tc.kind, tc.lexical), tc.want)
			assertTemporalError(t, ValidateTemporalLexical(tc.kind, raw), tc.want)
			assertTemporalError(t, parseTemporalForTest(t, tc.kind, tc.lexical), tc.want)
		})
	}
}

func assertTemporalError(t *testing.T, err error, want string) {
	t.Helper()
	got := ""
	if err != nil {
		got = err.Error()
	}
	if got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func parseTemporalForTest(t *testing.T, kind PrimitiveKind, lexical string) error {
	t.Helper()
	//nolint:exhaustive // This fixture routes only the eight temporal primitives.
	switch kind {
	case PrimitiveDate:
		_, err := ParseDateValue(lexical)
		return err
	case PrimitiveDateTime:
		_, err := ParseDateTimeValue(lexical)
		return err
	case PrimitiveTime:
		_, err := ParseTimeValue(lexical)
		return err
	case PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth:
		_, err := ParseGValue(kind, lexical)
		return err
	default:
		t.Fatalf("unsupported temporal fixture kind %d", kind)
	}
	return nil
}

func TestTemporalCanonicalProjectionOwnsBytes(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id            TypeID
		lexical, want string
	}{
		{builtinDate, "2000-02-29+00:00", "2000-02-29Z"},
		{builtinDateTime, "2000-02-29T24:00:00.000Z", "2000-03-01T00:00:00Z"},
		{builtinDateTime, "2000-02-29T00:30:00.1200+01:00", "2000-02-28T23:30:00.12Z"},
		{builtinDateTime, "2000-02-29T00:30:00.1200", "2000-02-29T00:30:00.12"},
		{builtinTime, "00:30:00+01:00", "23:30:00Z"},
		{builtinTime, "12:34:56.1200Z", "12:34:56.12Z"},
		{builtinTime, "12:34:56.000Z", "12:34:56Z"},
		{builtinGMonth, "--02+00:00", "--02Z"},
		{builtinGYearMonth, "-0001-02+14:00", "-0001-02+14:00"},
		{builtinGYear, "0001", "0001"},
		{builtinGMonthDay, "--01-01+14:00", "--01-01+14:00"},
		{builtinGDay, "---01-14:00", "---01-14:00"},
		{builtinGMonth, "--12", "--12"},
	} {
		t.Run(tc.lexical, func(t *testing.T) {
			got, err := p.Validate(tc.id, tc.lexical, Resolver{}, NeedCanonical, 16<<20, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got.CanonicalText() != tc.want {
				t.Fatalf("canonical = %q, want %q", got.CanonicalText(), tc.want)
			}
			raw := []byte(tc.lexical)
			got, err = p.ValidateBytes(tc.id, raw, Resolver{}, NeedCanonical, 16<<20, nil)
			if err != nil {
				t.Fatal(err)
			}
			clear(raw)
			if got.CanonicalText() != tc.want {
				t.Fatalf("canonical after input mutation = %q, want %q", got.CanonicalText(), tc.want)
			}
		})
	}
}
