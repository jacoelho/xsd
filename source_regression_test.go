package xsd_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestNilResolversDoNotSplitPublicSourceGraph(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="urn:child"/></xs:schema>`
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.xsd")
	if err := os.WriteFile(rootPath, []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}

	var nilResolver xsd.ResolverFunc
	var nilResolverPointer *xsd.ResolverFunc
	memory := xsd.Bytes("memory-root.xsd", []byte(schema))
	tests := []struct {
		name    string
		sources []xsd.SchemaSource
	}{
		{
			name:    "file explicit nil resolver",
			sources: []xsd.SchemaSource{xsd.File(rootPath), xsd.File(rootPath).WithResolver(nil)},
		},
		{
			name:    "typed nil resolver function",
			sources: []xsd.SchemaSource{memory, memory.WithResolver(nilResolver)},
		},
		{
			name:    "typed nil resolver function pointer",
			sources: []xsd.SchemaSource{memory, memory.WithResolver(nilResolverPointer)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaReferences: 1}, test.sources...); err != nil {
				t.Fatalf("CompileWithOptions() = %v, want equivalent nil resolver contexts to deduplicate", err)
			}
		})
	}
}

func TestCompileSourceLimitKeepsSourcePathForJoinedAcquisitionErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		readErr  error
		closeErr error
	}{
		{name: "read", readErr: errors.New("read failed at limit")},
		{name: "close", closeErr: errors.New("close failed at limit")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.CompileWithOptions(
				xsd.CompileOptions{MaxSchemaSourceBytes: 1},
				xsd.Open("schema.xsd", func() (io.ReadCloser, error) {
					return &sourceRegressionReadCloser{
						Reader:   strings.NewReader("ab"),
						readErr:  test.readErr,
						closeErr: test.closeErr,
					}, nil
				}),
			)
			if err == nil {
				t.Fatal("CompileWithOptions() succeeded beyond source limit")
			}
			wantCause := test.readErr
			if wantCause == nil {
				wantCause = test.closeErr
			}
			if !errors.Is(err, wantCause) {
				t.Fatalf("CompileWithOptions() error = %v, want cause %v", err, wantCause)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaLimit || diagnostic.Path() != "schema.xsd" {
				t.Fatalf("CompileWithOptions() diagnostic = %v, want schema limit at schema.xsd", err)
			}
			flat := xsderrors.Flatten(err)
			if len(flat) < 2 {
				t.Fatalf("Flatten(CompileWithOptions()) = %v, want limit plus acquisition cause", flat)
			}
			limit, ok := flat[0].(*xsderrors.Error) //nolint:errorlint // Flatten must expose the direct located diagnostic.
			if !ok || limit.Path() != "schema.xsd" {
				t.Fatalf("Flatten()[0] = %T %v, want located schema limit", flat[0], flat[0])
			}
		})
	}
}

type sourceRegressionReadCloser struct {
	*strings.Reader

	readErr  error
	closeErr error
}

func (r *sourceRegressionReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 && r.readErr != nil {
		return n, r.readErr
	}
	return n, err
}

func (r *sourceRegressionReadCloser) Close() error { return r.closeErr }
