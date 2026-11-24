package uriref

import (
	"errors"
	"fmt"
	"strings"
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
		{name: "numeric IPv6 port", raw: "http://[::1]:80/", escaped: "http://[::1]:80/", valid: true, length: 16},
		{name: "nondigit IPv6 port", raw: "http://[::1]:x/"},
		{name: "valid IPv6", raw: "http://[::1]/", escaped: "http://[::1]/", valid: true, length: 13},
		{name: "valid IPv6 eight explicit groups", raw: "http://[1:2:3:4:5:6:7:8]/", escaped: "http://[1:2:3:4:5:6:7:8]/", valid: true, length: 25},
		{name: "leading compressed IPv6 cannot omit zero groups", raw: "http://[::1:2:3:4:5:6:7:8]/"},
		{name: "inner compressed IPv6 cannot omit zero groups", raw: "http://[1:2:3:4::5:6:7:8]/"},
		{name: "valid IPv6 with embedded IPv4", raw: "http://[1:2:3:4:5:6:1.2.3.4]/", escaped: "http://[1:2:3:4:5:6:1.2.3.4]/", valid: true, length: 29},
		{name: "IPv6 with embedded IPv4 cannot exceed eight groups", raw: "http://[1:2:3:4:5:6:7:1.2.3.4]/"},
		{name: "compressed IPv6 cannot omit zero groups", raw: "http://[1:2:3:4:5:6:7:8::]/"},
		{name: "empty authority with path", raw: "///", escaped: "///", valid: true, length: 3},
		{name: "empty authority with query", raw: "//?q", escaped: "//?q", valid: true, length: 4},
		{name: "bare empty authority", raw: "//", escaped: "//", valid: true, length: 2},
		{name: "bare absolute empty authority", raw: "foo://", escaped: "foo://", valid: true, length: 6},
		{name: "leading colon", raw: ":a"},
		{name: "empty opaque part", raw: "a:"},
		{name: "scheme with plus, minus and dot", raw: "a+b-c.d:x", escaped: "a+b-c.d:x", valid: true, length: 9},
		{name: "scheme starting with a digit", raw: "1a:b"},
		{name: "query-only opaque part", raw: "a:?q", escaped: "a:?q", valid: true, length: 4},
		{name: "empty opaque part before a fragment", raw: "a:#f"},
		{name: "bracket inside an opaque part", raw: "urn:a[b", escaped: "urn:a[b", valid: true, length: 7},
		{name: "opaque part starting with a left bracket", raw: "urn:[a"},
		{name: "opaque part starting with a right bracket", raw: "urn:]a"},
		{name: "bracket in a query", raw: "a?b[c", escaped: "a?b[c", valid: true, length: 5},
		{name: "bad escape in a query", raw: "a?%zz"},
		{name: "truncated escape in a fragment", raw: "a#%4"},
		{name: "bracket in a relative first segment", raw: "a[b"},
		{name: "right bracket in a relative first segment", raw: "a]b/c"},
		{name: "bracket in a later relative segment", raw: "a/b[c"},
		{name: "bracket in an absolute path", raw: "/a[b"},
		{name: "bracket in a scheme absolute path", raw: "file:/a]b"},
		{name: "bracket in a path after an authority", raw: "http://h/a[b"},
		{name: "IP literal without a right bracket", raw: "http://[::1/"},
		{name: "IP literal with reversed brackets", raw: "http://]::1[/"},
		{name: "IP literal with two left brackets", raw: "http://[[::1]/"},
		{name: "IP literal with two right brackets", raw: "http://[::1]]/"},
		{name: "authority with only a right bracket", raw: "http://a]/"},
		{name: "userinfo before an IP literal", raw: "http://user;x@[::1]/", escaped: "http://user;x@[::1]/", valid: true, length: 20},
		{name: "host before an IP literal", raw: "http://x[::1]/"},
		{name: "host between userinfo and an IP literal", raw: "http://user@x[::1]/"},
		{name: "at sign inside userinfo before an IP literal", raw: "http://u@v@[::1]/"},
		{name: "nondigit after an IP literal", raw: "http://[::1]x/"},
		{name: "IPv6 group of five digits", raw: "http://[12345::1]/"},
		{name: "IPv6 group with a nonhex digit", raw: "http://[g::1]/"},
		{name: "IPv6 empty group after compression", raw: "http://[1:::2]/"},
		{name: "IPv6 empty final group", raw: "http://[1:2:3:4:5:6:7:]/"},
		{name: "embedded IPv4 maximum octets", raw: "http://[::255.255.255.255]/", escaped: "http://[::255.255.255.255]/", valid: true, length: 27},
		{name: "embedded IPv4 with three parts", raw: "http://[::1.2.3]/"},
		{name: "embedded IPv4 with five parts", raw: "http://[::1.2.3.4.5]/"},
		{name: "embedded IPv4 octet above 255", raw: "http://[::256.1.1.1]/"},
		{name: "embedded IPv4 octet of four digits", raw: "http://[::1234.1.1.1]/"},
		{name: "embedded IPv4 octet of four digits with leading zeros", raw: "http://[::0001.1.1.1]/"},
		{name: "embedded IPv4 trailing dot", raw: "http://[::1.2.3.]/"},
		{name: "embedded IPv4 empty part", raw: "http://[::1..2.3]/"},
		{name: "embedded IPv4 leading dot", raw: "http://[::.1.2.3]/"},
		{name: "embedded IPv4 nondigit", raw: "http://[::1.2.3.a]/"},
		{name: "embedded IPv4 before compression", raw: "http://[1.2.3.4::]/"},
		{name: "embedded IPv4 before a final group", raw: "http://[::1.2.3.4:1]/"},
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
				_, parseErr := Parse(tt.raw)
				if !errors.Is(err, ErrInvalid) || !errors.Is(byteErr, ErrInvalid) || !errors.Is(parseErr, ErrInvalid) {
					t.Fatalf("invalid URI %q: Check=%v, bytes=%v, Parse=%v; want ErrInvalid", tt.raw, err, byteErr, parseErr)
				}
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
	for _, invalidUTF8 := range [][]byte{
		{'a', 0xff},
		{'a', 0xe2, 0x98, 'A'},
		{'a', 0xf0, 0x9f, 0x98, 'A'},
		{'a', 0xe2, 0x98},
	} {
		if _, err := Check(invalidUTF8); !errors.Is(err, ErrInvalid) {
			t.Errorf("Check(% x) error = %v, want ErrInvalid", invalidUTF8, err)
		}
		if _, err := Parse(string(invalidUTF8)); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(% x) error = %v, want ErrInvalid", invalidUTF8, err)
		}
	}
}

