package tests_test

import (
	"errors"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestInternalInvariantMessagesCarryNoErrorText type-checks every package
// that imports or is xsderrors and fails when an operand of
// an error type reaches the message argument: directly, through a call such as
// err.Error() or fmt.Sprintf("%v", err), through string concatenation, or
// through a local variable assigned in the same package. Parameters are not
// followed into callers.
func TestInternalInvariantMessagesCarryNoErrorText(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	dirs := internalInvariantCallerDirs(t, root)
	if len(dirs) == 0 {
		t.Fatal("no package calls xsderrors.InternalInvariant")
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "source", nil)
	for _, dir := range dirs {
		for _, violation := range internalInvariantErrorTextViolations(t, fset, imp, root, dir) {
			t.Errorf("%s: InternalInvariant message carries an error's text; pass the error as the cause", violation)
		}
	}
}

func TestInternalInvariantErrorTextLintFixtures(t *testing.T) {
	t.Parallel()
	const header = `package fixture

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/xsderrors"
)

var _, _ = errors.New, fmt.Sprint

type readerError struct{}

func (*readerError) Error() string { return "reader" }

`
	tests := []struct {
		name string
		body string
		bad  bool
	}{
		{"constant", `func f() error { return xsderrors.InternalInvariant("broken", nil) }`, false},
		{"cause argument", `func f(err error) error { return xsderrors.InternalInvariant("broken", err) }`, false},
		{"string parameter", `func f(m string) error { return xsderrors.InternalInvariant("broken: "+m, nil) }`, false},
		{"Error call", `func f(err error) error { return xsderrors.InternalInvariant(err.Error(), nil) }`, true},
		{"concatenated Error call", `func f(err error) error { return xsderrors.InternalInvariant("broken: "+err.Error(), nil) }`, true},
		{"Sprintf operand", `func f(err error) error { return xsderrors.InternalInvariant(fmt.Sprintf("broken: %v", err), nil) }`, true},
		{"concrete error type", `func f(err *readerError) error { return xsderrors.InternalInvariant(fmt.Sprint(err), nil) }`, true},
		{"local assigned from error text", `func f(err error) error { m := "broken"; m += ": " + err.Error(); return xsderrors.InternalInvariant(m, nil) }`, true},
		{"local chain", `func f(err error) error { a := err.Error(); b := "broken: " + a; return xsderrors.InternalInvariant(b, nil) }`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "fixture.go", header+tt.body, 0)
			if err != nil {
				t.Fatal(err)
			}
			info := newInvariantLintInfo()
			conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
			if _, err := conf.Check("fixture", fset, []*ast.File{file}, info); err != nil {
				t.Fatal(err)
			}
			got := invariantLintFiles(fset, info, []*ast.File{file})
			if tt.bad != (len(got) > 0) {
				t.Fatalf("violations = %v, want bad = %v", got, tt.bad)
			}
		})
	}
}

func internalInvariantCallerDirs(t *testing.T, root string) []string {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if p != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata" || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		pkg, err := build.ImportDir(p, 0)
		if err != nil {
			if _, ok := errors.AsType[*build.NoGoError](err); ok {
				return nil
			}
			return err
		}
		for _, name := range pkg.GoFiles {
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(p, name), nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			if importsOrIs(file, xsderrorsImport) {
				dirs = append(dirs, p)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}

func importsOrIs(file *ast.File, importPath string) bool {
	if file.Name.Name == path.Base(importPath) {
		return true
	}
	return slices.ContainsFunc(file.Imports, func(spec *ast.ImportSpec) bool {
		return strings.Trim(spec.Path.Value, `"`) == importPath
	})
}

func internalInvariantErrorTextViolations(t *testing.T, fset *token.FileSet, imp types.Importer, root, dir string) []string {
	t.Helper()
	pkg, err := build.ImportDir(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	files := make([]*ast.File, 0, len(pkg.GoFiles))
	for _, name := range pkg.GoFiles {
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		files = append(files, file)
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	importPath := path.Join("github.com/jacoelho/xsd", filepath.ToSlash(rel))
	info := newInvariantLintInfo()
	conf := types.Config{Importer: imp}
	if _, err := conf.Check(importPath, fset, files, info); err != nil {
		t.Fatalf("type-check %s: %v", importPath, err)
	}
	return invariantLintFiles(fset, info, files)
}

func newInvariantLintInfo() *types.Info {
	return &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
}

func invariantLintFiles(fset *token.FileSet, info *types.Info, files []*ast.File) []string {
	assigns := collectAssignments(info, files)
	errorType, ok := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	if !ok {
		panic("universe error is not an interface")
	}
	var violations []string
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !isInternalInvariantCallee(info, call) {
				return true
			}
			if operand := errorOperand(info, assigns, errorType, call.Args[0], map[types.Object]bool{}); operand != nil {
				violations = append(violations, fset.Position(operand.Pos()).String())
			}
			return true
		})
	}
	return violations
}

func isInternalInvariantCallee(info *types.Info, call *ast.CallExpr) bool {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	default:
		return false
	}
	fn, ok := info.Uses[id].(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == xsderrorsImport && fn.Name() == "InternalInvariant"
}

// collectAssignments maps each variable to every expression assigned to it.
func collectAssignments(info *types.Info, files []*ast.File) map[types.Object][]ast.Expr {
	assigns := make(map[types.Object][]ast.Expr)
	object := func(id *ast.Ident) types.Object {
		if obj := info.Defs[id]; obj != nil {
			return obj
		}
		return info.Uses[id]
	}
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for i, lhs := range n.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					if obj := object(id); obj != nil {
						if len(n.Lhs) == len(n.Rhs) {
							assigns[obj] = append(assigns[obj], n.Rhs[i])
						} else {
							assigns[obj] = append(assigns[obj], n.Rhs...)
						}
					}
				}
			case *ast.ValueSpec:
				for i, id := range n.Names {
					obj := info.Defs[id]
					if obj == nil {
						continue
					}
					if len(n.Values) == len(n.Names) {
						assigns[obj] = append(assigns[obj], n.Values[i])
					} else {
						assigns[obj] = append(assigns[obj], n.Values...)
					}
				}
			}
			return true
		})
	}
	return assigns
}

// errorOperand returns the first operand of an error type within expr, also
// searching the expressions assigned to the variables expr reads.
func errorOperand(info *types.Info, assigns map[types.Object][]ast.Expr, errorType *types.Interface, expr ast.Expr, seen map[types.Object]bool) ast.Expr {
	var found ast.Expr
	ast.Inspect(expr, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if tv, typed := info.Types[e]; typed && tv.Type != nil && !tv.IsType() && !tv.IsNil() && types.Implements(tv.Type, errorType) {
			found = e
			return false
		}
		id, ok := e.(*ast.Ident)
		if !ok {
			return true
		}
		obj, ok := info.Uses[id].(*types.Var)
		if !ok || seen[obj] {
			return true
		}
		seen[obj] = true
		for _, assigned := range assigns[obj] {
			if operand := errorOperand(info, assigns, errorType, assigned, seen); operand != nil {
				found = operand
				return false
			}
		}
		return true
	})
	return found
}
