package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitectureDocsDoNotUseRetiredTerms(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		"docs/architecture.md",
		"internal/compiler/doc.go",
		"internal/validator/doc.go",
		"internal/semantics/doc.go",
	}
	retired := []string{
		"NewSchemaSet",
		"LoadWithOptions",
		"LoadFile",
		"CompileWithRuntimeOptions",
		"SchemaSet.Compile",
		"internal/preprocessor",
		"internal/semanticresolve",
		"internal/semanticcheck",
		"internal/architecture/",
		"schemaset_",
	}

	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(data)
		for _, term := range retired {
			if strings.Contains(text, term) {
				t.Fatalf("%s still contains retired term %q", rel, term)
			}
		}
	}
}
