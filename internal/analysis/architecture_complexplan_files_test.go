package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplexPlanFilesLiveInComplexPlanPackage(t *testing.T) {
	root := repoRoot(t)
	complexPlanDir := filepath.Join(root, "internal", "complexplan")
	semanticsDir := filepath.Join(root, "internal", "semantics")

	required := []string{
		"doc.go",
		"plan.go",
		"types.go",
	}
	for _, name := range required {
		path := filepath.Join(complexPlanDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing complexplan file %s: %v", path, err)
		}
	}

	entries, err := os.ReadDir(semanticsDir)
	if err != nil {
		t.Fatalf("read semantics dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "complex_type_plan.go" || strings.HasPrefix(name, "complex_type_plan") {
			t.Fatalf("complex-plan file still present under semantics: %s", filepath.Join(semanticsDir, name))
		}
	}
}
