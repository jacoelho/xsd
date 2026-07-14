package uriref

import (
	"errors"
	"testing"
)

func TestParseAndEscape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		escaped string
		valid   bool
		length  int
	}{
		{name: "empty", valid: true},
		{name: "absolute", raw: "https://example.test/a%20b", escaped: "https://example.test/a%20b", valid: true, length: 26},
		{name: "space", raw: "a b", escaped: "a%20b", valid: true, length: 3},
		{name: "caret", raw: "a^b", escaped: "a%5Eb", valid: true, length: 3},
		{name: "backslash", raw: `a\b`, escaped: "a%5Cb", valid: true, length: 3},
		{name: "del", raw: "a\x7fb", escaped: "a%7Fb", valid: true, length: 3},
		{name: "unicode", raw: "a/\u2603", escaped: "a/%E2%98%83", valid: true, length: 3},
		{name: "relative colon after slash", raw: "./a:", escaped: "./a:", valid: true, length: 4},
		{name: "valid IPv6", raw: "http://[::1]/", escaped: "http://[::1]/", valid: true, length: 13},
		{name: "empty authority with path", raw: "///", escaped: "///", valid: true, length: 3},
		{name: "empty authority with query", raw: "//?q", escaped: "//?q", valid: true, length: 4},
		{name: "leading colon", raw: ":a"},
		{name: "empty opaque part", raw: "a:"},
		{name: "bare empty authority", raw: "//"},
		{name: "bare absolute empty authority", raw: "foo://"},
		{name: "bad escape", raw: "%xz"},
		{name: "duplicate fragment", raw: "a#b#c"},
		{name: "invalid IPv6", raw: "http://[bad]/"},
		{name: "bracketed IPv4", raw: "http://[127.0.0.1]/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			characters, err := Check(tt.raw)
			byteCharacters, byteErr := Check([]byte(tt.raw))
			if (err == nil) != tt.valid || (byteErr == nil) != tt.valid {
				t.Fatalf("Check(%q) = %d, %v; bytes = %d, %v; valid %v", tt.raw, characters, err, byteCharacters, byteErr, tt.valid)
			}
			if !tt.valid {
				return
			}
			if characters != tt.length || byteCharacters != tt.length {
				t.Fatalf("Check(%q) characters = %d/%d, want %d", tt.raw, characters, byteCharacters, tt.length)
			}
			ref, err := Parse(tt.raw)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.raw, err)
			}
			if ref.Raw() != tt.raw || ref.Escaped() != tt.escaped {
				t.Fatalf("Parse(%q) = raw %q escaped %q, want %q/%q", tt.raw, ref.Raw(), ref.Escaped(), tt.raw, tt.escaped)
			}
		})
	}
	invalidUTF8 := []byte{'a', 0xff}
	if _, err := Check(invalidUTF8); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Check(invalid UTF-8) error = %v, want ErrInvalid", err)
	}
}

func TestReferenceFragments(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"a#fragment", "a#"} {
		ref, err := Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !ref.HasFragment() || ref.WithoutFragment().Raw() != "a" {
			t.Fatalf("fragment operations on %q failed", raw)
		}
	}
	ref, err := Parse("a%23b")
	if err != nil {
		t.Fatal(err)
	}
	if ref.HasFragment() || ref.WithoutFragment() != ref {
		t.Fatal("encoded hash was treated as a fragment")
	}
}

