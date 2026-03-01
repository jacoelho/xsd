package xsd_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
)

func TestSchemaSetCompileSingleRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	set := xsd.NewSchemaSet().WithLoadOptions(xsd.NewLoadOptions())
	if err := set.AddFS(fsys, "schema.xsd"); err != nil {
		t.Fatalf("AddFS() error = %v", err)
	}
	schema, err := set.Compile()
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<root xmlns="urn:test">ok</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestSchemaSetCompileMultipleRoots(t *testing.T) {
	fsysA := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootA" type="xs:string"/>
</xs:schema>`)},
	}
	fsysB := fstest.MapFS{
		"b.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootB" type="xs:string"/>
</xs:schema>`)},
	}

	set := xsd.NewSchemaSet()
	if err := set.AddFS(fsysA, "a.xsd"); err != nil {
		t.Fatalf("AddFS(a) error = %v", err)
	}
	if err := set.AddFS(fsysB, "b.xsd"); err != nil {
		t.Fatalf("AddFS(b) error = %v", err)
	}
	schema, err := set.Compile()
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootA xmlns="urn:test">ok</rootA>`)); err != nil {
		t.Fatalf("Validate(rootA) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootB xmlns="urn:test">ok</rootB>`)); err != nil {
		t.Fatalf("Validate(rootB) error = %v", err)
	}
}

func TestSchemaSetCompileMultipleRootsSameLocationDistinctFS(t *testing.T) {
	fsysA := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootA" type="xs:string"/>
</xs:schema>`)},
	}
	fsysB := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootB" type="xs:string"/>
</xs:schema>`)},
	}

	set := xsd.NewSchemaSet()
	if err := set.AddFS(fsysA, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(a) error = %v", err)
	}
	if err := set.AddFS(fsysB, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(b) error = %v", err)
	}
	schema, err := set.Compile()
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootA xmlns="urn:test">ok</rootA>`)); err != nil {
		t.Fatalf("Validate(rootA) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootB xmlns="urn:test">ok</rootB>`)); err != nil {
		t.Fatalf("Validate(rootB) error = %v", err)
	}
}

func TestSchemaSetCompileMultipleRootsSameLocationConflict(t *testing.T) {
	fsysA := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="shared" type="xs:string"/>
</xs:schema>`)},
	}
	fsysB := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="shared" type="xs:int"/>
</xs:schema>`)},
	}

	set := xsd.NewSchemaSet()
	if err := set.AddFS(fsysA, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(a) error = %v", err)
	}
	if err := set.AddFS(fsysB, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(b) error = %v", err)
	}
	_, err := set.Compile()
	if err == nil {
		t.Fatal("Compile() error = nil, want duplicate declaration error")
	}
	if !strings.Contains(err.Error(), "duplicate element") {
		t.Fatalf("Compile() error = %v, want duplicate element", err)
	}
}

func TestSchemaSetCompileWithRuntimeOptions(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)},
	}

	set := xsd.NewSchemaSet()
	if err := set.AddFS(fsys, "schema.xsd"); err != nil {
		t.Fatalf("AddFS() error = %v", err)
	}
	tightSchema, err := set.CompileWithRuntimeOptions(xsd.NewRuntimeOptions().WithInstanceMaxDepth(4))
	if err != nil {
		t.Fatalf("CompileWithRuntimeOptions() error = %v", err)
	}
	if err := tightSchema.Validate(strings.NewReader(deepAnyTypeDocument(8))); err == nil {
		t.Fatal("Validate() error = nil, want depth limit violation")
	}
}

func TestSchemaSetCompileWithoutRoots(t *testing.T) {
	set := xsd.NewSchemaSet()
	if _, err := set.Compile(); err == nil {
		t.Fatal("Compile() error = nil, want no roots error")
	}
}
