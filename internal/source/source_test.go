package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestKeyCanonicalizesLoadedSourceNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "relative path", in: "schemas/../types.xsd", want: filepath.Clean("types.xsd")},
		{name: "explicit local path resembling opaque URI", in: "./urn:types", want: "." + string(filepath.Separator) + "urn:types"},
		{name: "file uri", in: "file:///tmp/../tmp/types.xsd", want: filepath.Clean("/tmp/types.xsd")},
		{name: "opaque uri", in: "urn:types", want: "urn:types"},
		{name: "empty authority", in: "foo://", want: "foo://"},
		{name: "empty authority query", in: "foo://?q", want: "foo://?q"},
		{name: "empty authority path", in: "foo:///schema.xsd", want: "foo:///schema.xsd"},
		{name: "hierarchical uri", in: "https://example.test/schemas/../types.xsd", want: "https://example.test/types.xsd"},
		{name: "URL case and unreserved escape", in: "HTTPS://EXAMPLE.test/%74ypes.xsd", want: "https://example.test/types.xsd"},
		{name: "IPv6 address case preserves zone", in: "HTTP://[FE80::ABCD%25Eth0]:8080/types.xsd", want: "http://[fe80::abcd%25Eth0]:8080/types.xsd"},
		{name: "URL query unreserved escape", in: "https://example.test/types.xsd?q=%7e", want: "https://example.test/types.xsd?q=~"},
		{name: "opaque unreserved escape", in: "urn:item%7e", want: "urn:item~"},
		{name: "fragment unreserved escape", in: "https://example.test/types.xsd#%7e", want: "https://example.test/types.xsd#~"},
		{name: "empty fragment remains explicit", in: "https://example.test/types.xsd#", want: "https://example.test/types.xsd#"},
		{name: "URL reserved escape", in: "https://EXAMPLE.test/a%3fb.xsd", want: "https://example.test/a%3Fb.xsd"},
		{name: "URL query reserved escape", in: "https://example.test/types.xsd?q=%2f", want: "https://example.test/types.xsd?q=%2F"},
		{name: "URL empty path segments", in: "https://example.test/a//b.xsd", want: "https://example.test/a//b.xsd"},
		{name: "URL terminal parent preserves empty segments", in: "https://example.test/a///..", want: "https://example.test/a//"},
		{name: "file URI empty query remains URI", in: "file:///tmp/schema.xsd?", want: "file:///tmp/schema.xsd?"},
		{name: "file URI empty fragment remains URI", in: "file:///tmp/schema.xsd#", want: "file:///tmp/schema.xsd#"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Key(tt.in); got != tt.want {
				t.Fatalf("Key(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestReferenceBaseWithXMLBaseStripsFragment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		base      string
		reference string
		want      string
	}{
		{name: "fragment only", base: "https://example.test/schemas/root.xsd", reference: "#fragment", want: "https://example.test/schemas/root.xsd"},
		{name: "fragment only preserves hierarchical spelling", base: "HTTPS://EXAMPLE.test/%74.xsd#old", reference: "#new", want: "HTTPS://EXAMPLE.test/%74.xsd"},
		{name: "empty preserves hierarchical spelling", base: "HTTPS://EXAMPLE.test/%74.xsd#old", reference: "", want: "HTTPS://EXAMPLE.test/%74.xsd"},
		{name: "fragment only preserves opaque spelling", base: "URN:root%7e?x=%7e#old", reference: "#new", want: "URN:root%7e?x=%7e"},
		{name: "fragment only preserves empty authority spelling", base: "foo://?x=%7e#old", reference: "#new", want: "foo://?x=%7e"},
		{name: "fragment only preserves file URI spelling", base: "file:///tmp/%74.xsd#old", reference: "#new", want: "file:///tmp/%74.xsd"},
		{name: "relative", base: "https://example.test/schemas/root.xsd", reference: "sub/child.xsd#fragment", want: "https://example.test/schemas/sub/child.xsd"},
		{name: "absolute", base: "https://example.test/schemas/root.xsd", reference: "https://cdn.example.test/root.xsd#fragment", want: "https://cdn.example.test/root.xsd"},
		{name: "local fragment only", base: filepath.Join("schemas", "root.xsd"), reference: "#fragment", want: filepath.Clean(filepath.Join("schemas", "root.xsd"))},
		{name: "local hash remains a path character", base: filepath.Join("schemas", "root#old.xsd"), reference: "#new", want: filepath.Join("schemas", "root#old.xsd")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewReferenceBase(test.base).WithXMLBase(mustURIReference(t, test.reference))
			if err != nil {
				t.Fatalf("WithXMLBase() error = %v", err)
			}
			resolver, ok := got.ResolverValue()
			if !ok || resolver != test.want || !got.fallbackOK || got.fallback != test.want {
				t.Fatalf("WithXMLBase() = resolver %q/%v fallback %q/%v, want %q/true", resolver, ok, got.fallback, got.fallbackOK, test.want)
			}
		})
	}
	if _, err := uriref.Parse("#%zz"); err == nil {
		t.Fatal("Parse() accepted malformed fragment escape")
	}
}

