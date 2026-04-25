package compiler

import (
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
