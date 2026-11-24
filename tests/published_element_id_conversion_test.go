package tests_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// publishedElementIDProducers are the declarations of internal/schema's
// non-test files allowed to produce a publishedElementID other than by
// copying one. A declaration produces one by a conversion to the type, a
// constant expression of the type (such as an untyped constant assigned to
// it), arithmetic yielding the type, an increment, decrement or arithmetic
// assignment of a value of the type, or a generic instantiation with the type
// as a type argument. A function is named by its name, a method by its
// receiver type and name, and a constant or variable specification by its
// names. A publishedElementID is the element ID a compiled content table
// names a declaration by, so any other producer could store an ID in a table
// without the numbering's translation.
//
// The guard does not see the zero value, which a variable declared without a
// value or a composite literal omitting the field holds, a conversion inside
// a generic function's body, or a value written through reflection or unsafe.
var publishedElementIDProducers = []string{
	"newCompiledParticleRead",
	"noPublishedElement",
}

func TestOnlyParticleProjectionProducesPublishedElementIDs(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal", "schema")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		files = append(files, file)
	}
	info := &types.Info{
		Types:     make(map[ast.Expr]types.TypeAndValue),
		Uses:      make(map[*ast.Ident]types.Object),
		Instances: make(map[*ast.Ident]types.Instance),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd/internal/schema", fset, files, info)
	if err != nil {
		t.Fatalf("type-check internal/schema: %v", err)
	}
	published, ok := pkg.Scope().Lookup("publishedElementID").(*types.TypeName)
	if !ok {
		t.Fatal("internal/schema declares no publishedElementID type")
	}
	noPublished := pkg.Scope().Lookup("noPublishedElement")
	if noPublished == nil {
		t.Fatal("internal/schema declares no noPublishedElement constant")
	}
	check := publishedIDProducerCheck{info: info, target: published.Type(), copied: noPublished}
	var producers []string
	for _, file := range files {
		for _, unit := range declarationUnits(file) {
			if check.produces(unit.node) {
				producers = append(producers, unit.name)
			}
		}
	}
	slices.Sort(producers)
	if !slices.Equal(producers, publishedElementIDProducers) {
		t.Fatalf("declarations producing a publishedElementID = %v, want %v", producers, publishedElementIDProducers)
	}
}

type declarationUnit struct {
	name string
	node ast.Node
}

func declarationUnits(file *ast.File) []declarationUnit {
	var units []declarationUnit
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			units = append(units, declarationUnit{name: functionName(d), node: d})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				names := make([]string, len(value.Names))
				for i, name := range value.Names {
					names[i] = name.Name
				}
				units = append(units, declarationUnit{name: strings.Join(names, ","), node: value})
			}
		}
	}
	return units
}

func functionName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if ident, ok := recv.(*ast.Ident); ok {
		return ident.Name + "." + d.Name.Name
	}
	return d.Name.Name
}

// publishedIDProducerCheck finds the productions of target that
// publishedElementIDProducers lists, other than a use of the constant copied.
type publishedIDProducerCheck struct {
	info   *types.Info
	target types.Type
	copied types.Object
}

func (c publishedIDProducerCheck) produces(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if !found {
			found = c.producedBy(n)
		}
		return !found && !c.copies(n)
	})
	return found
}

func (c publishedIDProducerCheck) producedBy(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.IncDecStmt:
		return c.isTarget(n.X)
	case *ast.AssignStmt:
		return n.Tok != token.ASSIGN && n.Tok != token.DEFINE && len(n.Lhs) == 1 && c.isTarget(n.Lhs[0])
	case *ast.BinaryExpr:
		return isArithmetic(n.Op) && c.isTarget(n)
	case *ast.Ident:
		return c.instantiates(n)
	case ast.Expr:
		return c.producedByExpr(n)
	}
	return false
}

func (c publishedIDProducerCheck) isTarget(expr ast.Expr) bool {
	value, ok := c.info.Types[expr]
	return ok && types.Identical(value.Type, c.target)
}

func (c publishedIDProducerCheck) instantiates(ident *ast.Ident) bool {
	instance, ok := c.info.Instances[ident]
	if !ok {
		return false
	}
	for arg := range instance.TypeArgs.Types() {
		if types.Identical(arg, c.target) {
			return true
		}
	}
	return false
}

func isArithmetic(op token.Token) bool {
	return slices.Contains([]token.Token{token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR, token.SHL, token.SHR, token.AND_NOT}, op)
}

func (c publishedIDProducerCheck) producedByExpr(expr ast.Expr) bool {
	if c.copies(expr) {
		return false
	}
	if call, isCall := expr.(*ast.CallExpr); isCall {
		fun := c.info.Types[call.Fun]
		if fun.IsType() && types.Identical(fun.Type, c.target) {
			return true
		}
	}
	value := c.info.Types[expr]
	return value.Value != nil && types.Identical(value.Type, c.target)
}

func (c publishedIDProducerCheck) copies(n ast.Node) bool {
	ident, isIdent := n.(*ast.Ident)
	return isIdent && c.info.Uses[ident] == c.copied
}
