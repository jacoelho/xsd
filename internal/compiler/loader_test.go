package compiler

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/schemaast"
)

type resolverFunc func(req ResolveRequest) (io.ReadCloser, string, error)

func (f resolverFunc) Resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	return f(req)
}

type closeTrackingReader struct {
	*strings.Reader
	closeErr error
	closes   int
}

func (r *closeTrackingReader) Close() error {
	r.closes++
	return r.closeErr
}

func TestLoaderLoadNilLoader(t *testing.T) {
	var loader *Loader
	if _, err := loader.LoadDocuments("schema.xsd"); err == nil {
		t.Fatal("LoadDocuments() error = nil, want error")
	}
}

func TestLoaderLoadDocuments(t *testing.T) {
	loader := NewLoader(LoaderConfig{
		FS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:test"
	xmlns:tns="urn:test"
	elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
		},
	})
	docs, err := loader.LoadDocuments("schema.xsd")
	if err != nil {
		t.Fatalf("LoadDocuments() error = %v", err)
	}
	if docs == nil || len(docs.Documents) != 1 {
		t.Fatalf("LoadDocuments() documents = %v, want one document", docs)
	}
}

func TestLoaderLoadDocumentsRejectsImportWithoutNamespaceInNoNamespaceSchema(t *testing.T) {
	loader := NewLoader(LoaderConfig{
		FS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import schemaLocation="dep.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
			"dep.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="dep" type="xs:string"/>
</xs:schema>`)},
		},
	})

	_, err := loader.LoadDocuments("schema.xsd")
	if err == nil {
		t.Fatal("LoadDocuments() error = nil, want error")
	}
	if got, want := err.Error(), "schema without targetNamespace cannot use import without namespace attribute"; !strings.Contains(got, want) {
		t.Fatalf("LoadDocuments() error = %q, want %q", got, want)
	}
}

func TestLoaderLoadDocumentsRejectsIncludeWithDifferentNamespace(t *testing.T) {
	loader := NewLoader(LoaderConfig{
		FS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="dep.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
			"dep.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:dep">
  <xs:element name="dep" type="xs:string"/>
</xs:schema>`)},
		},
	})

	_, err := loader.LoadDocuments("schema.xsd")
	if err == nil {
		t.Fatal("LoadDocuments() error = nil, want error")
	}
	if got, want := err.Error(), "included schema namespace"; !strings.Contains(got, want) {
		t.Fatalf("LoadDocuments() error = %q, want %q", got, want)
	}
}

