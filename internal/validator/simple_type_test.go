package validator

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd/internal/loader"
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
		// TODO: Create a minimal test case for union with enumeration
		// this will be implemented after fixing the main validation logic
		t.Skip("To be implemented after fixing union validation")
	})

	t.Run("union_with_pattern_facet", func(t *testing.T) {
		// TODO: Create a minimal test case for union with pattern
		// this will be implemented after fixing the main validation logic
		t.Skip("To be implemented after fixing union validation")
	})
}
