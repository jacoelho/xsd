package tests_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestPublicationAuditTranslatesCalleeFailuresThroughAuditError parses the
// publication audit and fails when it builds an internal invariant around a
// callee's failure itself: such a failure goes through auditError, which alone
// decides that a schema.limit diagnostic passes and anything else is an
// internal invariant.
func TestPublicationAuditTranslatesCalleeFailuresThroughAuditError(t *testing.T) {
	path := filepath.Join(repoRoot(t), "internal", "schema", "schema_validate.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isSelector(call.Fun, "xsderrors", "InternalInvariant") {
			return true
		}
		calls++
		if len(call.Args) != 2 || !isIdent(call.Args[1], "nil") {
			t.Errorf("%s: InternalInvariant takes a cause in the publication audit; use auditError", fset.Position(call.Pos()))
		}
		return true
	})
	if calls == 0 {
		t.Fatal("found no InternalInvariant call in schema_validate.go; the check no longer inspects the audit")
	}
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name && isIdent(sel.X, pkg)
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
