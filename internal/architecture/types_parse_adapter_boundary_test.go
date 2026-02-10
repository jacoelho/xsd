package architecture_test

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

func TestModelParseAdaptersOnlyUsedByModel(t *testing.T) {
	t.Parallel()

	const modelImport = "github.com/jacoelho/xsd/internal/model"

	allowedScopes := []string{
		"internal/model",
	}

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		allowedScope := withinAnyScope(file.relPath, allowedScopes)
		modelAliases := importAliasesForPath(parsed, modelImport, "model")
		if len(modelAliases) == 0 {
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
			if !modelAliases[pkgIdent.Name] {
				return true
			}
			if !strings.HasPrefix(selector.Sel.Name, "Parse") {
				return true
			}
			if allowedScope {
				return true
			}
			t.Fatalf("model parse adapter boundary violation: %s uses %s.%s", file.relPath, pkgIdent.Name, selector.Sel.Name)
			return false
		})
	})
}
