package tests_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	diagnosticTextImport = "github.com/jacoelho/xsd/internal/diagnostictext"
	xsderrorsImport      = "github.com/jacoelho/xsd/xsderrors"
)

// TestSchemaDiagnosticsQuoteSchemaText is a lint, not a proof. Schema text
// reaches diagnostics unbounded and unescaped unless it passes through
// internal/diagnostictext, so every non-constant string operand of a schema
// diagnostic message must come from an allowlisted callee, a constant, or a
// local variable whose every assignment is itself allowlisted. A function
// parameter that feeds a message makes the function a sink, so its callers are
// checked instead.
//
// The lint has known gaps: it follows only direct data flow, trusts
// error.Error() and the allowlisted label renderers, and cannot see messages
// built through func values or struct fields. The public hostile-value table
// test in the repository root (schema_diagnostic_bound_test.go) is the
// contract proof; this guard only keeps new call sites from bypassing the
// quoting helpers by accident.
func TestSchemaDiagnosticsQuoteSchemaText(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "schema")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	violations := lintSchemaDiagnostics(t, fset, files)
	for _, v := range violations {
		t.Error(v)
	}
}

func TestSchemaDiagnosticLintFixtures(t *testing.T) {
	const header = `package schema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/xsderrors"
)

var _, _, _, _ = fmt.Sprint, strconv.Itoa, strings.TrimSpace, diagnostictext.ExpandedName

type node struct{ name string; items []string }

func (n node) label() string { return "" }

func schemaParseAt(line, col int, code xsderrors.Code, msg string, cause error) error {
	return xsderrors.SchemaParse(code, msg, cause)
}

func formatName(s string) string { return diagnostictext.Quote(s) }

`
	tests := []struct {
		name string
		body string
		bad  bool
	}{
		{"constant", `func f() error { return xsderrors.SchemaCompile("c", "fixed" + "text") }`, false},
		{"quoted", `func f(raw string) error { return xsderrors.SchemaCompile("c", "bad "+diagnostictext.Quote(raw)) }`, false},
		{"int", `func f(n int) error { return xsderrors.SchemaCompile("c", "bad "+strconv.Itoa(n)) }`, false},
		{"error text", `func f(err error) error { return xsderrors.SchemaCompile("c", err.Error()) }`, false},
		{"label method", `func f(n node) error { return xsderrors.SchemaCompile("c", "bad "+n.label()) }`, false},
		{"allowlisted local", `func f(raw string) error { l := formatName(raw); return xsderrors.SchemaCompile("c", "bad "+l) }`, false},
		{"allowlisted sprintf", `func f(raw string, n int) error { return xsderrors.SchemaCompile("c", fmt.Sprintf("bad %s %d", diagnostictext.Quote(raw), n)) }`, false},
		{"parse sink quoted", `func f(raw string) error { return schemaParseAt(1, 1, "c", "bad "+diagnostictext.Quote(raw), nil) }`, false},
		{"wrapper caller quoted", `func wrap(m string) error { return xsderrors.SchemaCompile("c", m) }
func f(raw string) error { return wrap(diagnostictext.Quote(raw)) }`, false},
		{"raw identifier", `func f(id string) error { return xsderrors.SchemaCompile("c", "a"+id) }
func g(n node) error { return f(n.name) }`, true},
		{"raw field", `func f(n node) error { return xsderrors.SchemaCompile("c", "a"+n.name) }`, true},
		{"raw sprintf", `func f(n node) error { return xsderrors.SchemaCompile("c", fmt.Sprintf("bad %s", n.name)) }`, true},
		{"raw errorf", `func f(n node) error { return fmt.Errorf("bad %s", n.name) }`, true},
		{"variable built", `func f(n node) error { msg := "bad " + n.name; return xsderrors.SchemaCompile("c", msg) }`, true},
		{"reassigned local", `func f(n node) error { l := formatName(n.name); l += n.name; return xsderrors.SchemaCompile("c", l) }`, true},
		{"trim bypass", `func f(n node) error { return xsderrors.SchemaCompile("c", "bad "+strings.TrimSpace(n.name)) }`, true},
		{"conversion bypass", `func f(b []byte) error { return xsderrors.SchemaCompile("c", string(b)) }`, true},
		{"wrapper caller raw", `func wrap(m string) error { return xsderrors.SchemaCompile("c", m) }
func f(n node) error { return wrap("bad " + n.name) }`, true},
		{"raw parse at", `func f(n node) error { return schemaParseAt(1, 1, "c", "bad "+n.name, nil) }`, true},
		{"wrapper caller quoted by local", `func wrap(m string) error { return xsderrors.SchemaCompile("c", m) }
func f(n node) error { q := diagnostictext.Quote(n.name); return wrap("bad " + q) }`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "fixture.go", header+tt.body, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := lintSchemaDiagnostics(t, fset, []*ast.File{file})
			if tt.bad && len(got) == 0 {
				t.Fatal("expected a violation, got none")
			}
			if !tt.bad && len(got) != 0 {
				t.Fatalf("unexpected violations: %v", got)
			}
		})
	}
}

