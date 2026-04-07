package compiler

import (
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/model"
)

type resolverFunc func(req ResolveRequest) (io.ReadCloser, string, error)

func (f resolverFunc) Resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	return f(req)
}

func TestLoaderLoadNilLoader(t *testing.T) {
	var loader *Loader
	if _, err := loader.Load("schema.xsd"); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoaderLoad(t *testing.T) {
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
	schema, err := loader.Load("schema.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if schema == nil {
		t.Fatal("Load() returned nil schema")
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

	first, err := loader.Load("schema.xsd")
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	second, err := loader.Load("schema.xsd")
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if first == second {
		t.Fatal("Load() reused cached schema across top-level calls")
	}
	if second.ElementDecls[model.QName{Namespace: "urn:test", Local: "second"}] == nil {
		t.Fatal("second Load() did not reflect second resolver document")
	}
}

func TestLoaderLoadMergesIncludesAndImports(t *testing.T) {
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

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, qname := range []model.QName{
		{Namespace: "urn:main", Local: "root"},
		{Namespace: "urn:main", Local: "common"},
		{Namespace: "urn:dep", Local: "dep"},
	} {
		if schema.ElementDecls[qname] == nil {
			t.Fatalf("Load() missing merged element %v", qname)
		}
	}
}
