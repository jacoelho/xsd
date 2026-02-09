package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestSchemaExportedValidateMethodsUseGateway(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	path := filepath.Join(root, "xsd.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse xsd.go: %v", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil {
			continue
		}
		if fn.Name == nil || (fn.Name.Name != "Validate" && fn.Name.Name != "ValidateFile") {
			continue
		}
		if !isSchemaMethod(fn) {
			continue
		}

		recvName := receiverName(fn)
		usesGateway := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil {
				return true
			}
			if sel.Sel.Name == "validateReader" && selectorBaseIdent(sel.X, recvName) {
				usesGateway = true
				return true
			}
			if !isEngineMethodCall(sel, recvName) {
				return true
			}
			t.Fatalf("gateway boundary violation: Schema.%s calls s.engine directly", fn.Name.Name)
			return false
		})
		if !usesGateway {
			t.Fatalf("gateway boundary violation: Schema.%s must call validateReader", fn.Name.Name)
		}
	}
}

func isSchemaMethod(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}
	typ := fn.Recv.List[0].Type
	star, ok := typ.(*ast.StarExpr)
	if !ok {
		return false
	}
	ident, ok := star.X.(*ast.Ident)
	return ok && ident.Name == "Schema"
}

func receiverName(fn *ast.FuncDecl) string {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) == 0 || len(fn.Recv.List[0].Names) == 0 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
}

func selectorBaseIdent(expr ast.Expr, name string) bool {
	if name == "" {
		return false
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func isEngineMethodCall(sel *ast.SelectorExpr, recvName string) bool {
	if sel == nil {
		return false
	}
	inner, ok := sel.X.(*ast.SelectorExpr)
	if !ok || inner.Sel == nil || inner.Sel.Name != "engine" {
		return false
	}
	return selectorBaseIdent(inner.X, recvName)
}