func TestReferenceBaseUsesRFC2396ForResolverAndFallback(t *testing.T) {
	t.Parallel()

	base, err := NewReferenceBase("http://a/b/c/d;p?q").WithXMLBase(mustURIReference(t, "?y"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "http://a/b/c/?y"
	resolver, ok := base.ResolverValue()
	if !ok || resolver != want || !base.fallbackOK || base.fallback != want {
		t.Fatalf("WithXMLBase() = resolver %q/%v fallback %q/%v, want %q/true", resolver, ok, base.fallback, base.fallbackOK, want)
	}
}

func TestReferenceBasePreservesFallbackBackendSemantics(t *testing.T) {
	t.Parallel()

	t.Run("invalid arbitrary source identity", func(t *testing.T) {
		t.Parallel()

		base, err := NewReferenceBase("x:%zz").WithXMLBase(mustURIReference(t, "sub/"))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := base.ResolverValue(); ok || base.fallbackOK {
			t.Fatalf("WithXMLBase() left resolver/fallback available: %+v", base)
		}
	})

	t.Run("local query replaced by path", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join("schemas", "root.xsd")
		base, err := NewReferenceBase(root).WithXMLBase(mustURIReference(t, "?version=1"))
		if err != nil {
			t.Fatal(err)
		}
		if base.fallbackOK {
			t.Fatalf("query-bearing local fallback = %q/true, want unavailable", base.fallback)
		}
		base, err = base.WithXMLBase(mustURIReference(t, "child.xsd"))
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join("schemas", "child.xsd")
		resolver, resolverOK := base.ResolverValue()
		if !resolverOK || resolver != want || !base.fallbackOK || base.fallback != want {
			t.Fatalf("path replacement = resolver %q/%v fallback %q/%v, want %q/true", resolver, resolverOK, base.fallback, base.fallbackOK, want)
		}
	})

	t.Run("network authority does not recover local path", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(string(filepath.Separator), "tmp", "root.xsd")
		base, err := NewReferenceBase(root).WithXMLBase(mustURIReference(t, "//cdn.example/schemas/"))
		if err != nil {
			t.Fatal(err)
		}
		base, err = base.WithXMLBase(mustURIReference(t, "/tmp/child.xsd"))
		if err != nil {
			t.Fatal(err)
		}
		resolver, resolverOK := base.ResolverValue()
		if !resolverOK || resolver != "//cdn.example/tmp/child.xsd" || base.fallbackOK {
			t.Fatalf("network path = resolver %q/%v fallback %q/%v", resolver, resolverOK, base.fallback, base.fallbackOK)
		}
	})
}

