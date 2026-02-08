package xsd_test

import (
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

	if _, err := xsd.Load(fsys, "main.xsd"); err == nil {
		t.Fatal("Load() err = nil, want missing import error")
	}

	opts := xsd.NewLoadOptions().WithAllowMissingImportLocations(true)
	if _, err := xsd.LoadWithOptions(fsys, "main.xsd", opts); err != nil {
		t.Fatalf("LoadWithOptions() error = %v", err)
	}
}
