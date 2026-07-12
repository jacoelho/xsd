package compile

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaSetAggregateLimits(t *testing.T) {
	t.Parallel()

	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`)
	childData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location == "child.xsd" {
			return source.Bytes("child.xsd", childData), nil
		}
		return source.Source{}, xsderrors.ErrSchemaNotFound
	})
	root := source.Bytes("root.xsd", rootData).WithResolver(resolver)
	totalBytes := int64(len(rootData) + len(childData))

	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{name: "exact source count", opts: Options{MaxSchemaSources: 2}},
		{name: "source count exceeded", opts: Options{MaxSchemaSources: 1}, wantErr: true},
		{name: "exact total bytes", opts: Options{MaxSchemaTotalBytes: totalBytes}},
		{name: "total bytes exceeded", opts: Options{MaxSchemaTotalBytes: totalBytes - 1}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Compile(tt.opts, []source.Source{root})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Compile() error = %v", err)
				}
				return
			}
			if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Code != xsderrors.CodeSchemaLimit {
				t.Fatalf("Compile() error = %v, want schema limit", err)
			}
		})
	}
}

func TestSchemaSetRetriesSameKeyAfterOptionalCandidateIsMissing(t *testing.T) {
	t.Parallel()

	rootSchema := func(target string) []byte {
		return []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + target + `"><xs:include schemaLocation="child.xsd"/></xs:schema>`)
	}
	missing := source.Resolver(func(_, _ string) (source.Source, error) {
		return source.Opener("child.xsd", func() (io.ReadCloser, error) {
			return nil, os.ErrNotExist
		}), nil
	})
	child := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	found := source.Resolver(func(_, _ string) (source.Source, error) {
		return source.Bytes("child.xsd", child), nil
	})
	roots := []source.Source{
		source.Bytes("a.xsd", rootSchema("urn:a")).WithResolver(missing),
		source.Bytes("b.xsd", rootSchema("urn:b")).WithResolver(found),
	}
	if _, err := Compile(Options{}, roots); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSchemaSetAggregateByteLimitBoundsCurrentRead(t *testing.T) {
	t.Parallel()

	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`)
	childData := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	readBytes := 0
	resolver := source.Resolver(func(_, _ string) (source.Source, error) {
		return source.Opener("child.xsd", func() (io.ReadCloser, error) {
			return &countingReadCloser{Reader: strings.NewReader(childData), bytes: &readBytes}, nil
		}), nil
	})
	_, err := Compile(
		Options{MaxSchemaTotalBytes: int64(len(rootData) + 1)},
		[]source.Source{source.Bytes("root.xsd", rootData).WithResolver(resolver)},
	)
	if err == nil || !strings.Contains(err.Error(), "MaxSchemaTotalBytes") {
		t.Fatalf("Compile() error = %v, want aggregate byte limit", err)
	}
	if readBytes > 2 {
		t.Fatalf("child bytes read = %d, want at most remaining budget plus probe", readBytes)
	}
}

func TestSchemaSetAggregateByteLimitChargesSuppressedReadErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		open func() io.ReadCloser
	}{
		{name: "read error", open: func() io.ReadCloser {
			return &dataErrorReadCloser{data: []byte("x"), err: os.ErrNotExist}
		}},
		{name: "close error", open: func() io.ReadCloser {
			return closeErrorReadCloser{Reader: strings.NewReader("x"), err: os.ErrNotExist}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="a.xsd"/><xs:include schemaLocation="b.xsd"/></xs:schema>`)
			calls := 0
			resolver := source.Resolver(func(_, location string) (source.Source, error) {
				calls++
				return source.Opener(location, func() (io.ReadCloser, error) { return tt.open(), nil }), nil
			})
			_, err := Compile(
				Options{MaxSchemaTotalBytes: int64(len(rootData) + 1)},
				[]source.Source{source.Bytes("root.xsd", rootData).WithResolver(resolver)},
			)
			if err == nil || !strings.Contains(err.Error(), "MaxSchemaTotalBytes") {
				t.Fatalf("Compile() error = %v, want aggregate byte limit", err)
			}
			if calls != 2 {
				t.Fatalf("resolver calls = %d, want 2", calls)
			}
		})
	}
}

func TestSchemaReferencesResolveAndReadOneAtATime(t *testing.T) {
	t.Parallel()

	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="a.xsd"/><xs:include schemaLocation="b.xsd"/></xs:schema>`)
	childData := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	resolverCalls := 0
	readCalls := 0
	resolvedAfterRead := true
	resolver := source.Resolver(func(_, _ string) (source.Source, error) {
		resolverCalls++
		if resolverCalls > 1 && readCalls == 0 {
			resolvedAfterRead = false
		}
		return source.Opener("child.xsd", func() (io.ReadCloser, error) {
			readCalls++
			return io.NopCloser(strings.NewReader(childData)), nil
		}), nil
	})
	_, err := Compile(Options{}, []source.Source{source.Bytes("root.xsd", rootData).WithResolver(resolver)})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if resolverCalls != 2 || readCalls != 1 || !resolvedAfterRead {
		t.Fatalf("resolver calls = %d, reads = %d, interleaved = %v; want 2, 1, true", resolverCalls, readCalls, resolvedAfterRead)
	}
}

func TestSchemaReferenceLimitPrecedesResolverCalls(t *testing.T) {
	t.Parallel()

	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="a.xsd"/><xs:include schemaLocation="b.xsd"/></xs:schema>`)
	calls := 0
	resolver := source.Resolver(func(_, _ string) (source.Source, error) {
		calls++
		return source.Source{}, xsderrors.ErrSchemaNotFound
	})
	_, err := Compile(
		Options{MaxSchemaReferences: 1},
		[]source.Source{source.Bytes("root.xsd", rootData).WithResolver(resolver)},
	)
	if err == nil || !strings.Contains(err.Error(), "MaxSchemaReferences") {
		t.Fatalf("Compile() error = %v, want reference limit", err)
	}
	if calls != 0 {
		t.Fatalf("resolver calls = %d, want 0", calls)
	}
}