func TestReferenceBaseKeepsLocalReservedCharactersAcrossNestedBases(t *testing.T) {
	t.Parallel()
	name := filepath.Join("schemas#archive", "root?old.xsd")
	base, err := NewReferenceBase(name).WithXMLBase(mustURIReference(t, "?version=1"))
	if err != nil {
		t.Fatal(err)
	}
	base, err = base.WithXMLBase(mustURIReference(t, "#fragment"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := base.ResolverValue(); !ok || got != name+"?version=1" {
		t.Fatalf("fragment base = %q/%v, want %q/true", got, ok, name+"?version=1")
	}
	base, err = base.WithXMLBase(mustURIReference(t, "child.xsd"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("schemas#archive", "child.xsd")
	if got, ok := base.ResolverValue(); !ok || got != want {
		t.Fatalf("relative base = %q/%v, want %q/true", got, ok, want)
	}
}

func TestResolveFromPreservesExtendedResolverInputs(t *testing.T) {
	t.Parallel()
	base, err := NewReferenceBase("schemas/root.xsd").WithXMLBase(mustURIReference(t, "sub/\x7f/"))
	if err != nil {
		t.Fatal(err)
	}
	called := false
	s := Bytes("schemas/root.xsd", nil).WithResolver(func(_ context.Context, gotBase, gotLocation string) (Source, error) {
		called = true
		if gotBase != "schemas/sub/\x7f/" || gotLocation != "child^name.xsd" {
			return Source{}, fmt.Errorf("resolver inputs = %q, %q", gotBase, gotLocation)
		}
		return Bytes("child.xsd", nil), nil
	})
	resolution, err := s.ResolveFrom(context.Background(), base, mustURIReference(t, "child^name.xsd"))
	if err != nil || !called {
		t.Fatalf("ResolveFrom() = %+v, %v; called %v", resolution, err, called)
	}
}

func TestResolveRejectsMalformedReferenceBeforeResolver(t *testing.T) {
	t.Parallel()
	called := false
	s := Bytes("root.xsd", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
		called = true
		return Bytes("child.xsd", nil), nil
	})
	if _, err := s.Resolve(context.Background(), "root.xsd", "http://[bad]/"); !IsReferenceResolutionError(err) {
		t.Fatalf("Resolve() error = %v, want reference-resolution error", err)
	}
	if called {
		t.Fatal("Resolve() called resolver for a malformed reference")
	}
}

func TestFileResolverUsesEscapedProjectionAfterCustomBoundary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "root.xsd")
	childName := "child^\x7f.xsd"
	child := filepath.Join(dir, childName)
	if err := os.WriteFile(root, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	resolution, err := File(root).Resolve(context.Background(), root, childName)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	resolved, ok := resolution.Source()
	if !ok || resolved.Name() != child {
		t.Fatalf("Resolve() source = %q/%v, want %q/true", resolved.Name(), ok, child)
	}
}

func mustURIReference(tb testing.TB, raw string) uriref.Reference {
	tb.Helper()
	reference, err := uriref.Parse(raw)
	if err != nil {
		tb.Fatalf("Parse(%q) error = %v", raw, err)
	}
	return reference
}

func TestKeyPreservesURIComponentBoundaries(t *testing.T) {
	t.Parallel()
	equal := [][2]string{
		{"https://example.test/schema.xsd?q=~", "https://example.test/schema.xsd?q=%7e"},
		{"urn:item~", "urn:item%7E"},
		{"https://example.test/schema.xsd#~", "https://example.test/schema.xsd#%7e"},
	}
	for _, pair := range equal {
		if left, right := Key(pair[0]), Key(pair[1]); left != right {
			t.Errorf("Key(%q) = %q, Key(%q) = %q; want equal", pair[0], left, pair[1], right)
		}
	}
	distinct := [][2]string{
		{"https://example.test/schema.xsd", "https://example.test/schema.xsd?"},
		{"https://example.test/schema.xsd", "https://example.test/schema.xsd#"},
		{"https://example.test/schema.xsd?q=/", "https://example.test/schema.xsd?q=%2F"},
		{"urn:item/", "urn:item%2F"},
		{"https://example.test/schema.xsd#/", "https://example.test/schema.xsd#%2F"},
		{"file:///tmp/schema.xsd", "file:///tmp/schema.xsd#"},
		{"foo:", "foo://"},
		{"file:", "file://"},
		{"http://[fe80::1%25ZoneA]/schema.xsd", "http://[fe80::1%25zonea]/schema.xsd"},
	}
	for _, pair := range distinct {
		if left, right := Key(pair[0]), Key(pair[1]); left == right {
			t.Errorf("Key(%q) and Key(%q) both = %q; want distinct", pair[0], pair[1], left)
		}
	}
}

func TestKeyCanonicalizesLongDotSegmentPath(t *testing.T) {
	t.Parallel()
	const segments = 10_000
	name := "https://example.test/" + strings.Repeat("a/", segments) + strings.Repeat("../", segments) + "schema.xsd"
	if got, want := Key(name), "https://example.test/schema.xsd"; got != want {
		t.Fatalf("Key(long dot path) = %q, want %q", got, want)
	}
}

func TestKeyPreservesLocalMarkerForSchemeShapedInvalidURI(t *testing.T) {
	t.Parallel()
	local := "." + string(filepath.Separator) + "x:%zz"
	if got := Key("./x:%zz"); got != local {
		t.Fatalf("Key(explicit local) = %q, want %q", got, local)
	}
	if got := Key("x:%zz"); got != "x:%zz" {
		t.Fatalf("Key(invalid scheme-bearing URI) = %q, want unchanged identity", got)
	}
	if Key("./x:%zz") == Key("x:%zz") {
		t.Fatal("explicit local and scheme-bearing identities collided")
	}
}

func TestBytesNilIsEmptySource(t *testing.T) {
	t.Parallel()
	data, err := Bytes("empty.xsd", nil).Read(context.Background(), 1)
	if err != nil {
		t.Fatalf("Bytes(nil).Read() error = %v", err)
	}
	if data == nil || len(data) != 0 {
		t.Fatalf("Bytes(nil).Read() = %#v, want non-nil empty slice", data)
	}
}

func TestBytesCopiesInputAndOutput(t *testing.T) {
	t.Parallel()
	input := []byte("schema")
	s := Bytes("schema.xsd", input)
	input[0] = 'X'
	first, err := s.Read(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'Y'
	second, err := s.Read(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(second); got != "schema" {
		t.Fatalf("second read = %q, want schema", got)
	}
}

func TestSourceResolve(t *testing.T) {
	t.Parallel()
	resolve := func(s Source, base, location string) (Source, string, bool, error) {
		resolution, err := s.Resolve(context.Background(), base, location)
		resolved, found := resolution.Source()
		return resolved, resolution.Target(), found, err
	}

	t.Run("missing resolver", func(t *testing.T) {
		t.Parallel()
		if _, target, found, err := resolve(Bytes("base.xsd", nil), "base.xsd", "child.xsd"); err != nil || found || target != "child.xsd" {
			t.Fatalf("Resolve() = found %v, error %v; want not found", found, err)
		}
	})

	t.Run("overrides acquired source resolver", func(t *testing.T) {
		t.Parallel()
		var locations []string
		resolver := Resolver(func(_ context.Context, _, location string) (Source, error) {
			locations = append(locations, location)
			if location == "child.xsd" {
				return File(filepath.Join(t.TempDir(), "cached-child.xsd")), nil
			}
			return Bytes(location, nil), nil
		})
		root := Bytes("root.xsd", nil).WithResolver(resolver)
		child, target, found, err := resolve(root, "root.xsd", "child.xsd")
		if err != nil || !found {
			t.Fatalf("Resolve(child) = found %v, error %v", found, err)
		}
		if target != Key(child.Name()) {
			t.Fatalf("Resolve(child) target = %q, want returned source identity", target)
		}
		if _, _, found, err := resolve(child, child.Name(), "grandchild.xsd"); err != nil || !found {
			t.Fatalf("Resolve(grandchild) = found %v, error %v", found, err)
		}
		if want := []string{"child.xsd", "grandchild.xsd"}; !slices.Equal(locations, want) {
			t.Fatalf("resolver locations = %v, want %v", locations, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		s := Bytes("base.xsd", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			return Source{}, xsderrors.ErrSchemaNotFound
		})
		if _, target, found, err := resolve(s, "base.xsd", "child.xsd"); err != nil || found || target != "child.xsd" {
			t.Fatalf("Resolve() = found %v, error %v; want not found", found, err)
		}
	})

	t.Run("custom miss preserves file fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		rootPath := filepath.Join(dir, "root.xsd")
		childPath := filepath.Join(dir, "child.xsd")
		if err := os.WriteFile(childPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		root := File(rootPath).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			return Source{}, xsderrors.ErrSchemaNotFound
		})
		child, target, found, err := resolve(root, root.Name(), "child.xsd")
		if err != nil || !found || child.Name() != childPath || target != Key(childPath) {
			t.Fatalf("Resolve() = child %q target %q found %v error %v", child.Name(), target, found, err)
		}
	})

	t.Run("custom fatal error does not use file fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		rootPath := filepath.Join(dir, "root.xsd")
		if err := os.WriteFile(filepath.Join(dir, "child.xsd"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		want := errors.New("resolver failed")
		root := File(rootPath).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			return Source{}, want
		})
		if _, _, found, err := resolve(root, root.Name(), "child.xsd"); found || !errors.Is(err, want) {
			t.Fatalf("Resolve() = found %v error %v, want fatal resolver error", found, err)
		}
	})

	t.Run("resolver returned file preserves descendant fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		childPath := filepath.Join(dir, "child.xsd")
		grandchildPath := filepath.Join(dir, "grandchild.xsd")
		if err := os.WriteFile(grandchildPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		root := Bytes("root.xsd", nil).WithResolver(func(_ context.Context, _, location string) (Source, error) {
			if location == "child.xsd" {
				return File(childPath), nil
			}
			return Source{}, xsderrors.ErrSchemaNotFound
		})
		child, _, found, err := resolve(root, root.Name(), "child.xsd")
		if err != nil || !found {
			t.Fatalf("Resolve(child) = found %v error %v", found, err)
		}
		grandchild, target, found, err := resolve(child, child.Name(), "grandchild.xsd")
		if err != nil || !found || grandchild.Name() != grandchildPath || target != Key(grandchildPath) {
			t.Fatalf("Resolve(grandchild) = name %q target %q found %v error %v", grandchild.Name(), target, found, err)
		}
	})

	t.Run("opaque base leaves relative reference unavailable", func(t *testing.T) {
		t.Parallel()
		sources := []Source{
			Bytes("urn:root", nil),
			Bytes("urn:root", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
				return Source{}, xsderrors.ErrSchemaNotFound
			}),
		}
		for _, s := range sources {
			if _, target, found, err := resolve(s, "urn:root", "child.xsd"); err != nil || found || target != "" {
				t.Fatalf("Resolve() = target %q found %v error %v, want unavailable", target, found, err)
			}
		}
	})

	t.Run("local backend limitations leave valid references unavailable", func(t *testing.T) {
		t.Parallel()
		sources := []Source{
			Bytes("schemas/root.xsd", nil),
			File(filepath.Join(t.TempDir(), "root.xsd")),
		}
		for _, s := range sources {
			for _, location := range []string{"child.xsd?version=1", "//example.test/child.xsd", "sub%2Fchild.xsd"} {
				if _, target, found, err := resolve(s, s.Name(), location); err != nil || found || target != "" {
					t.Fatalf("Resolve(%q) = target %q found %v error %v, want unavailable", location, target, found, err)
				}
			}
		}
	})

	t.Run("joined not found and fatal error", func(t *testing.T) {
		t.Parallel()
		fatal := errors.New("resolver cleanup failed")
		s := Bytes("base.xsd", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			return Source{}, errors.Join(xsderrors.ErrSchemaNotFound, fatal)
		})
		if _, _, found, err := resolve(s, "base.xsd", "child.xsd"); found || !errors.Is(err, fatal) {
			t.Fatalf("Resolve() = found %v, error %v; want fatal resolver error", found, err)
		}
	})

	t.Run("inherits resolver", func(t *testing.T) {
		t.Parallel()
		var bases []string
		resolver := Resolver(func(_ context.Context, base, location string) (Source, error) {
			bases = append(bases, base)
			return Bytes(location, nil), nil
		})
		root := Bytes("root.xsd", nil).WithResolver(resolver)
		child, _, found, err := resolve(root, "root.xsd", "child.xsd")
		if err != nil || !found {
			t.Fatalf("Resolve(child) = found %v, error %v", found, err)
		}
		if _, _, found, err := resolve(child, "child.xsd", "grandchild.xsd"); err != nil || !found {
			t.Fatalf("Resolve(grandchild) = found %v, error %v", found, err)
		}
		if want := []string{"root.xsd", "child.xsd"}; !slices.Equal(bases, want) {
			t.Fatalf("resolver bases = %v, want %v", bases, want)
		}
	})

	t.Run("preserves resolver error", func(t *testing.T) {
		t.Parallel()
		want := errors.New("resolver failed")
		s := Bytes("base.xsd", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			return Source{}, want
		})
		if _, _, found, err := resolve(s, "base.xsd", "child.xsd"); found || !errors.Is(err, want) {
			t.Fatalf("Resolve() = found %v, error %v; want %v", found, err, want)
		}
	})

	t.Run("resolver returned name owns identity", func(t *testing.T) {
		t.Parallel()
		s := Bytes("urn:root", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			return Bytes("urn:cache:child#v1", nil), nil
		})
		resolved, target, found, err := resolve(s, "urn:root", "relative?query#fragment")
		if err != nil || !found || resolved.Name() != "urn:cache:child#v1" || target != "urn:cache:child#v1" {
			t.Fatalf("Resolve() = name %q target %q found %v error %v", resolved.Name(), target, found, err)
		}
	})

	t.Run("resolver returned source requires name", func(t *testing.T) {
		t.Parallel()
		s := Bytes("base.xsd", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			return Source{}, nil
		})
		if _, _, found, err := resolve(s, "base.xsd", "child.xsd"); found || err == nil || !strings.Contains(err.Error(), "without a name") {
			t.Fatalf("Resolve() = found %v error %v, want unnamed-source error", found, err)
		}
	})

	t.Run("generic fragment is unavailable", func(t *testing.T) {
		t.Parallel()
		for _, location := range []string{"child.xsd#fragment", "child.xsd#"} {
			_, target, found, err := resolve(Bytes("base.xsd", nil), "base.xsd", location)
			if found || err != nil || target != "" {
				t.Fatalf("Resolve(%q) = target %q found %v error %v, want unavailable", location, target, found, err)
			}
		}
	})

	t.Run("file fragment is unavailable", func(t *testing.T) {
		t.Parallel()
		root := File(filepath.Join(t.TempDir(), "root.xsd"))
		for _, location := range []string{"child.xsd#fragment", "child.xsd#"} {
			_, target, found, err := resolve(root, root.Name(), location)
			if found || err != nil || target != "" {
				t.Fatalf("Resolve(%q) = target %q found %v error %v, want unavailable", location, target, found, err)
			}
		}
	})

	t.Run("malformed generic fragment is classified", func(t *testing.T) {
		t.Parallel()
		_, _, found, err := resolve(Bytes("base.xsd", nil), "base.xsd", "child.xsd#%zz")
		if found || !IsReferenceResolutionError(err) {
			t.Fatalf("Resolve() = found %v error %v, want reference-resolution error", found, err)
		}
	})

	t.Run("malformed location is rejected before custom resolver", func(t *testing.T) {
		t.Parallel()
		called := false
		s := Bytes("base.xsd", nil).WithResolver(func(_ context.Context, _, _ string) (Source, error) {
			called = true
			return Bytes("child.xsd", nil), nil
		})
		_, _, found, err := resolve(s, "base.xsd", "child.xsd#%zz")
		if found || !IsReferenceResolutionError(err) {
			t.Fatalf("Resolve() = found %v error %v, want malformed reference error", found, err)
		}
		if called {
			t.Fatal("Resolve() called custom resolver for malformed schemaLocation")
		}
	})

	t.Run("encoded hash is a path character", func(t *testing.T) {
		t.Parallel()
		_, target, found, err := resolve(Bytes("base.xsd", nil), "base.xsd", "child%23part.xsd")
		if found || err != nil || target != "child#part.xsd" {
			t.Fatalf("Resolve() = target %q found %v error %v", target, found, err)
		}
	})
}

