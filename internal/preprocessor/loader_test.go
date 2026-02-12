package preprocessor

import (
	"testing"
	"testing/fstest"
)

func TestLoaderLoadNilLoader(t *testing.T) {
	var loader *Loader
	if _, err := loader.Load("schema.xsd"); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoaderLoad(t *testing.T) {
	loader := NewLoader(Config{
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
