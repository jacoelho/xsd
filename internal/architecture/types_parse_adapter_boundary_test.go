package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypesParseAdaptersOnlyUsedByTypes(t *testing.T) {
	t.Parallel()

	const typesImport = "github.com/jacoelho/xsd/internal/types"

	allowedScopes := []string{
		"internal/types",
	}

	root := repoRoot(t)
	fset := token.NewFileSet()

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
		allowedScope := false
		for _, scope := range allowedScopes {
			if withinScope(relPath, scope) {
				allowedScope = true
				break
			}
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		typesAliases := make(map[string]struct{})
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if importPath != typesImport {
				continue
			}
			alias := "types"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			typesAliases[alias] = struct{}{}
		}
		if len(typesAliases) == 0 {
			return nil
		}

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := typesAliases[pkgIdent.Name]; !ok {
				return true
			}
			if !strings.HasPrefix(selector.Sel.Name, "Parse") {
				return true
			}
			if allowedScope {
				return true
			}
			pos := fset.Position(selector.Sel.Pos())
			t.Fatalf("types parse adapter boundary violation: %s uses %s.%s at %s", relPath, pkgIdent.Name, selector.Sel.Name, pos)
			return false
		})

		return nil
	})
	if err != nil {
		t.Fatalf("scan repository files: %v", err)
	}
}
