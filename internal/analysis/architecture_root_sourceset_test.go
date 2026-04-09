package analysis_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootSourceSetFilesUseSourceSetPrefix(t *testing.T) {
	root := repoRoot(t)

	required := []string{
		"sourceset.go",
		"sourceset_entry.go",
		"sourceset_prepare.go",
	}
	for _, name := range required {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing SourceSet file %s: %v", path, err)
		}
	}

	legacy := []string{
		"schemaset_types.go",
		"schemaset_prepare.go",
		"sourceset_types.go",
	}
	for _, name := range legacy {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy SourceSet file still present: %s", path)
		}
	}
}
