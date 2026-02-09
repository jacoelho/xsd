package architecture_test

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

func TestTypesParseAdaptersOnlyUsedByTypes(t *testing.T) {
	t.Parallel()

	const typesImport = "github.com/jacoelho/xsd/internal/types"

	allowedScopes := []string{
		"internal/types",
	}

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		allowedScope := withinAnyScope(file.relPath, allowedScopes)
		typesAliases := importAliasesForPath(parsed, typesImport, "types")
		if len(typesAliases) == 0 {
			return
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if !typesAliases[pkgIdent.Name] {
				return true
			}
			if !strings.HasPrefix(selector.Sel.Name, "Parse") {
				return true
			}
			if allowedScope {
				return true
			}
			t.Fatalf("types parse adapter boundary violation: %s uses %s.%s", file.relPath, pkgIdent.Name, selector.Sel.Name)
			return false
		})
	})
}
