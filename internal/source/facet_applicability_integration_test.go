package source

import (
	"os"
	"testing"
)

func TestFacetApplicability_ListTypes_W3CTestCases(t *testing.T) {
	// test the actual W3C test cases mentioned in the issue
	testDataDir := "../../testdata/xsdtests"
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found")
	}

	testCases := []struct {
		name       string
		schemaPath string
		shouldLoad bool
	}{
		{
			name:       "stF001 - length facet on list of integer",
			schemaPath: "msData/simpleType/stF001.xsd",
			shouldLoad: true, // expected: valid schema
		},
		{
			name:       "stG002 - maxLength facet on list of integer",
			schemaPath: "msData/simpleType/stG002.xsd",
			shouldLoad: true, // expected: valid schema
		},
		{
			name:       "stJ004 - length facet on list of integer",
			schemaPath: "msData/simpleType/stJ004.xsd",
			shouldLoad: true, // expected: valid schema
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				FS: os.DirFS(testDataDir),
			}
			l := NewLoader(cfg)
			_, err := loadAndPrepare(t, l, tt.schemaPath)

			if tt.shouldLoad {
				if err != nil {
					t.Errorf("Schema %s should load successfully but got error: %v", tt.schemaPath, err)
				}
			} else {
				if err == nil {
					t.Errorf("Schema %s should fail to load but loaded successfully", tt.schemaPath)
				}
			}
		})
	}
}
