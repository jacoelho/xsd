package architecture_test

import (
	"go/ast"
	"testing"
)

func TestSchemaflowClonePathNotUsedInProduction(t *testing.T) {
	t.Parallel()

	const schemaflowImportPath = "github.com/jacoelho/xsd/internal/schemaflow"

	forEachParsedRepoProductionGoFile(t, 0, func(file repoGoFile, parsed *ast.File) {
		if withinScope(file.relPath, "internal/schemaflow") {
			return
		}

		aliases := importAliasesForPath(parsed, schemaflowImportPath, "schemaflow")
		if len(aliases) == 0 {
			return
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "ResolveAndValidate" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || !aliases[pkg.Name] {
				return true
			}
			t.Fatalf("schemaflow boundary violation: %s calls %s.ResolveAndValidate", file.relPath, pkg.Name)
			return false
		})
	})
}