// schemaDiagnosticLabelFuncs render names or enum labels that never carry
// unbounded schema text; formatName delegates to diagnostictext.
var schemaDiagnosticLabelFuncs = map[string]bool{
	"formatName": true, "label": true, "Label": true,
	"cycleLabel": true, "missingLabel": true, "facetLabel": true,
}

// schemaDiagnosticFields are struct fields populated from vocabulary constants
// or already-quoted text at construction. The lint does not check the
// constructors, which is one of its known gaps.
var schemaDiagnosticFields = map[string]bool{
	"childOrderError.Message":                 true,
	"derivationSetParser.label":               true,
	"contentDerivationBase.Container":         true,
	"contentDerivationBase.Derivation":        true,
	"substitutionMembershipLabels.MemberName": true,
	"substitutionMembershipLabels.MemberType": true,
	"substitutionMembershipLabels.HeadName":   true,
	"substitutionMembershipLabels.HeadType":   true,
}

var schemaDiagnosticStrconv = map[string]bool{
	"Itoa": true, "FormatInt": true, "FormatUint": true, "Quote": true,
}

type diagnosticLint struct {
	fset     *token.FileSet
	pkg      *types.Package
	info     *types.Info
	assigns  map[types.Object][]ast.Expr // nil entry: assignment the lint cannot follow
	params   map[types.Object]paramSlot
	decls    map[*types.Func]*ast.FuncDecl
	sinks    map[*types.Func]map[int]bool
	changed  bool
	report   bool
	problems map[string]bool
}

type paramSlot struct {
	fn    *types.Func
	index int
}

func lintSchemaDiagnostics(t *testing.T, fset *token.FileSet, files []*ast.File) []string {
	t.Helper()
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd/internal/schema", fset, files, info)
	if err != nil {
		t.Fatalf("type-check internal/schema: %v", err)
	}
	l := &diagnosticLint{
		fset: fset, pkg: pkg, info: info,
		assigns:  make(map[types.Object][]ast.Expr),
		params:   make(map[types.Object]paramSlot),
		decls:    make(map[*types.Func]*ast.FuncDecl),
		sinks:    make(map[*types.Func]map[int]bool),
		problems: make(map[string]bool),
	}
	l.collect(files)
	// Parameters that feed messages turn their functions into sinks; iterate
	// until the set is stable, then report.
	for {
		l.changed = false
		l.walk(files)
		if !l.changed {
			break
		}
	}
	l.report = true
	l.walk(files)
	out := make([]string, 0, len(l.problems))
	for p := range l.problems {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (l *diagnosticLint) obj(id *ast.Ident) types.Object {
	if o := l.info.Defs[id]; o != nil {
		return o
	}
	return l.info.Uses[id]
}

func (l *diagnosticLint) collect(files []*ast.File) {
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				fn, ok := l.info.Defs[n.Name].(*types.Func)
				if !ok {
					return true
				}
				l.decls[fn] = n
				index := 0
				for _, field := range n.Type.Params.List {
					if len(field.Names) == 0 {
						index++
					}
					for _, name := range field.Names {
						if o := l.info.Defs[name]; o != nil {
							l.params[o] = paramSlot{fn, index}
						}
						index++
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range n.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					o := l.obj(id)
					if o == nil {
						continue
					}
					if (n.Tok == token.ASSIGN || n.Tok == token.DEFINE) && len(n.Lhs) == len(n.Rhs) {
						l.assigns[o] = append(l.assigns[o], n.Rhs[i])
					} else {
						l.assigns[o] = append(l.assigns[o], nil)
					}
				}
			case *ast.ValueSpec:
				for i, id := range n.Names {
					o := l.info.Defs[id]
					if o == nil {
						continue
					}
					if len(n.Values) == len(n.Names) {
						l.assigns[o] = append(l.assigns[o], n.Values[i])
					} else if len(n.Values) > 0 {
						l.assigns[o] = append(l.assigns[o], nil)
					}
				}
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := e.(*ast.Ident); ok {
						if o := l.obj(id); o != nil {
							l.assigns[o] = append(l.assigns[o], nil)
						}
					}
				}
			case *ast.IncDecStmt:
				if id, ok := n.X.(*ast.Ident); ok {
					if o := l.obj(id); o != nil {
						l.assigns[o] = append(l.assigns[o], nil)
					}
				}
			}
			return true
		})
	}
}

func (l *diagnosticLint) callee(call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, ok := l.info.Uses[id].(*types.Func)
	if !ok {
		return nil
	}
	return fn
}

