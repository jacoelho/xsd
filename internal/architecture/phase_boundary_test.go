package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const semanticImportPath = "github.com/jacoelho/xsd/internal/semantic"

var semanticPhaseFunctions = map[string]struct{}{
	"AssignIDs":         {},
	"ResolveReferences": {},
	"DetectCycles":      {},
	"ValidateUPA":       {},
}

func TestSemanticPhaseFunctionsOnlyInPipeline(t *testing.T) {
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
		path, err := filepath.Rel(root, absPath)
		if err != nil {
			t.Fatalf("rel path %s: %v", absPath, err)
		}
		if withinScope(path, "internal/pipeline") {
			continue
		}
		parsed, err := parser.ParseFile(fset, absPath, nil, 0)
		if err != nil {
			t.Fatalf("parse file %s: %v", path, err)
		}

		aliases := semanticAliases(parsed)
		if len(aliases) == 0 {
			continue
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || selector.Sel == nil {
				return true
			}
			if _, watched := semanticPhaseFunctions[selector.Sel.Name]; !watched {
				return true
			}
			if aliases[pkg.Name] {
				t.Fatalf("phase boundary violation: %s calls %s.%s", path, pkg.Name, selector.Sel.Name)
			}
			return true
		})
	}
}

func semanticAliases(parsed *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		if importPath != semanticImportPath {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				continue
			}
			aliases[imp.Name.Name] = true
			continue
		}
		aliases["semantic"] = true
	}
	return aliases
}
