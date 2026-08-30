package lex

import (
	"slices"
	"testing"
)

func TestStringNameValidatorsRejectMalformedUTF8(t *testing.T) {
	t.Parallel()
	malformed := string([]byte{'n', 0xff})
	if IsXMLName(malformed) {
		t.Fatal("IsXMLName accepted malformed UTF-8")
	}
	if IsNCName(malformed) {
		t.Fatal("IsNCName accepted malformed UTF-8")
	}
	if IsNMTOKEN(malformed) {
		t.Fatal("IsNMTOKEN accepted malformed UTF-8")
	}
	if parts := SplitQName(malformed); parts.Valid {
		t.Fatal("SplitQName accepted malformed UTF-8")
	}

	validReplacement := "n\uFFFD"
	if !IsXMLName(validReplacement) || !IsNCName(validReplacement) || !IsNMTOKEN(validReplacement) {
		t.Fatal("name validators rejected an encoded U+FFFD")
	}
}

func TestXMLWhitespaceHelpers(t *testing.T) {
	t.Parallel()

	if got := TrimXMLWhitespaceString("\t\r a \n"); got != "a" {
		t.Fatalf("TrimXMLWhitespaceString() = %q, want %q", got, "a")
	}
	if got := TrimXMLWhitespaceString("\u00a0a\u00a0"); got != "\u00a0a\u00a0" {
		t.Fatalf("TrimXMLWhitespaceString() = %q, want NBSPs preserved", got)
	}
	if got := string(TrimXMLWhitespaceBytes([]byte("\t\r a \n"))); got != "a" {
		t.Fatalf("TrimXMLWhitespaceBytes() = %q, want %q", got, "a")
	}
	byteTests := []struct {
		name string
		in   []byte
		want bool
	}{
		{name: "empty", in: nil, want: true},
		{name: "xml whitespace", in: []byte(" \t\r\n"), want: true},
		{name: "text", in: []byte(" a "), want: false},
		{name: "unicode space", in: []byte("\u00a0"), want: false},
	}
	for _, tt := range byteTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsXMLWhitespaceBytes(tt.in); got != tt.want {
				t.Fatalf("IsXMLWhitespaceBytes() = %v, want %v", got, tt.want)
			}
		})
	}
	fields := slices.Collect(XMLFieldsSeq(" \ta\nb\rc "))
	if !slices.Equal(fields, []string{"a", "b", "c"}) {
		t.Fatalf("XMLFieldsSeq() = %#v, want %#v", fields, []string{"a", "b", "c"})
	}
	fields = slices.Collect(XMLFieldsSeq("a\u00a0b c"))
	if !slices.Equal(fields, []string{"a\u00a0b", "c"}) {
		t.Fatalf("XMLFieldsSeq() = %#v, want NBSP inside first field", fields)
	}
}

func TestXMLWhitespaceNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       string
		replace  string
		collapse string
	}{
		{name: "plain", in: "abc", replace: "abc", collapse: "abc"},
		{name: "replace", in: "a\tb\nc\rd", replace: "a b c d", collapse: "a b c d"},
		{name: "collapse", in: " \ta  b\n ", replace: "  a  b  ", collapse: "a b"},
		{name: "unicode space", in: "a\u00a0 b", replace: "a\u00a0 b", collapse: "a\u00a0 b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ReplaceXMLWhitespace(tt.in); got != tt.replace {
				t.Fatalf("ReplaceXMLWhitespace() = %q, want %q", got, tt.replace)
			}
			if got := CollapseXMLWhitespace(tt.in); got != tt.collapse {
				t.Fatalf("CollapseXMLWhitespace() = %q, want %q", got, tt.collapse)
			}
		})
	}
}

func TestSplitASCIIQNameBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		prefix    string
		local     string
		ascii     bool
		ok        bool
		hasPrefix bool
	}{
		{name: "local", in: "row", local: "row", ascii: true, ok: true},
		{name: "prefixed", in: "xs:int", prefix: "xs", local: "int", ascii: true, ok: true, hasPrefix: true},
		{name: "leading_colon", in: ":bad", ascii: true},
		{name: "trailing_colon", in: "bad:", ascii: true},
		{name: "duplicate_colon", in: "a:b:c", ascii: true},
		{name: "invalid_char", in: "bad@name", ascii: true},
		{name: "unicode", in: "\u00e9", ascii: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := []byte(tt.in)
			parts := SplitASCIIQNameBytes(input)
			if parts.ASCII() != tt.ascii || parts.Valid() != tt.ok {
				t.Fatalf("SplitASCIIQNameBytes(%q) ascii=%v ok=%v, want ascii=%v ok=%v", tt.in, parts.ASCII(), parts.Valid(), tt.ascii, tt.ok)
			}
			if !parts.Valid() {
				return
			}
			prefix, local := parts.Bytes(input)
			if got := prefix != nil; got != tt.hasPrefix {
				t.Fatalf("prefix presence = %v, want %v", got, tt.hasPrefix)
			}
			if string(prefix) != tt.prefix || string(local) != tt.local {
				t.Fatalf("SplitASCIIQNameBytes(%q) prefix=%q local=%q, want prefix=%q local=%q", tt.in, prefix, local, tt.prefix, tt.local)
			}
		})
	}
}

func TestSplitQName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       string
		prefix   string
		local    string
		ok       bool
		prefixed bool
	}{
		{name: "local", in: "row", local: "row", ok: true},
		{name: "prefixed", in: "xs:int", prefix: "xs", local: "int", ok: true, prefixed: true},
		{name: "unicode", in: "\u00e9:name", prefix: "\u00e9", local: "name", ok: true, prefixed: true},
		{name: "empty"},
		{name: "leading_colon", in: ":bad"},
		{name: "trailing_colon", in: "bad:"},
		{name: "duplicate_colon", in: "a:b:c"},
		{name: "invalid_char", in: "bad@name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parts := SplitQName(tt.in)
			if parts.Valid != tt.ok || parts.Prefixed != tt.prefixed || parts.Prefix != tt.prefix || parts.Local != tt.local {
				t.Fatalf("SplitQName(%q) = (%q, %q, %v, %v), want (%q, %q, %v, %v)",
					tt.in, parts.Prefix, parts.Local, parts.Prefixed, parts.Valid, tt.prefix, tt.local, tt.prefixed, tt.ok)
			}
		})
	}
}

func TestXMLNameBytesAcceptUnicodeNames(t *testing.T) {
	t.Parallel()

	if !IsXMLNameBytes([]byte("é:name")) {
		t.Fatal("IsXMLNameBytes rejected unicode XML name")
	}
	if !IsNCNameBytes([]byte("é_name")) {
		t.Fatal("IsNCNameBytes rejected unicode NCName")
	}
	if IsNCNameBytes([]byte("é:name")) {
		t.Fatal("IsNCNameBytes accepted colon")
	}
	if !IsNMTOKENBytes([]byte("é.name")) {
		t.Fatal("IsNMTOKENBytes rejected unicode token")
	}
}

func TestIsLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{in: "en", want: true},
		{in: "en-US", want: true},
		{in: "x-private", want: true},
		{in: "", want: false},
		{in: " ", want: false},
		{in: "en-", want: false},
		{in: "1-en", want: false},
		{in: "abcdefghi", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			if got := IsLanguage(tt.in); got != tt.want {
				t.Fatalf("IsLanguage(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