func TestResolveRelativePathAgainstEmptyAuthorityPath(t *testing.T) {
	t.Parallel()
	base, err := Parse("http://a")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := Parse("g")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(base, ref)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := resolved.Raw(), "http://a/g"; got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestEscapedEncodesTheCharactersXLinkExcludes(t *testing.T) {
	t.Parallel()
	const excluded = "<>\"{}|\\^`"
	for b := range byte(0x80) {
		raw := "a" + string(rune(b))
		ref, err := Parse(raw)
		if err != nil {
			continue
		}
		want := raw
		if b <= 0x20 || b == 0x7f || strings.IndexByte(excluded, b) >= 0 {
			want = fmt.Sprintf("a%%%02X", b)
		}
		if ref.Escaped() != want {
			t.Errorf("Parse(%q).Escaped() = %q, want %q", raw, ref.Escaped(), want)
		}
	}
}

func TestWithoutFragmentReturnsTheReferenceParseGivesItsPrefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ raw, want string }{
		{raw: "foo://#fragment", want: "foo://"},
		{raw: "//#fragment", want: "//"},
		{raw: "a?q#", want: "a?q"},
		{raw: "http://[::1]:80/p#f", want: "http://[::1]:80/p"},
		{raw: "#f", want: ""},
	} {
		got := mustParse(t, tc.raw).WithoutFragment()
		if want := mustParse(t, tc.want); got != want {
			t.Errorf("Parse(%q).WithoutFragment() = %#v, want Parse(%q) = %#v", tc.raw, got, tc.want, want)
		}
	}
}

func TestResolveSameDocumentAgainstEmptyAuthorityBase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ base, want string }{
		{base: "//#f", want: "//"},
		{base: "foo://#f", want: "foo://"},
	} {
		got, err := Resolve(mustParse(t, tc.base), Reference{})
		if want := mustParse(t, tc.want); err != nil || got != want {
			t.Errorf("Resolve(%q, \"\") = %q, %v; want %q", tc.base, got.Raw(), err, tc.want)
		}
	}
}

func TestResolveMergesARelativePathAtTheBasesLastSlash(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ base, want string }{
		{base: "/main.xsd", want: "/b.xsd"},
		{base: "file:/main.xsd", want: "file:/b.xsd"},
		{base: "main.xsd", want: "b.xsd"},
	} {
		got, err := Resolve(mustParse(t, tc.base), mustParse(t, "b.xsd"))
		if err != nil || got.Raw() != tc.want {
			t.Errorf("Resolve(%q, \"b.xsd\") = %q, %v; want %q", tc.base, got.Raw(), err, tc.want)
		}
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

func TestResolveKeepsEmptyLeadingSegmentsOutOfTheAuthority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ base, ref, want string }{
		{base: ".///", ref: "0", want: ".///0"},
		{base: "./a:/", ref: "0", want: "./a:/0"},
		{base: "x/../a:/", ref: "0", want: "./a:/0"},
		{base: ".//x", ref: "y", want: ".//y"},
		{base: "/.//x", ref: "y", want: "/.//y"},
		{base: "file:/.//x", ref: "y", want: "file:/.//y"},
		{base: "http://a/.//x", ref: "y", want: "http://a//y"},
	} {
		got, err := Resolve(mustParse(t, tc.base), mustParse(t, tc.ref))
		if err != nil || got.Raw() != tc.want {
			t.Errorf("Resolve(%q, %q) = %q, %v; want %q", tc.base, tc.ref, got.Raw(), err, tc.want)
		}
	}
}

