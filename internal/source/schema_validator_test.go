package source

import (
	"os"
	"strings"
	"testing"
)

func TestGYearMinInclusive003Schema(t *testing.T) {
	testDataDir := "../../testdata/xsdtests"
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found")
	}

	cfg := Config{
		FS: os.DirFS(testDataDir),
	}
	l := NewLoader(cfg)
	schema, err := loadAndPrepare(t, l, "msData/datatypes/Facets/Schemas/gYear_minInclusive003.xsd")
	if err != nil {
		t.Logf("Schema loading failed as expected: %v", err)
		return
	}

	t.Errorf("Schema loaded successfully but should have failed validation")

	if schema != nil {
		t.Logf("Schema loaded with %d type definitions", len(schema.TypeDefs))
	}
}

func TestInvalidPatternSchemas(t *testing.T) {
	testDataDir := "../../testdata/xsdtests"
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found")
	}

	tests := []struct {
		name       string
		schemaPath string
		shouldFail bool
	}{
		{
			name:       "reM61 - invalid Unicode property",
			schemaPath: "msData/regex/reM61.xsd",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				FS: os.DirFS(testDataDir),
			}
			l := NewLoader(cfg)
			_, err := loadAndPrepare(t, l, tt.schemaPath)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Schema %s should have failed validation but loaded successfully", tt.schemaPath)
				} else {
					if !strings.Contains(err.Error(), "pattern") {
						t.Logf("Schema validation failed as expected, but error doesn't mention 'pattern': %v", err)
					} else {
						t.Logf("Schema validation correctly failed with pattern error: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Schema %s should have loaded successfully but got error: %v", tt.schemaPath, err)
				}
			}
		})
	}
}
