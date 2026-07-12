package source

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

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
		{name: "file uri", in: "file:///tmp/../tmp/types.xsd", want: filepath.Clean("/tmp/types.xsd")},
		{name: "opaque uri", in: "urn:types", want: "urn:types"},
		{name: "hierarchical uri", in: "https://example.test/schemas/../types.xsd", want: "https://example.test/types.xsd"},
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

func TestBytesNilIsEmptySource(t *testing.T) {
	t.Parallel()
	data, err := Bytes("empty.xsd", nil).Read(1)
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
	first, err := s.Read(100)
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'Y'
	second, err := s.Read(100)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(second); got != "schema" {
		t.Fatalf("second read = %q, want schema", got)
	}
}

func TestSourceResolve(t *testing.T) {
	t.Parallel()

	t.Run("missing resolver", func(t *testing.T) {
		t.Parallel()
		if _, found, err := Bytes("base.xsd", nil).Resolve("child.xsd"); err != nil || found {
			t.Fatalf("Resolve() = found %v, error %v; want not found", found, err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		s := Bytes("base.xsd", nil).WithResolver(func(_, _ string) (Source, error) {
			return Source{}, xsderrors.ErrSchemaNotFound
		})
		if _, found, err := s.Resolve("child.xsd"); err != nil || found {
			t.Fatalf("Resolve() = found %v, error %v; want not found", found, err)
		}
	})

	t.Run("inherits resolver", func(t *testing.T) {
		t.Parallel()
		var bases []string
		resolver := Resolver(func(base, location string) (Source, error) {
			bases = append(bases, base)
			return Bytes(location, nil), nil
		})
		root := Bytes("root.xsd", nil).WithResolver(resolver)
		child, found, err := root.Resolve("child.xsd")
		if err != nil || !found {
			t.Fatalf("Resolve(child) = found %v, error %v", found, err)
		}
		if _, found, err := child.Resolve("grandchild.xsd"); err != nil || !found {
			t.Fatalf("Resolve(grandchild) = found %v, error %v", found, err)
		}
		if want := []string{"root.xsd", "child.xsd"}; !slices.Equal(bases, want) {
			t.Fatalf("resolver bases = %v, want %v", bases, want)
		}
	})

	t.Run("preserves resolver error", func(t *testing.T) {
		t.Parallel()
		want := errors.New("resolver failed")
		s := Bytes("base.xsd", nil).WithResolver(func(_, _ string) (Source, error) {
			return Source{}, want
		})
		if _, found, err := s.Resolve("child.xsd"); found || !errors.Is(err, want) {
			t.Fatalf("Resolve() = found %v, error %v; want %v", found, err, want)
		}
	})
}

func TestSourceReadLimit(t *testing.T) {
	t.Parallel()
	_, err := Bytes("schema.xsd", []byte("1234")).Read(3)
	if !IsSchemaLimitError(err) {
		t.Fatalf("Read() error = %v, want schema limit", err)
	}
	if !IsSchemaLimitError(fmt.Errorf("wrapped: %w", err)) {
		t.Fatal("IsSchemaLimitError rejected wrapped error")
	}
}

func TestLimitedReaderRejectsNilAndOversize(t *testing.T) {
	t.Parallel()
	if _, err := LimitedReader("schema.xsd", nil, 10).Read(10); !errors.Is(err, ErrNilReader) {
		t.Fatalf("nil reader error = %v", err)
	}
	if _, err := LimitedReader("schema.xsd", strings.NewReader("1234"), 3).Read(3); !IsSchemaLimitError(err) {
		t.Fatalf("oversize reader error = %v", err)
	}
}

type closeErrorReader struct {
	io.Reader
	err error
}

func (r closeErrorReader) Close() error { return r.err }

func TestOpenerReturnsCloseErrorAfterSuccessfulRead(t *testing.T) {
	t.Parallel()
	want := errors.New("close failed")
	s := Opener("schema.xsd", func() (io.ReadCloser, error) {
		return closeErrorReader{Reader: strings.NewReader("schema"), err: want}, nil
	})
	if _, err := s.Read(100); !errors.Is(err, want) {
		t.Fatalf("Read() error = %v, want %v", err, want)
	}
}

func TestLocationKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		baseName string
		baseKey  string
		location string
		want     []string
	}{
		{name: "URL base", baseName: "https://example.test/a/main.xsd", location: "../types.xsd", want: []string{"https://example.test/types.xsd", filepath.Clean("../types.xsd")}},
		{name: "file base", baseName: "schemas/main.xsd", baseKey: filepath.Clean("schemas/main.xsd"), location: "types.xsd", want: []string{filepath.Clean("schemas/types.xsd"), filepath.Clean("types.xsd")}},
		{name: "deduplicated", baseName: "main.xsd", baseKey: filepath.Clean("main.xsd"), location: "types.xsd", want: []string{filepath.Clean("types.xsd")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := LocationKeys(tt.baseName, tt.baseKey, tt.location); !slices.Equal(got, tt.want) {
				t.Fatalf("LocationKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeSchemaLocation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "trim", in: "  types.xsd  ", want: "types.xsd", ok: true},
		{name: "collapse", in: "a\t\nb", want: "a b", ok: true},
		{name: "empty", in: " \t\r\n "},
		{name: "NBSP", in: "\u00a0types.xsd\u00a0", want: "\u00a0types.xsd\u00a0", ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := NormalizeSchemaLocation(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("NormalizeSchemaLocation(%q) = %q/%v, want %q/%v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestResolveLocalSchemaLocation(t *testing.T) {
	t.Parallel()
	if got, ok := ResolveLocalSchemaLocation("schemas/main.xsd", "types.xsd"); !ok || got != filepath.Clean("schemas/types.xsd") {
		t.Fatalf("ResolveLocalSchemaLocation() = %q/%v", got, ok)
	}
	if _, ok := ResolveLocalSchemaLocation("schemas/main.xsd", "https://example.test/types.xsd"); ok {
		t.Fatal("ResolveLocalSchemaLocation accepted remote URI")
	}
	if _, ok := ResolveLocalSchemaLocation("/tmp/main.xsd", "file://example.com/tmp/types.xsd"); ok {
		t.Fatal("ResolveLocalSchemaLocation accepted non-local file URI host")
	}
	want := filepath.Clean(filepath.FromSlash("/tmp/types.xsd"))
	for _, location := range []string{"file:///tmp/types.xsd", "file://localhost/tmp/types.xsd"} {
		got, ok := ResolveLocalSchemaLocation("/tmp/main.xsd", location)
		if !ok || got != want {
			t.Fatalf("ResolveLocalSchemaLocation(%q) = %q/%v, want %q/true", location, got, ok, want)
		}
	}
	if runtime.GOOS == "windows" {
		got, ok := ResolveLocalSchemaLocation(`C:\schemas\main.xsd`, `C:\schemas\types.xsd`)
		if want := filepath.Clean(`C:\schemas\types.xsd`); !ok || got != want {
			t.Fatalf("ResolveLocalSchemaLocation(drive) = %q/%v, want %q/true", got, ok, want)
		}
	}
}

func TestLocalFileURIPath(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"file://remotehost/tmp/schema.xsd", "https://example.test/schema.xsd"} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := LocalFileURIPath(u); ok {
			t.Fatalf("LocalFileURIPath(%q) succeeded", raw)
		}
	}
}
