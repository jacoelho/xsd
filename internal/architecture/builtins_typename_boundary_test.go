package architecture_test

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

func TestBuiltinsCallsitesUseBuiltinsTypeAliases(t *testing.T) {
	t.Parallel()

	const (
		builtinsImport = "github.com/jacoelho/xsd/internal/builtins"
		modelImport    = "github.com/jacoelho/xsd/internal/model"
	)

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		builtinsAliases := importAliasesForPath(parsed, builtinsImport, "builtins")
		if len(builtinsAliases) == 0 {
			return
		}
		modelAliases := importAliasesForPath(parsed, modelImport, "model")
		if len(modelAliases) == 0 {
			return
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := selector.X.(*ast.Ident)
			if !ok || !builtinsAliases[pkgIdent.Name] {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}

			switch selector.Sel.Name {
			case "Get", "NewSimpleType":
				if usesModelTypeNameExpr(call.Args[0], modelAliases) {
					t.Fatalf("builtins alias boundary violation: %s passes model type-name expression to %s", file.relPath, selector.Sel.Name)
				}
			case "GetNS":
				if usesModelNamespaceExpr(call.Args[0], modelAliases) {
					t.Fatalf("builtins alias boundary violation: %s passes model namespace expression to GetNS", file.relPath)
				}
			}
			return true
		})
	})
}

func usesModelTypeNameExpr(expr ast.Expr, modelAliases map[string]bool) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		pkgIdent, ok := typed.X.(*ast.Ident)
		if !ok || !modelAliases[pkgIdent.Name] {
			return false
		}
		return strings.HasPrefix(typed.Sel.Name, "TypeName")
	case *ast.CallExpr:
		sel, ok := typed.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || !modelAliases[pkgIdent.Name] {
			return false
		}
		return sel.Sel.Name == "TypeName"
	default:
		return false
	}
}

func usesModelNamespaceExpr(expr ast.Expr, modelAliases map[string]bool) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		pkgIdent, ok := typed.X.(*ast.Ident)
		if !ok || !modelAliases[pkgIdent.Name] {
			return false
		}
		return typed.Sel.Name == "XSDNamespace"
	case *ast.CallExpr:
		sel, ok := typed.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || !modelAliases[pkgIdent.Name] {
			return false
		}
		return sel.Sel.Name == "NamespaceURI"
	default:
		return false
	}
}
