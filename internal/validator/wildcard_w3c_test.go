package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/parser"
)

// composeSchemasForTest composes multiple schema documents into a single schema
// This mimics what the W3C test runner does
func composeSchemasForTest(testDataDir string, schemaFiles []string) (*parser.Schema, error) {
	if len(schemaFiles) == 0 {
		return nil, nil
	}

	basePath := schemaFiles[0]
	baseDir := filepath.Dir(testDataDir + "/" + basePath)
	baseFile := filepath.Base(basePath)

	cfg := loader.Config{
		FS: os.DirFS(baseDir),
	}
	l := loader.NewLoader(cfg)
	composedSchema, err := l.Load(baseFile)
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(schemaFiles); i++ {
		docPath := schemaFiles[i]
		docDir := filepath.Dir(testDataDir + "/" + docPath)
		docFile := filepath.Base(docPath)

		docLoader := loader.NewLoader(loader.Config{
			FS: os.DirFS(docDir),
		})
		additionalSchema, err := docLoader.Load(docFile)
		if err != nil {
			return nil, err
		}

		// merge additional schema into composed schema (preserve original namespaces)
		for qname, decl := range additionalSchema.AttributeDecls {
			if _, exists := composedSchema.AttributeDecls[qname]; exists {
				continue
			}
			declCopy := *decl
			declCopy.SourceNamespace = additionalSchema.TargetNamespace
			composedSchema.AttributeDecls[qname] = &declCopy
		}
	}

	return composedSchema, nil
}

// TestWildO016 covers W3C wildO016 for anyAttribute with ##any.
// It expects validation to succeed with merged schema declarations.
func TestWildO016(t *testing.T) {
	testDataDir := "../../testdata/xsdtests"

	// compose schemas like the W3C test runner does
	schema, err := composeSchemasForTest(testDataDir, []string{
		"msData/wildcards/wildO016.xsd",
		"msData/wildcards/wildO016a.xsd",
	})
	if err != nil {
		t.Fatalf("Failed to compose schemas: %v", err)
	}

	instancePath := "msData/wildcards/wildO016.xml"
	instanceFile, err := os.Open(testDataDir + "/" + instancePath)
	if err != nil {
		t.Fatalf("Failed to open instance: %v", err)
	}
	defer func() {
		if err := instanceFile.Close(); err != nil {
			t.Fatalf("Failed to close instance: %v", err)
		}
	}()

	v := New(mustCompile(t, schema))
	violations, err := v.ValidateStream(instanceFile)
	if err != nil {
		t.Fatalf("ValidateStream() error: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

// TestWildO018 covers W3C wildO018 for anyAttribute with ##other.
// It expects validation to succeed with merged schema declarations.
func TestWildO018(t *testing.T) {
	testDataDir := "../../testdata/xsdtests"

	schema, err := composeSchemasForTest(testDataDir, []string{
		"msData/wildcards/wildO018.xsd",
		"msData/wildcards/wildO018a.xsd",
	})
	if err != nil {
		t.Fatalf("Failed to compose schemas: %v", err)
	}

	instanceFile, err := os.Open(testDataDir + "/msData/wildcards/wildO018.xml")
	if err != nil {
		t.Fatalf("Failed to open instance: %v", err)
	}
	defer func() {
		if err := instanceFile.Close(); err != nil {
			t.Fatalf("Failed to close instance: %v", err)
		}
	}()

	v := New(mustCompile(t, schema))
	violations, err := v.ValidateStream(instanceFile)
	if err != nil {
		t.Fatalf("ValidateStream() error: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

// TestWildO037 covers W3C wildO037 for anyAttribute with a namespace list.
// It expects validation to succeed with merged schema declarations.
func TestWildO037(t *testing.T) {
	testDataDir := "../../testdata/xsdtests"

	schema, err := composeSchemasForTest(testDataDir, []string{
		"msData/wildcards/wildO037.xsd",
		"msData/wildcards/wildO037a.xsd",
	})
	if err != nil {
		t.Fatalf("Failed to compose schemas: %v", err)
	}

	instanceFile, err := os.Open(testDataDir + "/msData/wildcards/wildO037.xml")
	if err != nil {
		t.Fatalf("Failed to open instance: %v", err)
	}
	defer func() {
		if err := instanceFile.Close(); err != nil {
			t.Fatalf("Failed to close instance: %v", err)
		}
	}()

	v := New(mustCompile(t, schema))
	violations, err := v.ValidateStream(instanceFile)
	if err != nil {
		t.Fatalf("ValidateStream() error: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}
