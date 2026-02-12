package architecture_test

import (
	"testing"
)

func TestPublicAPIAllowlist(t *testing.T) {
	got := collectRootExports(t)
	want := map[string]struct{}{
		"type Schema":                                             {},
		"type QName":                                              {},
		"type SchemaSet":                                          {},
		"type LoadOptions":                                        {},
		"type RuntimeOptions":                                     {},
		"func LoadWithOptions":                                    {},
		"func LoadFile":                                           {},
		"func NewSchemaSet":                                       {},
		"func NewLoadOptions":                                     {},
		"func NewRuntimeOptions":                                  {},
		"method Schema.Validate":                                  {},
		"method Schema.ValidateFSFile":                            {},
		"method Schema.ValidateFile":                              {},
		"method SchemaSet.WithLoadOptions":                        {},
		"method SchemaSet.AddFS":                                  {},
		"method SchemaSet.Compile":                                {},
		"method SchemaSet.CompileWithRuntimeOptions":              {},
		"method LoadOptions.RuntimeOptions":                       {},
		"method LoadOptions.Validate":                             {},
		"method LoadOptions.WithAllowMissingImportLocations":      {},
		"method LoadOptions.WithSchemaMaxDepth":                   {},
		"method LoadOptions.WithSchemaMaxAttrs":                   {},
		"method LoadOptions.WithSchemaMaxTokenSize":               {},
		"method LoadOptions.WithSchemaMaxQNameInternEntries":      {},
		"method LoadOptions.WithRuntimeOptions":                   {},
		"method RuntimeOptions.Validate":                          {},
		"method RuntimeOptions.WithMaxDFAStates":                  {},
		"method RuntimeOptions.WithMaxOccursLimit":                {},
		"method RuntimeOptions.WithInstanceMaxDepth":              {},
		"method RuntimeOptions.WithInstanceMaxAttrs":              {},
		"method RuntimeOptions.WithInstanceMaxTokenSize":          {},
		"method RuntimeOptions.WithInstanceMaxQNameInternEntries": {},
	}

	for item := range want {
		if _, ok := got[item]; !ok {
			t.Errorf("missing public export: %s", item)
		}
	}
	for item := range got {
		if _, ok := want[item]; !ok {
			t.Errorf("unexpected public export: %s", item)
		}
	}
}