func TestSourceReadLimit(t *testing.T) {
	t.Parallel()
	_, err := Bytes("schema.xsd", []byte("1234")).Read(context.Background(), 3)
	if !IsSchemaLimitError(err) {
		t.Fatalf("Read() error = %v, want schema limit", err)
	}
	if !IsSchemaLimitError(fmt.Errorf("wrapped: %w", err)) {
		t.Fatal("IsSchemaLimitError rejected wrapped error")
	}
}

func TestSourceReadWithZeroLimitDistinguishesEmptyAndOversize(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		source   Source
		wantData string
		wantOver bool
	}{
		{name: "empty bytes", source: Bytes("empty.xsd", nil)},
		{name: "empty opener", source: Opener("empty.xsd", func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		})},
		{name: "non-empty bytes", source: Bytes("schema.xsd", []byte("x")), wantOver: true},
		{name: "non-empty opener", source: Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("x")), nil
		}), wantOver: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.source.Acquire(context.Background(), 0)
			if result.LimitExceeded != tt.wantOver {
				t.Fatalf("Acquire() exceeded = %v, want %v", result.LimitExceeded, tt.wantOver)
			}
			if tt.wantOver {
				if !IsSchemaLimitError(result.Err) {
					t.Fatalf("Acquire() error = %v, want schema limit", result.Err)
				}
				return
			}
			if result.Err != nil || string(result.Data) != tt.wantData {
				t.Fatalf("Acquire() = %q, %v; want %q, nil", result.Data, result.Err, tt.wantData)
			}
		})
	}
}

