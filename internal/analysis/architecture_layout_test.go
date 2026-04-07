package analysis_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCorePhasePackagesHaveDoc(t *testing.T) {
	root := repoRoot(t)
	required := []string{
		"internal/parser",
		"internal/semantics",
		"internal/analysis",
		"internal/compiler",
	}

	for _, rel := range required {
		doc := filepath.Join(root, rel, "doc.go")
		if _, err := os.Stat(doc); err != nil {
			if os.IsNotExist(err) {
				t.Errorf("missing package doc: %s", doc)
				continue
			}
			t.Fatalf("stat %s: %v", doc, err)
		}
	}
}
