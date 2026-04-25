package archtest_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCorePhasePackagesHaveDoc(t *testing.T) {
	root := repoRoot(t)
	required := []string{
		"internal/schemaast",
		"internal/archtest",
		"internal/compiler",
		"internal/schemair",
		"internal/xsdpath",
		"internal/runtimebuild",
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

func TestRetiredSchemaIRSourceAliasFileIsGone(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "schemair", "source_types.go")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("retired source alias file still exists: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
