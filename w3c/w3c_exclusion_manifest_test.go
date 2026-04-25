package w3c

import (
	"strings"
	"testing"
)

const expectedW3CExclusionCount = 286

type exclusionCategory string

const (
	exclusionXML11                exclusionCategory = "xml_1_1"
	exclusionXSD11                exclusionCategory = "xsd_1_1"
	exclusionUnsupportedImport    exclusionCategory = "unsupported_import"
	exclusionUnsupportedRegex     exclusionCategory = "unsupported_regex"
	exclusionUnsupportedRedefine  exclusionCategory = "unsupported_redefine"
	exclusionImplementationPolicy exclusionCategory = "implementation_policy"
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
	categories := make(map[exclusionCategory]int)
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
		categories[entry.Category()]++
	}

	required := []exclusionCategory{
		exclusionXML11,
		exclusionXSD11,
		exclusionUnsupportedImport,
		exclusionUnsupportedRegex,
		exclusionUnsupportedRedefine,
		exclusionImplementationPolicy,
	}
	for _, category := range required {
		if categories[category] == 0 {
			t.Fatalf("W3C exclusion category %s has no entries", category)
		}
	}
}

func (e ExclusionReason) Category() exclusionCategory {
	reason := strings.ToLower(e.Reason)
	switch {
	case strings.Contains(reason, "xml 1.1"):
		return exclusionXML11
	case strings.Contains(reason, "xsd 1.1"):
		return exclusionXSD11
	case strings.Contains(reason, "http schema imports"):
		return exclusionUnsupportedImport
	case strings.Contains(reason, "redefine"):
		return exclusionUnsupportedRedefine
	case strings.Contains(reason, "regexp"),
		strings.Contains(reason, "\\p{"),
		strings.Contains(reason, "\\i"),
		strings.Contains(reason, "\\c"),
		strings.Contains(reason, "character class subtraction"),
		strings.Contains(reason, "unicode property"):
		return exclusionUnsupportedRegex
	default:
		return exclusionImplementationPolicy
	}
}