func TestSchemaReferenceLimitCountsImportsWithoutSchemaLocation(t *testing.T) {
	t.Parallel()

	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root"><xs:import namespace="urn:a"/><xs:import namespace="urn:b"/></xs:schema>`)
	root := source.Bytes("root.xsd", rootData)
	if _, err := Compile(Options{MaxSchemaReferences: 2}, []source.Source{root}); err != nil {
		t.Fatalf("Compile(exact) error = %v", err)
	}
	_, err := Compile(Options{MaxSchemaReferences: 1}, []source.Source{root})
	if err == nil || !strings.Contains(err.Error(), "MaxSchemaReferences") {
		t.Fatalf("Compile(over) error = %v, want reference limit", err)
	}
}

func TestSchemaReferenceLimitExemptsBuiltinXMLNamespaceImport(t *testing.T) {
	t.Parallel()

	schema := func(imports string) source.Source {
		data := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root"><xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="xml.xsd"/>` + imports + `</xs:schema>`)
		return source.Bytes("root.xsd", data)
	}
	if _, err := Compile(Options{MaxSchemaReferences: 1}, []source.Source{schema(`<xs:import namespace="urn:a"/>`)}); err != nil {
		t.Fatalf("Compile(exact) error = %v", err)
	}
	_, err := Compile(Options{MaxSchemaReferences: 1}, []source.Source{schema(`<xs:import namespace="urn:a"/><xs:import namespace="urn:b"/>`)})
	if err == nil || !strings.Contains(err.Error(), "MaxSchemaReferences") {
		t.Fatalf("Compile(over) error = %v, want reference limit", err)
	}
}

func TestSchemaSourceLimitStopsResolutionIncrementally(t *testing.T) {
	t.Parallel()

	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="a.xsd"/><xs:include schemaLocation="b.xsd"/></xs:schema>`)
	calls := 0
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		calls++
		return source.Bytes(location, []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
	})
	_, err := Compile(
		Options{MaxSchemaSources: 1},
		[]source.Source{source.Bytes("root.xsd", rootData).WithResolver(resolver)},
	)
	if err == nil || !strings.Contains(err.Error(), "MaxSchemaSources") {
		t.Fatalf("Compile() error = %v, want source limit", err)
	}
	if calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", calls)
	}
}

func TestSchemaTargetContextLimit(t *testing.T) {
	t.Parallel()

	schemas := map[string][]byte{
		"common.xsd": []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`),
		"leaf.xsd":   []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
	}
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		data, ok := schemas[location]
		if !ok {
			return source.Source{}, xsderrors.ErrSchemaNotFound
		}
		return source.Bytes(location, data), nil
	})
	root := func(name, target string) source.Source {
		data := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + target + `"><xs:include schemaLocation="common.xsd"/></xs:schema>`)
		return source.Bytes(name, data).WithResolver(resolver)
	}
	sources := []source.Source{root("a.xsd", "urn:a"), root("b.xsd", "urn:b")}
	for _, tt := range []struct {
		name    string
		limit   int
		wantErr bool
	}{
		{name: "exact", limit: 6},
		{name: "over", limit: 5, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Compile(Options{MaxSchemaTargetContexts: tt.limit}, sources)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "MaxSchemaTargetContexts") {
					t.Fatalf("Compile() error = %v, want target context limit", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
		})
	}
}

func TestSchemaInstantiatedNodeLimit(t *testing.T) {
	t.Parallel()

	t.Run("primary contexts", func(t *testing.T) {
		t.Parallel()

		data := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
		sources := []source.Source{source.Bytes("a.xsd", data), source.Bytes("b.xsd", data)}
		if _, err := Compile(Options{MaxSchemaInstantiatedNodes: 2}, sources); err != nil {
			t.Fatalf("Compile(exact) error = %v", err)
		}
		_, err := Compile(Options{MaxSchemaInstantiatedNodes: 1}, sources)
		if err == nil || !strings.Contains(err.Error(), "MaxSchemaInstantiatedNodes") {
			t.Fatalf("Compile(over) error = %v, want instantiated node limit", err)
		}
	})

	schemas := map[string][]byte{
		"common.xsd": []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`),
		"leaf.xsd":   []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
	}
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		return source.Bytes(location, schemas[location]), nil
	})
	root := func(name, target string) source.Source {
		data := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + target + `"><xs:include schemaLocation="common.xsd"/></xs:schema>`)
		return source.Bytes(name, data).WithResolver(resolver)
	}
	sources := []source.Source{root("a.xsd", "urn:a"), root("b.xsd", "urn:b")}
	for _, tt := range []struct {
		name    string
		limit   int
		wantErr bool
	}{
		{name: "exact", limit: 10},
		{name: "over", limit: 9, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Compile(Options{MaxSchemaInstantiatedNodes: tt.limit}, sources)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "MaxSchemaInstantiatedNodes") {
					t.Fatalf("Compile() error = %v, want instantiated node limit", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
		})
	}
}

type countingReadCloser struct {
	io.Reader
	bytes *int
}

func (r *countingReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	*r.bytes += n
	return n, err
}

func (*countingReadCloser) Close() error { return nil }

type dataErrorReadCloser struct {
	data []byte
	err  error
	done bool
}

func (r *dataErrorReadCloser) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func (*dataErrorReadCloser) Close() error { return nil }

type closeErrorReadCloser struct {
	io.Reader
	err error
}

func (r closeErrorReadCloser) Close() error { return r.err }