func TestSourceAcquirePreservesBytesAndStageBeforeReadError(t *testing.T) {
	t.Parallel()

	want := errors.New("read failed")
	s := Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		return &dataErrorReader{data: []byte("schema"), err: want}, nil
	})
	result := s.Acquire(context.Background(), 100)
	if result.LimitExceeded || result.Stage != ReadStageRead || !errors.Is(result.Err, want) || string(result.Data) != "schema" {
		t.Fatalf("Acquire() = %+v", result)
	}
}

func TestSourceAcquirePreservesReadErrorAtByteLimit(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed at limit")
	result := Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		return &dataErrorReader{data: []byte("ab"), err: readErr}, nil
	}).Acquire(context.Background(),

		1)

	if !result.LimitExceeded || result.Stage != ReadStageRead || !errors.Is(result.Err, readErr) {
		t.Fatalf("Acquire() = %+v, want byte limit joined with read error", result)
	}
}

func TestSourceAcquireRejectsRepeatedEmptyReadsAndCloses(t *testing.T) {
	t.Parallel()

	reader := &emptyReadCloser{terminal: errors.New("unbounded empty reads")}
	result := Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		return reader, nil
	}).Acquire(context.Background(),

		1)

	if result.Stage != ReadStageRead || !errors.Is(result.Err, io.ErrNoProgress) {
		t.Fatalf("Acquire() = %+v, want read-stage io.ErrNoProgress", result)
	}
	if reader.reads != maxConsecutiveEmptySchemaReads || !reader.closed {
		t.Fatalf("reader = %d reads, closed %v; want %d, true", reader.reads, reader.closed, maxConsecutiveEmptySchemaReads)
	}
}

