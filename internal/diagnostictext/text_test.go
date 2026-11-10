package diagnostictext_test

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/diagnostictext"
)

func TestQuote(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"", `""`},
		{"\"\\\n\t\x00", `"\"\\\n\t\x00"`},
		{strings.Repeat("a", 128), `"` + strings.Repeat("a", 128) + `"`},
		{strings.Repeat("a", 127) + "éx", `"` + strings.Repeat("a", 127) + `"...`},
		{"\xff", `"\xff"`},
	} {
		if got := diagnostictext.Quote(test.input); got != test.want {
			t.Errorf("Quote(%q) = %q, want %q", test.input, got, test.want)
		}
		raw := []byte(test.input)
		got := diagnostictext.Quote(raw)
		clear(raw)
		if got != test.want {
			t.Errorf("Quote(bytes of %q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestExpandedName(t *testing.T) {
	for _, test := range []struct{ namespace, local, want string }{
		{"", "child", `"child"`},
		{"urn:one", "child", `"{urn:one}child"`},
		{strings.Repeat("a", 127) + "é", "child", `"{` + strings.Repeat("a", 127) + `"...`},
	} {
		if got := diagnostictext.ExpandedName(test.namespace, test.local); got != test.want {
			t.Errorf("ExpandedName() = %q, want %q", got, test.want)
		}
	}
}

func TestAppendEscaped(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"empty", "", ""},
		{"exactly the bound", strings.Repeat("a", 128), strings.Repeat("a", 128)},
		{"multi-byte rune straddles the cut", strings.Repeat("a", 127) + "éx", strings.Repeat("a", 127) + "..."},
		{"backslash", `a\b`, `a\\b`},
		{"line feed", "a\nb", `a\nb`},
		{"line separator", "a\u2028b", `a\u2028b`},
		{"non-printable above U+FFFF", "a\U000E0001b", `a\U000e0001b`},
		{"invalid byte", "a\xffb", `a\xffb`},
		{"double quote", `a"b`, `a"b`},
		{"maximal expansion", strings.Repeat("\x01", 128), strings.Repeat(`\x01`, 128)},
		{"over the bound", strings.Repeat("a", 129), strings.Repeat("a", 128) + "..."},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := diagnostictext.AppendEscaped([]byte("prefix:"), test.input)
			if string(got) != "prefix:"+test.want {
				t.Fatalf("AppendEscaped() = %q, want %q", got, "prefix:"+test.want)
			}
			if !utf8.Valid(got) {
				t.Fatalf("AppendEscaped() = %q, not valid UTF-8", got)
			}
		})
	}
}

func TestAppendEscapedMatchesQuote(t *testing.T) {
	alphabet := []string{"a", "\\", "\n", "\t", "\x00", "\x01", "\u00e9", "\u2028", "\U000E0001", "\xff", " ", "\x7f"}
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // Deterministic test input, not security-sensitive.
	for range 2000 {
		var input strings.Builder
		for range rng.IntN(200) {
			input.WriteString(alphabet[rng.IntN(len(alphabet))])
		}
		text := input.String()
		got := string(diagnostictext.AppendEscaped(nil, text))
		quoted := diagnostictext.Quote(text)
		want, omitted := quoted, ""
		if i := strings.LastIndexByte(quoted, '"'); i >= 0 {
			want, omitted = quoted[:i+1], quoted[i+1:]
		}
		want = want[1:len(want)-1] + omitted
		if got != want {
			t.Fatalf("AppendEscaped(%q) = %q, want %q", text, got, want)
		}
		if len(got) > 4*diagnostictext.MaxValueBytes+3 || !utf8.ValidString(got) {
			t.Fatalf("AppendEscaped(%q) = %q, over the bound or invalid UTF-8", text, got)
		}
	}
}
