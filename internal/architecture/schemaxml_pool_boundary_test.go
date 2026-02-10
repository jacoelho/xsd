package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestSchemaXMLNoPackageLevelSyncPool(t *testing.T) {
	t.Parallel()

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		if !withinScope(file.relPath, "internal/schemaxml") {
			return
		}

		for _, decl := range parsed.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if isSyncPoolType(valueSpec.Type) {
					t.Fatalf("schemaxml boundary violation: package-level sync.Pool variable in %s", file.relPath)
				}
				for _, value := range valueSpec.Values {
					composite, ok := value.(*ast.CompositeLit)
					if !ok {
						continue
					}
					if isSyncPoolType(composite.Type) {
						t.Fatalf("schemaxml boundary violation: package-level sync.Pool composite in %s", file.relPath)
					}
				}
			}
		}
	})
}

func isSyncPoolType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "sync" && selector.Sel.Name == "Pool"
}
