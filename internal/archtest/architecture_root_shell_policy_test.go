package archtest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootPackagesDoNotDefineTypeAliases(t *testing.T) {
	root := repoRoot(t)
	rootDirs := []string{
		filepath.Join(root, "internal", "compiler"),
		filepath.Join(root, "internal", "validator"),
	}

	fset := token.NewFileSet()
	for _, dir := range rootDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}

			path := filepath.Join(dir, name)
			node, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, decl := range node.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				for _, spec := range gen.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || !typeSpec.Assign.IsValid() {
						continue
					}
					relPath, err := filepath.Rel(root, path)
					if err != nil {
						relPath = path
					}
					t.Errorf("root package type alias %s in %s; depend on the leaf type directly or keep the type local", typeSpec.Name.Name, filepath.ToSlash(relPath))
				}
			}
		}
	}
}