func TestLoaderLoadDocumentsIgnoresImportOfActiveNamespace(t *testing.T) {
	loader := NewLoader(LoaderConfig{
		FS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a" xmlns="urn:a">
  <xs:import schemaLocation="dep.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
			"dep.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import namespace="urn:a" schemaLocation="schema.xsd"/>
  <xs:element name="dep" type="xs:string"/>
</xs:schema>`)},
		},
	})

	docs, err := loader.LoadDocuments("schema.xsd")
	if err != nil {
		t.Fatalf("LoadDocuments() error = %v", err)
	}
	if got, want := len(docs.Documents), 2; got != want {
		t.Fatalf("documents = %d, want %d", got, want)
	}
}

func TestLoaderLoadIsIsolatedPerCall(t *testing.T) {
	type rotatingResolver struct {
		docs  []string
		calls int
	}
	resolveDoc := func(r *rotatingResolver, _ ResolveRequest) (io.ReadCloser, string, error) {
		doc := r.docs[r.calls%len(r.docs)]
		r.calls++
		return io.NopCloser(strings.NewReader(doc)), "schema.xsd", nil
	}

	resolver := &rotatingResolver{
		docs: []string{
			`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:tns="urn:test"><xs:element name="first" type="xs:string"/></xs:schema>`,
			`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:tns="urn:test"><xs:element name="second" type="xs:string"/></xs:schema>`,
		},
	}
	loader := NewLoader(LoaderConfig{
		Resolver: resolverFunc(func(req ResolveRequest) (io.ReadCloser, string, error) {
			return resolveDoc(resolver, req)
		}),
	})

	first, err := loader.LoadDocuments("schema.xsd")
	if err != nil {
		t.Fatalf("first LoadDocuments() error = %v", err)
	}
	second, err := loader.LoadDocuments("schema.xsd")
	if err != nil {
		t.Fatalf("second LoadDocuments() error = %v", err)
	}
	if first == second {
		t.Fatal("LoadDocuments() reused document set across top-level calls")
	}
	if !documentSetHasElement(second, schemaast.QName{Namespace: "urn:test", Local: "second"}) {
		t.Fatal("second LoadDocuments() did not reflect second resolver document")
	}
}

func TestLoaderLoadDocumentsClosesResolverReaderOnSuccess(t *testing.T) {
	reader := &closeTrackingReader{Reader: strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)}
	loader := NewLoader(LoaderConfig{
		Resolver: resolverFunc(func(req ResolveRequest) (io.ReadCloser, string, error) {
			return reader, "schema.xsd", nil
		}),
	})

	if _, err := loader.LoadDocuments("schema.xsd"); err != nil {
		t.Fatalf("LoadDocuments() error = %v", err)
	}
	if reader.closes != 1 {
		t.Fatalf("Close() calls = %d, want 1", reader.closes)
	}
}

func TestLoaderLoadDocumentsClosesResolverReaderOnParseError(t *testing.T) {
	reader := &closeTrackingReader{Reader: strings.NewReader(`<xs:schema`)}
	loader := NewLoader(LoaderConfig{
		Resolver: resolverFunc(func(req ResolveRequest) (io.ReadCloser, string, error) {
			return reader, "schema.xsd", nil
		}),
	})

	if _, err := loader.LoadDocuments("schema.xsd"); err == nil {
		t.Fatal("LoadDocuments() error = nil, want error")
	}
	if reader.closes != 1 {
		t.Fatalf("Close() calls = %d, want 1", reader.closes)
	}
}

func TestLoaderLoadDocumentsReturnsCloseErrorAfterSuccess(t *testing.T) {
	closeErr := errors.New("close failed")
	reader := &closeTrackingReader{
		Reader: strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		closeErr: closeErr,
	}
	loader := NewLoader(LoaderConfig{
		Resolver: resolverFunc(func(req ResolveRequest) (io.ReadCloser, string, error) {
			return reader, "schema.xsd", nil
		}),
	})

	_, err := loader.LoadDocuments("schema.xsd")
	if !errors.Is(err, closeErr) {
		t.Fatalf("LoadDocuments() error = %v, want close error", err)
	}
	if got, want := err.Error(), "close schema.xsd: close failed"; got != want {
		t.Fatalf("LoadDocuments() error = %q, want %q", got, want)
	}
	if reader.closes != 1 {
		t.Fatalf("Close() calls = %d, want 1", reader.closes)
	}
}

func TestLoaderLoadDocumentsParseErrorWinsOverCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	reader := &closeTrackingReader{
		Reader:   strings.NewReader(`<xs:schema`),
		closeErr: closeErr,
	}
	loader := NewLoader(LoaderConfig{
		Resolver: resolverFunc(func(req ResolveRequest) (io.ReadCloser, string, error) {
			return reader, "schema.xsd", nil
		}),
	})

	_, err := loader.LoadDocuments("schema.xsd")
	if err == nil {
		t.Fatal("LoadDocuments() error = nil, want parse error")
	}
	if errors.Is(err, closeErr) {
		t.Fatalf("LoadDocuments() error = %v, want parse error to win over close error", err)
	}
	if reader.closes != 1 {
		t.Fatalf("Close() calls = %d, want 1", reader.closes)
	}
}

func TestLoaderLoadDocumentsClosesResolverReaderOnDirectiveLoadError(t *testing.T) {
	reader := &closeTrackingReader{Reader: strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
</xs:schema>`)}
	loader := NewLoader(LoaderConfig{
		Resolver: resolverFunc(func(req ResolveRequest) (io.ReadCloser, string, error) {
			if req.SchemaLocation == "schema.xsd" {
				return reader, "schema.xsd", nil
			}
			return nil, "", errors.New("missing schema")
		}),
	})

	_, err := loader.LoadDocuments("schema.xsd")
	if err == nil || !strings.Contains(err.Error(), "load included schema missing.xsd") {
		t.Fatalf("LoadDocuments() error = %v, want include load error", err)
	}
	if reader.closes != 1 {
		t.Fatalf("Close() calls = %d, want 1", reader.closes)
	}
}

func TestLoaderLoadDocumentsCollectsIncludesAndImports(t *testing.T) {
	loader := NewLoader(LoaderConfig{
		FS: fstest.MapFS{
			"main.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:main"
	xmlns:main="urn:main"
	elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:import namespace="urn:dep" schemaLocation="dep.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
			"common.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:main"
	xmlns:main="urn:main"
	elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`)},
			"dep.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:dep"
	xmlns:dep="urn:dep"
	elementFormDefault="qualified">
  <xs:element name="dep" type="xs:string"/>
</xs:schema>`)},
		},
	})

	docs, err := loader.LoadDocuments("main.xsd")
	if err != nil {
		t.Fatalf("LoadDocuments() error = %v", err)
	}
	for _, qname := range []schemaast.QName{
		{Namespace: "urn:main", Local: "root"},
		{Namespace: "urn:main", Local: "common"},
		{Namespace: "urn:dep", Local: "dep"},
	} {
		if !documentSetHasElement(docs, qname) {
			t.Fatalf("LoadDocuments() missing element %v", qname)
		}
	}
}

func documentSetHasElement(docs *schemaast.DocumentSet, name schemaast.QName) bool {
	if docs == nil {
		return false
	}
	for _, doc := range docs.Documents {
		for _, decl := range doc.Decls {
			if decl.Kind == schemaast.DeclElement && decl.Name == name {
				return true
			}
		}
	}
	return false
}
