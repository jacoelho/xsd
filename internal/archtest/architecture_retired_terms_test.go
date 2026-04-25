package archtest_test

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
		"internal/schemair/doc.go",
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
		"internal/schemair/" + "check",
		"internal/schemair/" + "resolveindex",
		"internal/schemair/" + "upamodel",
		"internal/schemair/" + "xpath",
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

func TestArchitectureCodeDoesNotUseRetiredCompileScaffolding(t *testing.T) {
	root := repoRoot(t)
	terms := []string{
		"XSD_DIRECT_" + "PARITY",
		"XSD_DIRECT_" + "W3C_PARITY",
		"build" + "GraphRuntimeForParity",
		"compile" + "GraphSchemaFromPath",
		"Document" + "ElementDecl",
		"Document" + "AttributeDecl",
		"Document" + "NotationDecl",
		"schemaast." + "TypeDecl",
		"ast." + "TypeDecl",
		"Type" + "Defs",
		"Schema" + "Graph",
		"Base" + "Decl",
		"New" + "SimpleTypeRef",
		"simple" + "Memo",
		"Clone" + "Schema",
		"Copy" + "Type",
	}

	for _, dir := range []string{"internal", "w3c"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(data)
			for _, term := range terms {
				if strings.Contains(text, term) {
					rel, err := filepath.Rel(root, path)
					if err != nil {
						rel = path
					}
					t.Fatalf("%s still contains retired term %q", filepath.ToSlash(rel), term)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}
}