// sinkArgs returns the argument indexes of call that carry message text.
func (l *diagnosticLint) sinkArgs(call *ast.CallExpr) map[int]bool {
	fn := l.callee(call)
	if fn == nil || fn.Pkg() == nil {
		return nil
	}
	path, name := fn.Pkg().Path(), fn.Name()
	switch {
	case path == xsderrorsImport && (name == "SchemaCompile" || name == "SchemaParse"):
		return map[int]bool{1: true}
	case path == "fmt" && name == "Errorf":
		out := make(map[int]bool)
		for i := range call.Args {
			out[i] = true
		}
		return out
	case path == l.pkg.Path() && name == "schemaParseAt":
		return map[int]bool{3: true}
	case path == l.pkg.Path() && name == "schemaCompileAt":
		return map[int]bool{2: true}
	}
	return l.sinks[fn]
}

func (l *diagnosticLint) walk(files []*ast.File) {
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for i := range l.sinkArgs(call) {
				if i < len(call.Args) {
					l.checkOperand(call.Args[i], 0)
				}
			}
			return true
		})
	}
}

func (l *diagnosticLint) flag(e ast.Expr, why string) {
	if l.report {
		l.problems[l.fset.Position(e.Pos()).String()+": "+why] = true
	}
}

func isErrorOrNumeric(t types.Type) bool {
	if t == nil {
		return false
	}
	if types.Identical(t, types.Universe.Lookup("error").Type()) {
		return true
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&(types.IsNumeric|types.IsBoolean) != 0
}

func (l *diagnosticLint) checkOperand(e ast.Expr, depth int) {
	e = ast.Unparen(e)
	tv := l.info.Types[e]
	if tv.Value != nil || isErrorOrNumeric(tv.Type) {
		return
	}
	if depth > 8 {
		l.flag(e, "operand nests too deeply to follow")
		return
	}
	switch e := e.(type) {
	case *ast.BinaryExpr:
		l.checkOperand(e.X, depth+1)
		l.checkOperand(e.Y, depth+1)
	case *ast.CallExpr:
		l.checkCall(e, depth)
	case *ast.Ident:
		l.checkIdent(e, depth)
	case *ast.SelectorExpr:
		if !l.allowedField(e) {
			l.flag(e, "field is not an allowlisted quoted value")
		}
	default:
		l.flag(e, "operand is not quoted through internal/diagnostictext")
	}
}

func (l *diagnosticLint) checkCall(call *ast.CallExpr, depth int) {
	fn := l.callee(call)
	if fn == nil {
		l.flag(call, "call is not an allowlisted quoting callee")
		return
	}
	path, name := "", fn.Name()
	if fn.Pkg() != nil {
		path = fn.Pkg().Path()
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		l.flag(call, "callee has no signature")
		return
	}
	switch {
	case path == diagnosticTextImport:
	case path == "strconv" && schemaDiagnosticStrconv[name]:
	case path == "fmt" && (name == "Sprintf" || name == "Errorf"):
		for _, arg := range call.Args {
			l.checkOperand(arg, depth+1)
		}
	case name == "Error" && sig.Recv() != nil && sig.Params().Len() == 0:
	case path == l.pkg.Path() && schemaDiagnosticLabelFuncs[name]:
	default:
		decl := l.decls[fn]
		if decl == nil || decl.Body == nil || sig.Results().Len() != 1 {
			l.flag(call, "call to "+path+"."+name+" is not an allowlisted quoting callee")
			return
		}
		// A same-package string function is as safe as everything it returns.
		ast.Inspect(decl.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt:
				for _, result := range n.Results {
					l.checkOperand(result, depth+1)
				}
			}
			return true
		})
	}
}

func (l *diagnosticLint) checkIdent(id *ast.Ident, depth int) {
	o := l.obj(id)
	if o == nil {
		l.flag(id, "unresolved identifier")
		return
	}
	if slot, ok := l.params[o]; ok {
		if l.sinks[slot.fn] == nil {
			l.sinks[slot.fn] = make(map[int]bool)
		}
		if !l.sinks[slot.fn][slot.index] {
			l.sinks[slot.fn][slot.index] = true
			l.changed = true
		}
		return
	}
	rhs := l.assigns[o]
	if len(rhs) == 0 {
		l.flag(id, "variable "+id.Name+" has no allowlisted assignment")
		return
	}
	for _, expr := range rhs {
		if expr == nil {
			l.flag(id, "variable "+id.Name+" has an assignment the lint cannot follow")
			return
		}
		l.checkOperand(expr, depth+1)
	}
}

func (l *diagnosticLint) allowedField(sel *ast.SelectorExpr) bool {
	selection := l.info.Selections[sel]
	if selection == nil || selection.Kind() != types.FieldVal {
		return false
	}
	recv := selection.Recv()
	if ptr, ok := recv.(*types.Pointer); ok {
		recv = ptr.Elem()
	}
	named, ok := types.Unalias(recv).(*types.Named)
	return ok && schemaDiagnosticFields[named.Obj().Name()+"."+selection.Obj().Name()]
}
