package xsd_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
)

func TestPrepareAndBuild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	prepared, err := xsd.Prepare(fsys, "schema.xsd")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil {
		t.Fatal("Prepare() returned nil")
	}

	order := prepared.GlobalElementOrder()
	if len(order) != 1 {
		t.Fatalf("GlobalElementOrder() length = %d, want 1", len(order))
	}
	if order[0].Local != "root" {
		t.Fatalf("GlobalElementOrder()[0].Local = %q, want root", order[0].Local)
	}

	schema, err := prepared.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if schema == nil {
		t.Fatal("Build() returned nil")
	}

	doc := `<root xmlns="urn:test">ok</root>`
	if err := schema.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPreparedSchemaBuildWithOptionsRejectsInvalidRuntimeLimits(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	prepared, err := xsd.Prepare(fsys, "schema.xsd")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	opts := xsd.NewRuntimeOptions().WithInstanceMaxDepth(-1)
	if _, err := prepared.BuildWithOptions(opts); err == nil {
		t.Fatal("BuildWithOptions() error = nil, want invalid runtime options error")
	}
}

func TestLoadWithOptionsRejectsInvalidSchemaLimits(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	opts := xsd.NewLoadOptions().WithSchemaMaxDepth(-1)
	if _, err := xsd.LoadWithOptions(fsys, "schema.xsd", opts); err == nil {
		t.Fatal("LoadWithOptions() error = nil, want invalid schema options error")
	}
}
