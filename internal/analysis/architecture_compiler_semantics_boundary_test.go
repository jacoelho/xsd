package analysis_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerInternalFilesAvoidSemanticsImports(t *testing.T) {
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
		if name == "prepare_semantics.go" {
			continue
		}
		path := filepath.Join(compilerDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if bytes.Contains(src, []byte(`"github.com/jacoelho/xsd/internal/semantics"`)) {
			t.Fatalf("%s must not import internal/semantics", path)
		}
	}
}
