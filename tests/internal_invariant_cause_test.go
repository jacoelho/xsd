package tests_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestInternalInvariantsPassErrorsAsCauses(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata" || entry.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isInternalInvariantCall(call, file.Name.Name) || len(call.Args) == 0 {
				return true
			}
			if errorText := findErrorTextCall(call.Args[0]); errorText != nil {
				violations = append(violations, fset.Position(errorText.Pos()).String())
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Errorf("%s: InternalInvariant message embeds an error's text; pass the error as the cause", violation)
	}
}

func isInternalInvariantCall(call *ast.CallExpr, pkg string) bool {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		qualifier, ok := fun.X.(*ast.Ident)
		return ok && qualifier.Name == "xsderrors" && fun.Sel.Name == "InternalInvariant"
	case *ast.Ident:
		return pkg == "xsderrors" && fun.Name == "InternalInvariant"
	}
	return false
}

func findErrorTextCall(message ast.Expr) *ast.CallExpr {
	var found *ast.CallExpr
	ast.Inspect(message, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Error" && len(call.Args) == 0 {
			found = call
			return false
		}
		return true
	})
	return found
}
