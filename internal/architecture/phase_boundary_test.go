package architecture_test

import (
	"go/ast"
	"testing"
)

const schemaanalysisImportPath = "github.com/jacoelho/xsd/internal/schemaanalysis"

var semanticPhaseFunctions = map[string]struct{}{
	"AssignIDs":         {},
	"ResolveReferences": {},
	"DetectCycles":      {},
	"ValidateUPA":       {},
}

func TestSchemaanalysisPhaseFunctionsOnlyInPipeline(t *testing.T) {
	t.Parallel()

	forEachParsedRepoProductionGoFile(t, 0, func(file repoGoFile, parsed *ast.File) {
		if withinScope(file.relPath, "internal/pipeline") {
			return
		}
		aliases := importAliasesForPath(parsed, schemaanalysisImportPath, "schemaanalysis")
		if len(aliases) == 0 {
			return
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
				t.Fatalf("phase boundary violation: %s calls %s.%s", file.relPath, pkg.Name, selector.Sel.Name)
			}
			return true
		})
	})
}
