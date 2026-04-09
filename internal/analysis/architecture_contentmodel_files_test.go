package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentModelFilesLiveInContentModelPackage(t *testing.T) {
	root := repoRoot(t)
	contentModelDir := filepath.Join(root, "internal", "contentmodel")
	semanticsDir := filepath.Join(root, "internal", "semantics")

	required := []string{
		"doc.go",
		"bitset.go",
		"tree.go",
		"followpos.go",
		"glushkov.go",
		"determinize.go",
		"determinism.go",
		"substitution.go",
		"group_refs.go",
	}
	for _, name := range required {
		path := filepath.Join(contentModelDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing contentmodel file %s: %v", path, err)
		}
	}

	entries, err := os.ReadDir(semanticsDir)
	if err != nil {
		t.Fatalf("read semantics dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "contentmodel_") || name == "group_refs.go" {
			t.Fatalf("content-model file still present under semantics: %s", filepath.Join(semanticsDir, name))
		}
	}
}