func TestResolveRFC2396AppendixC(t *testing.T) {
	t.Parallel()
	base := mustParse(t, "http://a/b/c/d;p?q")
	tests := []struct{ ref, want string }{
		{"g:h", "g:h"},
		{"g", "http://a/b/c/g"},
		{"./g", "http://a/b/c/g"},
		{"g/", "http://a/b/c/g/"},
		{"/g", "http://a/g"},
		{"//g", "http://g"},
		{"?y", "http://a/b/c/?y"},
		{"g?y", "http://a/b/c/g?y"},
		{"#s", "http://a/b/c/d;p?q#s"},
		{"g#s", "http://a/b/c/g#s"},
		{"g?y#s", "http://a/b/c/g?y#s"},
		{";x", "http://a/b/c/;x"},
		{"g;x", "http://a/b/c/g;x"},
		{"g;x?y#s", "http://a/b/c/g;x?y#s"},
		{".", "http://a/b/c/"},
		{"./", "http://a/b/c/"},
		{"..", "http://a/b/"},
		{"../", "http://a/b/"},
		{"../g", "http://a/b/g"},
		{"../..", "http://a/"},
		{"../../", "http://a/"},
		{"../../g", "http://a/g"},
		{"", "http://a/b/c/d;p?q"},
		{"../../../g", "http://a/../g"},
		{"../../../../g", "http://a/../../g"},
		{"/./g", "http://a/./g"},
		{"/../g", "http://a/../g"},
		{"g.", "http://a/b/c/g."},
		{".g", "http://a/b/c/.g"},
		{"g..", "http://a/b/c/g.."},
		{"..g", "http://a/b/c/..g"},
		{"./../g", "http://a/b/g"},
		{"./g/.", "http://a/b/c/g/"},
		{"g/./h", "http://a/b/c/g/h"},
		{"g/../h", "http://a/b/c/h"},
		{"g;x=1/./y", "http://a/b/c/g;x=1/y"},
		{"g;x=1/../y", "http://a/b/c/y"},
		{"g?y/./x", "http://a/b/c/g?y/./x"},
		{"g?y/../x", "http://a/b/c/g?y/../x"},
		{"g#s/./x", "http://a/b/c/g#s/./x"},
		{"g#s/../x", "http://a/b/c/g#s/../x"},
		{"http:g", "http:g"},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			t.Parallel()
			got, err := Resolve(base, mustParse(t, tt.ref))
			if err != nil || got.Raw() != tt.want {
				t.Fatalf("Resolve(%q) = %q, %v; want %q", tt.ref, got.Raw(), err, tt.want)
			}
		})
	}
}

func TestResolveOpaqueBasePolicy(t *testing.T) {
	t.Parallel()
	base := mustParse(t, "urn:root?old")
	query, err := Resolve(base, mustParse(t, "?new"))
	if err != nil || query.Raw() != "urn:root?new" {
		t.Fatalf("Resolve(query) = %q, %v", query.Raw(), err)
	}
	if _, resolveErr := Resolve(base, mustParse(t, "child")); !errors.Is(resolveErr, ErrOpaqueBase) {
		t.Fatalf("Resolve(relative opaque) error = %v, want ErrOpaqueBase", resolveErr)
	}
	authority, err := Resolve(base, mustParse(t, "//example.test/a"))
	if err != nil || authority.Raw() != "urn://example.test/a" {
		t.Fatalf("Resolve(authority) = %q, %v", authority.Raw(), err)
	}
}

func mustParse(tb testing.TB, raw string) Reference {
	tb.Helper()
	ref, err := Parse(raw)
	if err != nil {
		tb.Fatalf("Parse(%q) error = %v", raw, err)
	}
	return ref
}

var (
	benchmarkReference  Reference
	benchmarkCharacters int
	errBenchmark        error
)

func BenchmarkReference(b *testing.B) {
	for _, value := range []string{
		"schema.xsd",
		"https://example.com/schema.xsd",
		"path/schema%20name.xsd",
		"a^b",
		"a/\u2603",
	} {
		b.Run("Check/"+value, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkCharacters, errBenchmark = Check(value)
			}
		})
		b.Run("Parse/"+value, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkReference, errBenchmark = Parse(value)
			}
		})
	}
	b.Run("Escaped/ASCII", func(b *testing.B) {
		ref := mustParse(b, "schema.xsd")
		b.ReportAllocs()
		for b.Loop() {
			_ = ref.Escaped()
		}
	})
	b.Run("Escaped/extended", func(b *testing.B) {
		ref := mustParse(b, "a^b")
		b.ReportAllocs()
		for b.Loop() {
			_ = ref.Escaped()
		}
	})
}
