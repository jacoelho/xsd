package tests_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// auditFiles hold the publication audit. A file with listed functions holds
// only those and the methods of the listed types; the rest of it is shared with
// compilation.
var auditFiles = map[string][]string{
	"schema_validate.go": nil,
	"schema_publish.go":  nil,
	"content_model.go":   {"validateContentModelTable", "contentModelTableAudit"},
	"notation_use.go":    {"validateNotationTypeUse", "validateNotationConstructorUses", "validateNotationConstructorEdge", "sourceConstructorUses"},
	"reachability.go":    {"validatePublishedSimpleTypes"},
}

// auditOutsideFunctions are functions in auditFiles that are not part of the
// audit: they build the published program, and sealSchemaProgram translates
// their failures with auditError.
var auditOutsideFunctions = map[string]bool{
	"publishSchema":            true,
	"newSchemaProgram":         true,
	"newPublishedElementReads": true,
}

// auditTypedCallees return only diagnostics, so the audit may return their
// failures as they are.
var auditTypedCallees = map[string]bool{
	"auditError":                true,
	"spendTypeDerivationWork":   true,
	"spendContentModelWork":     true,
	"spendContentModelWorkN":    true,
	"chargedParticles":          true,
	"withSchemaCompileLocation": true,
}

// auditError alone decides that a schema.limit diagnostic passes and anything
// else is an internal invariant, so the audit must not build an invariant around
// a cause or return a callee's error that auditError has not seen.
func TestPublicationAuditTranslatesCalleeFailuresThroughAuditError(t *testing.T) {
	fset := token.NewFileSet()
	var audit []*ast.FuncDecl
	names := map[string]bool{}
	for name, only := range auditFiles {
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot(t), "internal", "schema", name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !isAuditFunction(only, fn) {
				continue
			}
			audit = append(audit, fn)
			names[fn.Name.Name] = true
		}
	}
	if len(audit) == 0 {
		t.Fatal("found no audit function; the check no longer inspects the audit")
	}
	for _, fn := range audit {
		checkAuditFunction(t, fset, fn, func(callee string) bool {
			return names[callee] || auditTypedCallees[callee]
		})
	}
}

func isAuditFunction(only []string, fn *ast.FuncDecl) bool {
	if auditOutsideFunctions[fn.Name.Name] {
		return false
	}
	return only == nil || slices.Contains(only, fn.Name.Name) || slices.Contains(only, receiverName(fn))
}

func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func checkAuditFunction(t *testing.T, fset *token.FileSet, fn *ast.FuncDecl, translated func(callee string) bool) {
	t.Helper()
	origins := map[string]string{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if isSelector(n.Fun, "xsderrors", "InternalInvariant") && (len(n.Args) != 2 || !isIdent(n.Args[1], "nil")) {
				t.Errorf("%s: %s builds an internal invariant around a cause; use auditError", fset.Position(n.Pos()), fn.Name.Name)
			}
		case *ast.AssignStmt:
			recordErrorOrigin(origins, n)
		case *ast.ReturnStmt:
			if len(n.Results) == 0 {
				return true
			}
			last := n.Results[len(n.Results)-1]
			var callee string
			switch last := last.(type) {
			case *ast.Ident:
				callee = origins[last.Name]
			case *ast.CallExpr:
				callee = calleeName(last.Fun)
				if strings.HasPrefix(callee, "xsderrors.") {
					return true
				}
			default:
				return true
			}
			if callee != "" && !translated(callee) {
				t.Errorf("%s: %s returns the failure of %s untranslated; use auditError", fset.Position(n.Pos()), fn.Name.Name, callee)
			}
		}
		return true
	})
}

func recordErrorOrigin(origins map[string]string, assign *ast.AssignStmt) {
	if len(assign.Rhs) != 1 {
		return
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	callee := calleeName(call.Fun)
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
			origins[ident.Name] = callee
		}
	}
}

func calleeName(fun ast.Expr) string {
	switch fun := fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if isIdent(fun.X, "xsderrors") {
			return "xsderrors." + fun.Sel.Name
		}
		return fun.Sel.Name
	case *ast.IndexExpr:
		return calleeName(fun.X)
	case *ast.IndexListExpr:
		return calleeName(fun.X)
	default:
		return ""
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
