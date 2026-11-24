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

// publishedElementIDConverters are the declarations of internal/schema
// allowed to convert a value to publishedElementID, the element ID a compiled
// content table names a declaration by. Any other conversion could store a
// build ID in a published table without the numbering's translation.
var publishedElementIDConverters = []string{
	"newCompiledParticleRead",
	"noPublishedElement",
}

func TestOnlyParticleProjectionConvertsToPublishedElementIDs(t *testing.T) {
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
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd/internal/schema", fset, files, info)
	if err != nil {
		t.Fatalf("type-check internal/schema: %v", err)
	}
	published, ok := pkg.Scope().Lookup("publishedElementID").(*types.TypeName)
	if !ok {
		t.Fatal("internal/schema declares no publishedElementID type")
	}
	var converters []string
	for _, file := range files {
		for _, decl := range file.Decls {
			if convertsTo(decl, info, published.Type()) {
				converters = append(converters, declaredName(decl))
			}
		}
	}
	slices.Sort(converters)
	if !slices.Equal(converters, publishedElementIDConverters) {
		t.Fatalf("declarations converting to publishedElementID = %v, want %v", converters, publishedElementIDConverters)
	}
}

func convertsTo(decl ast.Decl, info *types.Info, target types.Type) bool {
	found := false
	ast.Inspect(decl, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok {
			fun := info.Types[call.Fun]
			found = found || fun.IsType() && types.Identical(fun.Type, target)
		}
		return !found
	})
	return found
}

func declaredName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Name.Name
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			if value, ok := spec.(*ast.ValueSpec); ok && len(value.Names) != 0 {
				return value.Names[0].Name
			}
		}
	}
	return "unnamed declaration"
}
