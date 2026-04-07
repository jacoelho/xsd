package analysis_test

import (
	"testing"
)

func TestPublicAPIAllowlist(t *testing.T) {
	got := collectRootExports(t)
	want := map[string]struct{}{
		"type Schema":                                              {},
		"type Validator":                                           {},
		"type QName":                                               {},
		"type SourceSet":                                           {},
		"type PreparedSchema":                                      {},
		"type SourceOptions":                                       {},
		"type BuildOptions":                                        {},
		"type ValidateOptions":                                     {},
		"func Compile":                                             {},
		"func CompileFile":                                         {},
		"func NewSourceSet":                                        {},
		"func NewSourceOptions":                                    {},
		"func NewBuildOptions":                                     {},
		"func NewValidateOptions":                                  {},
		"method Schema.NewValidator":                               {},
		"method Schema.Validate":                                   {},
		"method Schema.ValidateFSFile":                             {},
		"method Schema.ValidateFile":                               {},
		"method Validator.Validate":                                {},
		"method Validator.ValidateFSFile":                          {},
		"method Validator.ValidateFile":                            {},
		"method SourceSet.WithSourceOptions":                       {},
		"method SourceSet.AddFS":                                   {},
		"method SourceSet.Prepare":                                 {},
		"method SourceSet.Build":                                   {},
		"method PreparedSchema.Build":                              {},
		"method SourceOptions.Validate":                            {},
		"method SourceOptions.WithAllowMissingImportLocations":     {},
		"method SourceOptions.WithSchemaMaxDepth":                  {},
		"method SourceOptions.WithSchemaMaxAttrs":                  {},
		"method SourceOptions.WithSchemaMaxTokenSize":              {},
		"method SourceOptions.WithSchemaMaxQNameInternEntries":     {},
		"method BuildOptions.Validate":                             {},
		"method BuildOptions.WithMaxDFAStates":                     {},
		"method BuildOptions.WithMaxOccursLimit":                   {},
		"method ValidateOptions.Validate":                          {},
		"method ValidateOptions.WithInstanceMaxDepth":              {},
		"method ValidateOptions.WithInstanceMaxAttrs":              {},
		"method ValidateOptions.WithInstanceMaxTokenSize":          {},
		"method ValidateOptions.WithInstanceMaxQNameInternEntries": {},
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
