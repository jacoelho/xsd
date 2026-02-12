package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoPointerToSliceTypes(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", "bin", "vendor", ".codex", ".cursor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			star, ok := n.(*ast.StarExpr)
			if !ok {
				return true
			}
			arrayType, ok := star.X.(*ast.ArrayType)
			if !ok || arrayType.Len != nil {
				return true
			}
			found = true
			return false
		})
		if found {
			t.Errorf("pointer-to-slice type is forbidden: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan for pointer-to-slice types: %v", err)
	}
}
