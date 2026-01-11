package validator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/parser"
)

// TestUnionTypeValidation tests union type validation issues from Category 8
func TestUnionTypeValidation(t *testing.T) {
	testSuiteDir := "../../testdata/xsdtests"

	if _, err := os.Stat(testSuiteDir); os.IsNotExist(err) {
		t.Skip("W3C test suite not found at", testSuiteDir)
	}

	// test case: stZ071 - union with memberTypes and inline types
	t.Run("stZ071", func(t *testing.T) {
		schemaPath := filepath.Join(testSuiteDir, "msData/simpleType/test298668_a.xsd")
		instancePath := filepath.Join(testSuiteDir, "msData/simpleType/test298668.xml")

		schemaDir := filepath.Dir(schemaPath)
		schemaFile := filepath.Base(schemaPath)

		l := loader.NewLoader(loader.Config{
			FS: os.DirFS(schemaDir),
		})

		schema, err := l.Load(schemaFile)
		if err != nil {
			t.Fatalf("Failed to load schema: %v", err)
		}

		instanceData, err := os.ReadFile(instancePath)
		if err != nil {
			t.Fatalf("Failed to read instance: %v", err)
		}

		v := New(mustCompile(t, schema))
		violations, err := v.ValidateStream(bytes.NewReader(instanceData))
		if err != nil {
			t.Fatalf("ValidateStream() error: %v", err)
		}

		if len(violations) > 0 {
			t.Errorf("Expected no violations, got %d:", len(violations))
			for _, v := range violations {
				t.Errorf("  [%s] %s at %s", v.Code, v.Message, v.Path)
			}
		}
	})

	// test case: stZ072 - list with enumeration, itemType is union
	t.Run("stZ072", func(t *testing.T) {
		schemaPath := filepath.Join(testSuiteDir, "msData/simpleType/stZ072.xsd")
		instancePath := filepath.Join(testSuiteDir, "msData/simpleType/stZ072.xml")

		schemaDir := filepath.Dir(schemaPath)
		schemaFile := filepath.Base(schemaPath)

		l := loader.NewLoader(loader.Config{
			FS: os.DirFS(schemaDir),
		})

		schema, err := l.Load(schemaFile)
		if err != nil {
			t.Fatalf("Failed to load schema: %v", err)
		}

		instanceData, err := os.ReadFile(instancePath)
		if err != nil {
			t.Fatalf("Failed to read instance: %v", err)
		}

		v := New(mustCompile(t, schema))
		violations, err := v.ValidateStream(bytes.NewReader(instanceData))
		if err != nil {
			t.Fatalf("ValidateStream() error: %v", err)
		}

		if len(violations) > 0 {
			t.Errorf("Expected no violations, got %d:", len(violations))
			for _, v := range violations {
				t.Errorf("  [%s] %s at %s", v.Code, v.Message, v.Path)
			}
		}
	})

	// test case: stZ074 - union of list types
	t.Run("stZ074", func(t *testing.T) {
		schemaPath := filepath.Join(testSuiteDir, "msData/simpleType/stZ074_a.xsd")
		instancePath := filepath.Join(testSuiteDir, "msData/simpleType/stZ074.xml")

		schemaDir := filepath.Dir(schemaPath)
		schemaFile := filepath.Base(schemaPath)

		l := loader.NewLoader(loader.Config{
			FS: os.DirFS(schemaDir),
		})

		schema, err := l.Load(schemaFile)
		if err != nil {
			t.Fatalf("Failed to load schema: %v", err)
		}

		instanceData, err := os.ReadFile(instancePath)
		if err != nil {
			t.Fatalf("Failed to read instance: %v", err)
		}

		v := New(mustCompile(t, schema))
		violations, err := v.ValidateStream(bytes.NewReader(instanceData))
		if err != nil {
			t.Fatalf("ValidateStream() error: %v", err)
		}

		if len(violations) > 0 {
			t.Errorf("Expected no violations, got %d:", len(violations))
			for _, v := range violations {
				t.Errorf("  [%s] %s at %s", v.Code, v.Message, v.Path)
			}
		}
	})
}

// TestUnionTypeWithFacets tests union types with pattern and enumeration facets
func TestUnionTypeWithFacets(t *testing.T) {
	// test that union-level facets are applied before member type validation
	// this is a unit test for the validateUnionType function
	t.Run("union_with_enumeration_facet", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           xmlns:tns="http://example.com/test"
           elementFormDefault="qualified">
  <xs:simpleType name="BaseUnion">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="RestrictedUnion">
    <xs:restriction base="tns:BaseUnion">
      <xs:enumeration value="foo"/>
      <xs:enumeration value="42"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:RestrictedUnion"/>
</xs:schema>`

		schema, err := parser.Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("Parse schema: %v", err)
		}

		v := New(mustCompile(t, schema))
		okDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">foo</root>`
		if violations := validateStream(t, v, okDoc); len(violations) > 0 {
			t.Errorf("Expected no violations, got %d:", len(violations))
			for _, v := range violations {
				t.Errorf("  [%s] %s at %s", v.Code, v.Message, v.Path)
			}
		}

		badDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">43</root>`
		if violations := validateStream(t, v, badDoc); len(violations) == 0 {
			t.Errorf("Expected enumeration violation, got none")
		}
	})

	t.Run("union_with_pattern_facet", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           xmlns:tns="http://example.com/test"
           elementFormDefault="qualified">
  <xs:simpleType name="BaseUnion">
    <xs:union memberTypes="xs:string xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="RestrictedUnion">
    <xs:restriction base="tns:BaseUnion">
      <xs:pattern value="A[0-9]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:RestrictedUnion"/>
</xs:schema>`

		schema, err := parser.Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("Parse schema: %v", err)
		}

		v := New(mustCompile(t, schema))
		okDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">A12</root>`
		if violations := validateStream(t, v, okDoc); len(violations) > 0 {
			t.Errorf("Expected no violations, got %d:", len(violations))
			for _, v := range violations {
				t.Errorf("  [%s] %s at %s", v.Code, v.Message, v.Path)
			}
		}

		badDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">B12</root>`
		if violations := validateStream(t, v, badDoc); len(violations) == 0 {
			t.Errorf("Expected pattern violation, got none")
		}
	})
}