func FuzzReferencePartsRoundTrip(f *testing.F) {
	for _, raw := range []string{"", "http://a", "//?q", "a?", "a#", "a/\u2603", "urn:opaque?query#fragment", "http://[::1]:80/"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		ref, err := Parse(raw)
		if err != nil {
			return
		}
		roundTrip, err := compose(ref.Parts())
		if err != nil || roundTrip.Raw() != raw || roundTrip.Parts() != ref.Parts() {
			t.Fatalf("compose(Parse(%q).Parts()) = %q, %v; want %q", raw, roundTrip.Raw(), err, raw)
		}
	})
}

func FuzzCheckAgreesWithParse(f *testing.F) {
	for _, raw := range []string{"", "http://a", "a b", "http://[::1]/", "http://[bad]/", "%zz", "a#b#c"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		_, checkErr := Check(raw)
		_, parseErr := Parse(raw)
		if (checkErr == nil) != (parseErr == nil) {
			t.Fatalf("Check(%q) error = %v, Parse error = %v", raw, checkErr, parseErr)
		}
	})
}

func FuzzReferenceOperationsReturnParsedReferences(f *testing.F) {
	f.Add("http://a/b/c/d;p?q", "../g#s")
	f.Add("//#f", "")
	f.Add("foo://#f", "#g")
	f.Add("/main.xsd", "b.xsd")
	f.Fuzz(func(t *testing.T, rawBase, rawRef string) {
		base, err := Parse(rawBase)
		if err != nil {
			return
		}
		without := base.WithoutFragment()
		if reparsed, reparseErr := Parse(without.Raw()); reparseErr != nil || reparsed != without || without.HasFragment() {
			t.Fatalf("Parse(%q).WithoutFragment() = %#v; Parse of its Raw = %#v, %v", rawBase, without, reparsed, reparseErr)
		}
		ref, err := Parse(rawRef)
		if err != nil {
			return
		}
		got, err := Resolve(base, ref)
		if errors.Is(err, ErrOpaqueBase) {
			return
		}
		if err != nil {
			t.Fatalf("Resolve(%q, %q) error = %v", rawBase, rawRef, err)
		}
		if reparsed, reparseErr := Parse(got.Raw()); reparseErr != nil || reparsed != got {
			t.Fatalf("Resolve(%q, %q) = %#v; Parse of its Raw = %#v, %v", rawBase, rawRef, got, reparsed, reparseErr)
		}
	})
}

func FuzzResolveKeepsInheritedComponents(f *testing.F) {
	f.Add("http://a/b/c/d;p?q", "../g#s")
	f.Add("http://a", "g")
	f.Add("urn:root?old", "?new")
	f.Add("http://a/b", "//c/d")
	f.Fuzz(func(t *testing.T, rawBase, rawRef string) {
		base, err := Parse(rawBase)
		if err != nil {
			return
		}
		ref, err := Parse(rawRef)
		if err != nil {
			return
		}
		got, err := Resolve(base, ref)
		if err != nil {
			return
		}
		b, r, g := base.Parts(), ref.Parts(), got.Parts()
		if g.HasAuthority && g.Path != "" && g.Path[0] != '/' {
			t.Fatalf("Resolve(%q, %q) = %q: authority followed by a relative path", rawBase, rawRef, got.Raw())
		}
		if g.HasFragment != r.HasFragment || g.Fragment != r.Fragment {
			t.Fatalf("Resolve(%q, %q) = %q: fragment differs from the reference's", rawBase, rawRef, got.Raw())
		}
		if !r.HasScheme && (g.HasScheme != b.HasScheme || g.Scheme != b.Scheme) {
			t.Fatalf("Resolve(%q, %q) = %q: scheme differs from the base's", rawBase, rawRef, got.Raw())
		}
		if !r.HasScheme && !r.HasAuthority && (g.HasAuthority != b.HasAuthority || g.Authority != b.Authority) {
			t.Fatalf("Resolve(%q, %q) = %q: authority differs from the base's", rawBase, rawRef, got.Raw())
		}
		relative := func(p Parts) bool { return !p.HasScheme && !p.HasAuthority && !strings.HasPrefix(p.Path, "/") }
		if relative(b) && relative(r) && strings.HasPrefix(g.Path, "/") {
			t.Fatalf("Resolve(%q, %q) = %q: relative base and reference resolve to an absolute path", rawBase, rawRef, got.Raw())
		}
	})
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
		{"?y", "http://a/b/c/d;p?y"},
		{"?y#s", "http://a/b/c/d;p?y#s"},
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
