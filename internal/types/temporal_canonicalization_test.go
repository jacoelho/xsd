package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/value"
)

func TestTemporalCanonicalizationMatchesValue(t *testing.T) {
	cases := []struct {
		kind    TypeName
		lexical string
	}{
		{kind: TypeNameDateTime, lexical: "2001-10-26T21:32:52+05:30"},
		{kind: TypeNameTime, lexical: "21:32:52+05:30"},
		{kind: TypeNameGYearMonth, lexical: "2001-10+05:30"},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			bt := GetBuiltin(tc.kind)
			if bt == nil {
				t.Fatalf("builtin %s missing", tc.kind)
			}
			tv, err := ParseValueForType(tc.lexical, tc.kind, bt)
			if err != nil {
				t.Fatalf("ParseValueForType(%s) error = %v", tc.kind, err)
			}
			got := tv.String()

			parsed, err := parseTemporalForKind(tc.kind, tc.lexical)
			if err != nil {
				t.Fatalf("parse temporal %s error = %v", tc.kind, err)
			}
			tzKind := value.TimezoneKindFromLexical([]byte(tc.lexical))
			want := value.CanonicalDateTimeString(parsed, string(tc.kind), tzKind)
			if got != want {
				t.Fatalf("canonical = %q, want %q", got, want)
			}
		})
	}
}

func TestTemporalCanonicalizationRoundTripParseable(t *testing.T) {
	cases := []struct {
		kind    TypeName
		lexical string
	}{
		{kind: TypeNameDateTime, lexical: "1999-12-31T23:59:60+02:00"},
		{kind: TypeNameTime, lexical: "23:59:60+02:00"},
		{kind: TypeNameTime, lexical: "23:59:60Z"},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind)+"_"+tc.lexical, func(t *testing.T) {
			bt := GetBuiltin(tc.kind)
			if bt == nil {
				t.Fatalf("builtin %s missing", tc.kind)
			}
			tv, err := ParseValueForType(tc.lexical, tc.kind, bt)
			if err != nil {
				t.Fatalf("ParseValueForType(%s) error = %v", tc.kind, err)
			}
			canonical := tv.String()
			if _, err := parseTemporalForKind(tc.kind, canonical); err != nil {
				t.Fatalf("parse canonical %q for %s error = %v", canonical, tc.kind, err)
			}
		})
	}
}

func parseTemporalForKind(kind TypeName, lexical string) (time.Time, error) {
	switch kind {
	case TypeNameDateTime:
		return value.ParseDateTime([]byte(lexical))
	case TypeNameTime:
		return value.ParseTime([]byte(lexical))
	case TypeNameGYearMonth:
		return value.ParseGYearMonth([]byte(lexical))
	default:
		return time.Time{}, fmt.Errorf("unsupported temporal kind %s", kind)
	}
}
