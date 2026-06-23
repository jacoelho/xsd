package tests_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootTestsUsePublicPackage(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(root, name)
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if parsed.Name.Name != "xsd_test" {
			t.Fatalf("%s package = %s, want xsd_test", path, parsed.Name.Name)
		}
		for _, imp := range parsed.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(importPath, "/internal/") {
				t.Fatalf("%s imports internal package %s", path, importPath)
			}
		}
	}
}
