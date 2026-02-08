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

const (
	pipelineImportPath = "github.com/jacoelho/xsd/internal/pipeline"
	sourceImportPath   = "github.com/jacoelho/xsd/internal/source"
)

func TestPipelineImportsScopedToXSD(t *testing.T) {
	t.Parallel()
	assertImportScoped(t, pipelineImportPath, []string{"xsd.go", "internal/pipeline"})
}

func TestSourceImportsScopedToXSD(t *testing.T) {
	t.Parallel()
	assertImportScoped(t, sourceImportPath, []string{"xsd.go", "internal/source"})
}

func assertImportScoped(t *testing.T, importPath string, allowedScopes []string) {
	t.Helper()

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
		if withinAnyScope(path, allowedScopes) {
			continue
		}
		for _, imp := range parsed.Imports {
			if strings.Trim(imp.Path.Value, "\"") == importPath {
				t.Fatalf("orchestration boundary violation: %s imports %s", path, importPath)
			}
		}
	}
}

func withinAnyScope(path string, scopes []string) bool {
	for _, scope := range scopes {
		if withinScope(path, scope) {
			return true
		}
	}
	return false
}
