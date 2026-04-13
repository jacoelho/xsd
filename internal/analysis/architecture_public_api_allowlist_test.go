package analysis_test

import (
	"testing"
)

func TestPublicAPIAllowlist(t *testing.T) {
	got := collectRootExports(t)
	want := map[string]struct{}{
		"type Schema":                        {},
		"type Validator":                     {},
		"type QName":                         {},
		"type SourceSet":                     {},
		"type PreparedSchema":                {},
		"type CompileOption":                 {},
		"type SourceOption":                  {},
		"type SourceOptionValue":             {},
		"type BuildOption":                   {},
		"type BuildOptionValue":              {},
		"type ValidateOption":                {},
		"type ValidateOptionValue":           {},
		"func Compile":                       {},
		"func CompileFile":                   {},
		"func NewSourceSet":                  {},
		"func AllowMissingImportLocations":   {},
		"func SchemaMaxDepth":                {},
		"func SchemaMaxAttrs":                {},
		"func SchemaMaxTokenSize":            {},
		"func SchemaMaxQNameInternEntries":   {},
		"func MaxDFAStates":                  {},
		"func MaxOccursLimit":                {},
		"func InstanceMaxDepth":              {},
		"func InstanceMaxAttrs":              {},
		"func InstanceMaxTokenSize":          {},
		"func InstanceMaxQNameInternEntries": {},
		"method Schema.NewValidator":         {},
		"method Schema.Validate":             {},
		"method Schema.ValidateFSFile":       {},
		"method Schema.ValidateFile":         {},
		"method Validator.Validate":          {},
		"method Validator.ValidateFSFile":    {},
		"method Validator.ValidateFile":      {},
		"method SourceSet.WithOptions":       {},
		"method SourceSet.AddFS":             {},
		"method SourceSet.Prepare":           {},
		"method SourceSet.Build":             {},
		"method PreparedSchema.Build":        {},
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
