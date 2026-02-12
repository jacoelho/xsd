package xsd_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
)

func TestLoadWithOptionsAllowsMissingImportLocation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	if _, err := xsd.LoadWithOptions(fsys, "main.xsd", xsd.NewLoadOptions()); err == nil {
		t.Fatal("LoadWithOptions() err = nil, want missing import error")
	}

	opts := xsd.NewLoadOptions().WithAllowMissingImportLocations(true)
	if _, err := xsd.LoadWithOptions(fsys, "main.xsd", opts); err != nil {
		t.Fatalf("LoadWithOptions() error = %v", err)
	}
}

func TestSchemaSetCompileAppliesRuntimeOptions(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	runtimeOpts := xsd.NewRuntimeOptions().WithInstanceMaxDepth(-1)
	loadOpts := xsd.NewLoadOptions().WithRuntimeOptions(runtimeOpts)
	set := xsd.NewSchemaSet(loadOpts)
	if err := set.AddFS(fsys, "schema.xsd"); err != nil {
		t.Fatalf("AddFS() error = %v", err)
	}
	if _, err := set.Compile(); err == nil {
		t.Fatal("Compile() error = nil, want invalid runtime options error")
	}
}

func TestLoadOptionsRejectsMixedRuntimeConfiguration(t *testing.T) {
	loadOptionsType := reflect.TypeOf(xsd.NewLoadOptions())
	forbiddenRuntimeSetters := []string{
		"WithMaxDFAStates",
		"WithMaxOccursLimit",
		"WithInstanceMaxDepth",
		"WithInstanceMaxAttrs",
		"WithInstanceMaxTokenSize",
		"WithInstanceMaxQNameInternEntries",
	}
	for _, method := range forbiddenRuntimeSetters {
		if _, ok := loadOptionsType.MethodByName(method); ok {
			t.Fatalf("LoadOptions should not export runtime setter %s; use WithRuntimeOptions", method)
		}
	}
	if _, ok := loadOptionsType.MethodByName("WithRuntimeOptions"); !ok {
		t.Fatal("LoadOptions should expose WithRuntimeOptions bridge")
	}

	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	opts := xsd.NewLoadOptions().WithRuntimeOptions(
		xsd.NewRuntimeOptions().WithInstanceMaxDepth(-1),
	)
	set := xsd.NewSchemaSet(opts)
	if err := set.AddFS(fsys, "schema.xsd"); err != nil {
		t.Fatalf("AddFS() error = %v", err)
	}
	if _, err := set.Compile(); err == nil {
		t.Fatal("Compile() error = nil, want runtime options validation error")
	}
}
