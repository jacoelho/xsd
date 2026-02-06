package value

import "testing"

func TestTimezoneKindFromLexical(t *testing.T) {
	tests := []struct {
		name    string
		lexical string
		want    TimezoneKind
	}{
		{name: "empty", lexical: "", want: TZNone},
		{name: "no timezone", lexical: "2001-10-26", want: TZNone},
		{name: "z suffix", lexical: "2001-10-26Z", want: TZKnown},
		{name: "plus zero", lexical: "2001-10-26+00:00", want: TZKnown},
		{name: "minus zero", lexical: "2001-10-26-00:00", want: TZKnown},
		{name: "plus max", lexical: "2001-10-26+14:00", want: TZKnown},
		{name: "minus max", lexical: "2001-10-26-14:00", want: TZKnown},
		{name: "trimmed surrounding whitespace", lexical: "\n\t2001-10-26+02:30 \r", want: TZKnown},
		{name: "timezone only lexical", lexical: "+01:00", want: TZKnown},
		{name: "short plus one hour", lexical: "2001-10-26+1:00", want: TZNone},
		{name: "short minutes", lexical: "2001-10-26+01:0", want: TZNone},
		{name: "missing colon", lexical: "2001-10-26+0100", want: TZNone},
		{name: "bad separator", lexical: "2001-10-26+00;00", want: TZNone},
		{name: "trailing plus", lexical: "2001-10-26+", want: TZNone},
		{name: "non terminal z", lexical: "2001-10-26Za", want: TZNone},
	}

	for _, tc := range tests {
		got := TimezoneKindFromLexical([]byte(tc.lexical))
		if got != tc.want {
			t.Fatalf("%s: TimezoneKindFromLexical(%q) = %v, want %v", tc.name, tc.lexical, got, tc.want)
		}
	}
}
