package compiler_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/compiler"
)

func TestPrepareRootsRejectsMissingRoots(t *testing.T) {
	t.Parallel()

	_, err := compiler.PrepareRoots(compiler.LoadConfig{})
	if err == nil {
		t.Fatal("PrepareRoots() error = nil, want missing roots error")
	}
}

func TestPrepareRootsBuildsRuntime(t *testing.T) {
	t.Parallel()

	prepared, err := compiler.PrepareRoots(compiler.LoadConfig{
		Roots: []compiler.Root{{
			FS: fstest.MapFS{
				"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
			},
			Location: "schema.xsd",
		}},
	})
	if err != nil {
		t.Fatalf("PrepareRoots() error = %v", err)
	}

	rt, err := prepared.Build(compiler.BuildConfig{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if rt == nil {
		t.Fatal("Build() returned nil runtime schema")
	}
}

func TestPrepareRootsAllowsMissingImportLocation(t *testing.T) {
	t.Parallel()

	_, err := compiler.PrepareRoots(compiler.LoadConfig{
		Roots: []compiler.Root{{
			FS: fstest.MapFS{
				"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
			},
			Location: "schema.xsd",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "import missing schemaLocation") {
		t.Fatalf("PrepareRoots() error = %v, want missing import location", err)
	}

	prepared, err := compiler.PrepareRoots(compiler.LoadConfig{
		Roots: []compiler.Root{{
			FS: fstest.MapFS{
				"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
			},
			Location: "schema.xsd",
		}},
		AllowMissingImportLocations: true,
	})
	if err != nil {
		t.Fatalf("PrepareRoots() with AllowMissingImportLocations error = %v", err)
	}
	if prepared == nil {
		t.Fatal("PrepareRoots() with AllowMissingImportLocations returned nil prepared schema")
	}
}