func TestSourceAcquireClosesReaderReturnedWithOpenError(t *testing.T) {
	t.Parallel()
	openErr := errors.New("open failed")
	closeErr := errors.New("close failed")
	reader := &trackingReadCloser{Reader: strings.NewReader("schema"), closeErr: closeErr}
	result := Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		return reader, openErr //nolint:nilnil // Exercise cleanup when an opener returns both values.
	}).Acquire(context.Background(),

		100)

	if result.Stage != ReadStageOpen || !errors.Is(result.Err, openErr) || !errors.Is(result.Err, closeErr) {
		t.Fatalf("Acquire() = %+v, want joined open and close errors", result)
	}
	if !reader.closed {
		t.Fatal("Acquire() did not close reader returned with open error")
	}
}

func TestSourceAcquireClassifiesOnlyPureOpenAbsence(t *testing.T) {
	t.Parallel()

	pure := Opener("missing.xsd", func(context.Context) (io.ReadCloser, error) {
		return nil, fmt.Errorf("open missing schema: %w", os.ErrNotExist)
	}).Acquire(context.Background(),

		100)

	if !pure.OpenNotFound {
		t.Fatalf("Acquire(pure absence) = %+v, want OpenNotFound", pure)
	}

	closeErr := errors.New("close failed")
	reader := &trackingReadCloser{Reader: strings.NewReader("schema"), closeErr: closeErr}
	mixed := Opener("missing.xsd", func(context.Context) (io.ReadCloser, error) {
		return reader, os.ErrNotExist //nolint:nilnil // Exercise cleanup when an opener returns both values.
	}).Acquire(context.Background(),

		100)

	if mixed.OpenNotFound || !errors.Is(mixed.Err, os.ErrNotExist) || !errors.Is(mixed.Err, closeErr) {
		t.Fatalf("Acquire(mixed absence) = %+v, want unsuppressible joined error", mixed)
	}
}

func TestMissingFileSourceReturnsPureOpenAbsence(t *testing.T) {
	t.Parallel()
	result := File(filepath.Join(t.TempDir(), "missing.xsd")).Acquire(context.Background(), 100)
	if !result.OpenNotFound || result.Stage != ReadStageOpen || !errors.Is(result.Err, os.ErrNotExist) {
		t.Fatalf("Acquire(missing file) = %+v, want pure open absence", result)
	}
	if errors.Is(result.Err, os.ErrInvalid) {
		t.Fatalf("Acquire(missing file) error = %v, contains typed-nil cleanup error", result.Err)
	}
}

func TestOpenerNormalizesTypedNilReaderOnOpenFailure(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing.xsd")
	result := Opener(missing, func(context.Context) (io.ReadCloser, error) {
		return os.Open(missing) //nolint:gosec // Test path is contained by t.TempDir.
	}).Acquire(context.Background(),

		100)

	if !result.OpenNotFound || result.Stage != ReadStageOpen || !errors.Is(result.Err, os.ErrNotExist) {
		t.Fatalf("Acquire(typed nil reader) = %+v, want pure open absence", result)
	}
	if errors.Is(result.Err, os.ErrInvalid) {
		t.Fatalf("Acquire(typed nil reader) error = %v, contains cleanup error", result.Err)
	}
}

func TestSourceAcquireRejectsNilOpener(t *testing.T) {
	t.Parallel()
	result := Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("open returned nil reader")
	}).Acquire(context.Background(),

		10)

	if result.Stage != ReadStageOpen || result.Err == nil {
		t.Fatalf("Acquire() = %+v, want open-stage error", result)
	}
}

type closeErrorReader struct {
	io.Reader
	err error
}

