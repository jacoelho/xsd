package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type repoGoFile struct {
	absPath string
	relPath string
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("resolve repo root %s: %v", root, err)
	}
	return root
}

func withinScope(path, scope string) bool {
	if strings.HasSuffix(scope, ".go") {
		return filepath.Clean(path) == filepath.Clean(scope)
	}
	cleanScope := filepath.Clean(scope) + string(filepath.Separator)
	return strings.HasPrefix(filepath.Clean(path), cleanScope)
}

func withinAnyScope(path string, scopes []string) bool {
	for _, scope := range scopes {
		if withinScope(path, scope) {
			return true
		}
	}
	return false
}

func repoProductionGoFiles(t *testing.T) []repoGoFile {
	t.Helper()

	root := repoRoot(t)
	files := make([]repoGoFile, 0, 128)
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
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, repoGoFile{
			absPath: path,
			relPath: relPath,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk go files: %v", err)
	}

	slices.SortFunc(files, func(left, right repoGoFile) int {
		return strings.Compare(left.relPath, right.relPath)
	})
	return files
}

func forEachParsedRepoProductionGoFile(t *testing.T, mode parser.Mode, visit func(file repoGoFile, parsed *ast.File)) {
	t.Helper()

	files := repoProductionGoFiles(t)
	fset := token.NewFileSet()
	for _, file := range files {
		parsed, err := parser.ParseFile(fset, file.absPath, nil, mode)
		if err != nil {
			t.Fatalf("parse file %s: %v", file.relPath, err)
		}
		visit(file, parsed)
	}
}

func importAliasesForPath(parsed *ast.File, importPath, defaultAlias string) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range parsed.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		if path != importPath {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				continue
			}
			aliases[imp.Name.Name] = true
			continue
		}
		aliases[defaultAlias] = true
	}
	return aliases
}
