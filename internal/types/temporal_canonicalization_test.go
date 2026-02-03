package types

import (
	"fmt"
	"testing"
	"time"

	valuepkg "github.com/jacoelho/xsd/internal/value"
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
			hasTZ := valuepkg.HasTimezone([]byte(tc.lexical))
			want := valuepkg.CanonicalDateTimeString(parsed, string(tc.kind), hasTZ)
			if got != want {
				t.Fatalf("canonical = %q, want %q", got, want)
			}
		})
	}
}

func parseTemporalForKind(kind TypeName, lexical string) (time.Time, error) {
	switch kind {
	case TypeNameDateTime:
		return valuepkg.ParseDateTime([]byte(lexical))
	case TypeNameTime:
		return valuepkg.ParseTime([]byte(lexical))
	case TypeNameGYearMonth:
		return valuepkg.ParseGYearMonth([]byte(lexical))
	default:
		return time.Time{}, fmt.Errorf("unsupported temporal kind %s", kind)
	}
}