func (r closeErrorReader) Close() error { return r.err }

type trackingReadCloser struct {
	io.Reader
	closeErr error
	closed   bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return r.closeErr
}

type dataErrorReader struct {
	data []byte
	err  error
	done bool
}

func (r *dataErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func (*dataErrorReader) Close() error { return nil }

type emptyReadCloser struct {
	terminal error
	reads    int
	closed   bool
}

func (r *emptyReadCloser) Read([]byte) (int, error) {
	if r.reads > maxConsecutiveEmptySchemaReads {
		return 0, r.terminal
	}
	r.reads++
	return 0, nil
}

func (r *emptyReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestOpenerReturnsCloseErrorAfterSuccessfulRead(t *testing.T) {
	t.Parallel()
	want := errors.New("close failed")
	s := Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		return closeErrorReader{Reader: strings.NewReader("schema"), err: want}, nil
	})
	if _, err := s.Read(context.Background(), 100); !errors.Is(err, want) {
		t.Fatalf("Read() error = %v, want %v", err, want)
	}
}

func TestOpenerPreservesReadAndCloseErrors(t *testing.T) {
	t.Parallel()
	readErr := errors.New("read failed")
	closeErr := errors.New("close failed")
	reader := &trackingReadCloser{Reader: &dataErrorReader{data: []byte("schema"), err: readErr}, closeErr: closeErr}
	result := Opener("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		return reader, nil
	}).Acquire(context.Background(),

		100)

	if result.Stage != ReadStageRead || !errors.Is(result.Err, readErr) || !errors.Is(result.Err, closeErr) {
		t.Fatalf("Acquire() = %+v, want joined read and close errors", result)
	}
	if !reader.closed {
		t.Fatal("Acquire() did not close reader after read error")
	}
}

