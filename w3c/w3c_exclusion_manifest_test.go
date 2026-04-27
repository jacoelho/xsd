package w3c

import (
	"strings"
	"testing"
)

const expectedW3CExclusionCount = 286

type ExclusionCategory string

const (
	ExclusionCategoryXML11                ExclusionCategory = "xml_1_1"
	ExclusionCategoryXSD11                ExclusionCategory = "xsd_1_1"
	ExclusionCategoryUnsupportedImport    ExclusionCategory = "unsupported_import"
	ExclusionCategoryUnsupportedRegex     ExclusionCategory = "unsupported_regex"
	ExclusionCategoryUnsupportedRedefine  ExclusionCategory = "unsupported_redefine"
	ExclusionCategoryImplementationPolicy ExclusionCategory = "implementation_policy"
)

type exclusionManifest struct {
	ExpectedCount int
	Entries       []ExclusionReason
}

var w3cExclusionManifest = exclusionManifest{
	ExpectedCount: expectedW3CExclusionCount,
	Entries:       excludePatterns,
}

func TestW3CExclusionManifestIsAuditable(t *testing.T) {
	manifest := w3cExclusionManifest
	if len(manifest.Entries) != manifest.ExpectedCount {
		t.Fatalf("W3C exclusion count = %d, want %d", len(manifest.Entries), manifest.ExpectedCount)
	}

	seen := make(map[string]struct{}, len(manifest.Entries))
	categories := make(map[ExclusionCategory]int)
	for _, entry := range manifest.Entries {
		if strings.TrimSpace(entry.Pattern) == "" {
			t.Fatal("W3C exclusion has empty pattern")
		}
		if strings.TrimSpace(entry.Reason) == "" {
			t.Fatalf("W3C exclusion %q has empty reason", entry.Pattern)
		}
		pattern := strings.ToLower(entry.Pattern)
		if _, ok := seen[pattern]; ok {
			t.Fatalf("duplicate W3C exclusion pattern %q", entry.Pattern)
		}
		seen[pattern] = struct{}{}
		if entry.Category == "" {
			t.Fatalf("W3C exclusion %q has empty category", entry.Pattern)
		}
		categories[entry.Category]++
	}

	required := []ExclusionCategory{
		ExclusionCategoryXML11,
		ExclusionCategoryXSD11,
		ExclusionCategoryUnsupportedImport,
		ExclusionCategoryUnsupportedRegex,
		ExclusionCategoryUnsupportedRedefine,
		ExclusionCategoryImplementationPolicy,
	}
	for _, category := range required {
		if categories[category] == 0 {
			t.Fatalf("W3C exclusion category %s has no entries", category)
		}
	}
}
