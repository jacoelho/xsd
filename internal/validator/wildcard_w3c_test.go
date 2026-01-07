package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd/internal/loader"
	xsdschema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/xml"
)

// composeSchemasForTest composes multiple schema documents into a single schema
// This mimics what the W3C test runner does
func composeSchemasForTest(testDataDir string, schemaFiles []string) (*xsdschema.Schema, error) {
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

// TestWildO016 tests W3C test case wildO016
// Schema has anyAttribute with namespace="##any" (strict mode by default)
// Instance has attribute from namespace "http://foo" which is declared in merged schema
// Expected: valid
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

	doc, err := xml.Parse(instanceFile)
	if err != nil {
		t.Fatalf("Failed to parse instance: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := v.Validate(doc)

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

// TestWildO018 tests W3C test case wildO018
// Schema has anyAttribute with namespace="##other" (strict mode by default)
// Instance has attribute from namespace "http://foo" which is declared in merged schema
// Expected: valid
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

	doc, err := xml.Parse(instanceFile)
	if err != nil {
		t.Fatalf("Failed to parse instance: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := v.Validate(doc)

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

// TestWildO037 tests W3C test case wildO037
// Schema has anyAttribute with namespace="##local http://www.w3.org/1999/xhtml" (namespace list, strict mode by default)
// Instance has attribute from namespace "http://www.w3.org/1999/xhtml" which is declared in merged schema
// Expected: valid
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

	doc, err := xml.Parse(instanceFile)
	if err != nil {
		t.Fatalf("Failed to parse instance: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := v.Validate(doc)

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}