func TestResolveReference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		base     string
		location string
		want     string
	}{
		{name: "URL base", base: "https://example.test/a/main.xsd", location: "../types.xsd", want: "https://example.test/types.xsd"},
		{name: "URL rooted reference", base: "https://example.test/a/main.xsd", location: "/types.xsd", want: "https://example.test/types.xsd"},
		{name: "URL authority reference", base: "https://example.test/a/main.xsd", location: "//cdn.example.test/types.xsd", want: "https://cdn.example.test/types.xsd"},
		{name: "URL empty authority reference", base: "https://example.test/a/main.xsd?old", location: "//", want: "https://"},
		{name: "URL empty authority query reference", base: "https://example.test/a/main.xsd?old", location: "//?q", want: "https://?q"},
		{name: "URL empty authority path reference", base: "https://example.test/a/main.xsd?old", location: "///child", want: "https:///child"},
		{name: "file base", base: "schemas/main.xsd", location: "types.xsd", want: filepath.Clean("schemas/types.xsd")},
		{name: "explicit local resembling opaque URI", base: "root.xsd", location: "./urn:types", want: "." + string(filepath.Separator) + "urn:types"},
		{name: "percent decoded file", base: "schemas/main.xsd", location: "child%20name.xsd", want: filepath.Clean("schemas/child name.xsd")},
		{name: "directory base", base: "schemas/main.xsd", location: "sub/", want: filepath.Clean("schemas/sub") + string(filepath.Separator)},
		{name: "empty reference", base: "schemas/main.xsd", location: "", want: "schemas/main.xsd"},
		{name: "empty reference preserves directory base", base: "schemas/sub/", location: "", want: filepath.Clean("schemas/sub") + string(filepath.Separator)},
		{name: "terminal dot is directory", base: "schemas/main.xsd", location: "sub/.", want: filepath.Clean("schemas/sub") + string(filepath.Separator)},
		{name: "terminal parent is directory", base: "schemas/main.xsd", location: "sub/..", want: filepath.Clean("schemas") + string(filepath.Separator)},
		{name: "explicit local base permits literal percent", base: "./schemas%zz/main.xsd", location: "child.xsd", want: filepath.Clean("schemas%zz/child.xsd")},
		{name: "absolute local", base: "schemas/main.xsd", location: "/tmp/types.xsd", want: filepath.Clean("/tmp/types.xsd")},
		{name: "URL unreserved escape", base: "https://example.test/a/main.xsd", location: "%74ypes.xsd", want: "https://example.test/a/types.xsd"},
		{name: "absolute URL ignores opaque base", base: "urn:root", location: "HTTPS://EXAMPLE.test/%74ypes.xsd", want: "https://example.test/types.xsd"},
		{name: "file URI query preserved", base: "schemas/main.xsd", location: "file:///tmp/types.xsd?v=1", want: "file:///tmp/types.xsd?v=1"},
		{name: "file URI empty query preserved", base: "schemas/main.xsd", location: "file:///tmp/types.xsd?", want: "file:///tmp/types.xsd?"},
		{name: "URL encoded separator preserved", base: "https://example.test/a/main.xsd", location: "a%2fb.xsd", want: "https://example.test/a/a%2Fb.xsd"},
		{name: "URL empty segment preserved", base: "https://example.test/a/main.xsd", location: "x//y.xsd", want: "https://example.test/a/x//y.xsd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveReference(tt.base, tt.location)
			if err != nil || got != tt.want {
				t.Fatalf("ResolveReference() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	unavailable := []string{"child%2fname.xsd", "child%00name.xsd", "child.xsd?", "//"}
	if os.IsPathSeparator('\\') {
		unavailable = append(unavailable, "child%5cname.xsd")
	}
	for _, location := range unavailable {
		if got, err := ResolveReference("schemas/main.xsd", location); !IsReferenceUnavailable(err) || got != "" {
			t.Fatalf("ResolveReference(%q) = %q, %v; want unavailable", location, got, err)
		}
	}
	if !os.IsPathSeparator('\\') {
		want := filepath.Join("schemas", "child\\name.xsd")
		if got, err := ResolveReference("schemas/main.xsd", "child%5cname.xsd"); err != nil || got != want {
			t.Fatalf("ResolveReference(backslash) = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := ResolveReference("https://example.test/%zz/root.xsd", "child.xsd"); err == nil {
		t.Fatal("ResolveReference() accepted malformed scheme-bearing URI base")
	}
	if got, err := ResolveReference("urn:root", "child.xsd"); !IsReferenceUnavailable(err) || got != "" {
		t.Fatalf("ResolveReference() = %q, %v; want unavailable", got, err)
	}
	if runtime.GOOS != "windows" {
		for _, base := range []string{"/tmp/a#b/main.xsd", "/tmp/a?b/main.xsd"} {
			want := filepath.Join(filepath.Dir(base), "child.xsd")
			got, err := ResolveReference(base, "child.xsd")
			if err != nil || got != want {
				t.Fatalf("ResolveReference(%q) = %q, %v; want %q", base, got, err, want)
			}
		}
	}
}

func TestRemoveURLDotSegmentsPreservesEmptySegments(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"/a///..":  "/a//",
		"/a//..":   "/a/",
		"/a/..":    "/",
		"/a//.":    "/a//",
		"/a//b/..": "/a//",
		"///..":    "//",
	} {
		if got := removeURLDotSegments(in); got != want {
			t.Errorf("removeURLDotSegments(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveReferencePreservesChainedLocalDirectoryBase(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		base      string
		reference string
		wantBase  string
		wantChild string
	}{
		{
			name:      "empty reference",
			base:      "schemas/sub/",
			wantBase:  filepath.Clean("schemas/sub") + string(filepath.Separator),
			wantChild: filepath.Clean("schemas/sub/child.xsd"),
		},
		{
			name:      "terminal dot",
			base:      "schemas/root.xsd",
			reference: "sub/.",
			wantBase:  filepath.Clean("schemas/sub") + string(filepath.Separator),
			wantChild: filepath.Clean("schemas/sub/child.xsd"),
		},
		{
			name:      "terminal parent",
			base:      "schemas/sub/root.xsd",
			reference: "..",
			wantBase:  filepath.Clean("schemas") + string(filepath.Separator),
			wantChild: filepath.Clean("schemas/child.xsd"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, err := ResolveReference(tt.base, tt.reference)
			if err != nil || base != tt.wantBase {
				t.Fatalf("ResolveReference(base) = %q, %v; want %q", base, err, tt.wantBase)
			}
			child, err := ResolveReference(base, "child.xsd")
			if err != nil || child != tt.wantChild {
				t.Fatalf("ResolveReference(child) = %q, %v; want %q", child, err, tt.wantChild)
			}
		})
	}
}

func TestLocalFileURIPath(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"file://remotehost/tmp/schema.xsd",
		"file://user@localhost/tmp/schema.xsd",
		"file:///tmp/schema.xsd?",
		"file:///tmp/schema.xsd?v=1",
		"file:///tmp/schema.xsd#",
		"file:///tmp/schema.xsd#fragment",
		"https://example.test/schema.xsd",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := localFileURIPath(u, strings.IndexByte(raw, '#') >= 0); ok {
			t.Fatalf("localFileURIPath(%q) succeeded", raw)
		}
	}
	u, err := url.Parse("file:///tmp/a%2520schema.xsd")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(filepath.FromSlash("/tmp/a%20schema.xsd"))
	if got, ok := localFileURIPath(u, false); !ok || got != want {
		t.Fatalf("localFileURIPath(literal percent) = %q/%v, want %q/true", got, ok, want)
	}
	if runtime.GOOS == "windows" {
		u, err := url.Parse("file:///C:/schemas/a.xsd")
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := localFileURIPath(u, false); !ok || got != filepath.Clean(`C:\schemas\a.xsd`) {
			t.Fatalf("localFileURIPath(drive) = %q/%v", got, ok)
		}
	}
}

func TestFileURIQueriesHaveDistinctKeys(t *testing.T) {
	t.Parallel()
	first := Key("file:///tmp/schema.xsd?v=1")
	second := Key("file:///tmp/schema.xsd?v=2")
	if first == second {
		t.Fatalf("file URI query keys collapsed to %q", first)
	}
}

func TestFileURIUserinfoDoesNotAliasLocalPath(t *testing.T) {
	t.Parallel()
	uri := Key("file://user@localhost/tmp/schema.xsd")
	local := Key(filepath.Clean("/tmp/schema.xsd"))
	if uri == local {
		t.Fatalf("file URI userinfo and local path collapsed to %q", uri)
	}
}
