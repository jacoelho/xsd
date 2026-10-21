package diagnostictext_test

import (
	"strings"
	"testing"

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
