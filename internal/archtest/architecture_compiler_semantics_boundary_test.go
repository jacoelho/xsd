package archtest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerDoesNotImportSemanticCheck(t *testing.T) {
	root := repoRoot(t)
	compilerDir := filepath.Join(root, "internal", "compiler")

	entries, err := os.ReadDir(compilerDir)
	if err != nil {
		t.Fatalf("read %s: %v", compilerDir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(compilerDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		banned := `"github.com/jacoelho/xsd/internal/schemair/` + `check"`
		hasSemanticsImport := bytes.Contains(src, []byte(banned))
		if hasSemanticsImport {
			t.Fatalf("%s must not import retired semantic package", path)
		}
	}
}
