package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const semanticResolveImportPath = "github.com/jacoelho/xsd/internal/semanticresolve"

func TestSemanticResolveImportsScopedToSchemaflow(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk go files: %v", err)
	}
	slices.Sort(files)

	fset := token.NewFileSet()
	for _, absPath := range files {
		parsed, err := parser.ParseFile(fset, absPath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports %s: %v", absPath, err)
		}
		path, err := filepath.Rel(root, absPath)
		if err != nil {
			t.Fatalf("rel path %s: %v", absPath, err)
		}
		if withinScope(path, "internal/schemaflow") || withinScope(path, "internal/semanticresolve") {
			continue
		}
		for _, imp := range parsed.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if importPath == semanticResolveImportPath {
				t.Fatalf("semanticresolve boundary violation: %s imports %s", path, importPath)
			}
		}
	}
}
