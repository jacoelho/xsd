package compile

import (
	"regexp"
	"testing"
	"unicode/utf8"
)

func FuzzXSDRegexSyntax(f *testing.F) {
	for _, seed := range []string{
		`[A-Z]{2}\d{4}`,
		`\p{Lu}+`,
		`a|b`,
		`[abc-]`,
		`([a-z]+)?`,
		`\p{}0`,
		`\p{Is}`,
		`\C0`,
		`0{0002}`,
		`0{1001}`,
		`0{1001,}`,
		`0{0,1001}`,
		`0{1001,1000}`,
		`0{0001001}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 256 {
			t.Skip()
		}
		if !utf8.ValidString(source) {
			return
		}
		if err := ValidateXSDRegexSyntax(source, nil); err != nil {
			return
		}
		goSource := "^(?:" + TranslateXSDRegexToGo(source) + ")$"
		if _, err := regexp.Compile(goSource); err != nil {
			t.Fatalf("validated regex does not compile: %q -> %q: %v", source, goSource, err)
		}
	})
}
