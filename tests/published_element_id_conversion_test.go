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
// copying one: by conversion or by a constant expression of that type, such
// as an untyped constant assigned to it. A function is named by its name, a
// method by its receiver type and name, and a constant or variable
// specification by its names. A publishedElementID is the element ID a
// compiled content table names a declaration by, so any other producer could
// store an ID in a published table without the numbering's translation.
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
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Uses: make(map[*ast.Ident]types.Object)}
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

// declarationUnits returns each function and each constant or variable
// specification of file.
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

// publishedIDProducerCheck finds conversions to target and constant
// expressions of type target other than a use of the constant copied.
type publishedIDProducerCheck struct {
	info   *types.Info
	target types.Type
	copied types.Object
}

func (c publishedIDProducerCheck) produces(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if expr, isExpr := n.(ast.Expr); isExpr && !found {
			found = c.producedBy(expr)
		}
		return !found && !c.copies(n)
	})
	return found
}

func (c publishedIDProducerCheck) producedBy(expr ast.Expr) bool {
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
