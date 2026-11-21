package tests_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// particleListReaders are the readers of a content model's particle list
// allowed outside chargedParticles, functions keyed by receiver type and name.
// Every other function reads the list through chargedParticles, or reads only
// its length or an element at a constant index.
var particleListReaders = struct {
	files     map[string]string
	functions map[string]string
}{
	files: map[string]string{
		"content_compile.go":               "per-root analysis and Element Declarations Consistent, charged per particle visited",
		"content_restriction.go":           "Particle Valid (Restriction), charged per particle compared",
		"content_restriction_normalize.go": "restriction normalizer, charged per particle normalized",
		"particle_continuation.go":         "continuation sets, memoized per model and charged per particle",
		"particle_overlap.go":              "emptiability and count ranges, memoized per model and charged per particle",
	},
	functions: map[string]string{
		"chargedParticles":               "the charged accessor",
		"appendModelParticles":           "the table writer that charges each particle it appends",
		"compiler.appendModel":           "the table writer",
		"compiler.completeModel":         "the table writer",
		"contentModelTableAudit.advance": "reads one particle per step of a model that enter charged through chargedParticles",
		"validateContentModelShape":      "called by the restriction graph check before its charged loop over the same particles",
		"modelParticlePreservesBase":     "called once per extension type, whose root automaton is compiled from the base particles and charged first",
	},
}

func TestContentModelParticleListsAreReadThroughChargedParticles(t *testing.T) {
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
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd/internal/schema", fset, files, info)
	if err != nil {
		t.Fatalf("type-check internal/schema: %v", err)
	}
	model, ok := pkg.Scope().Lookup("contentModelNode").Type().Underlying().(*types.Struct)
	if !ok {
		t.Fatal("contentModelNode is not a struct")
	}
	particles := fieldByName(model, "Particles")
	if particles == nil {
		t.Fatal("contentModelNode has no Particles field")
	}

	readers := map[string]bool{}
	for _, file := range files {
		name := filepath.Base(fset.Position(file.Pos()).Filename)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			key := funcKey(fn)
			for _, sel := range particleListReads(fn, info, particles) {
				readers[key] = true
				if _, ok := particleListReaders.files[name]; ok {
					continue
				}
				if _, ok := particleListReaders.functions[key]; ok {
					continue
				}
				t.Errorf("%s: %s reads a content model's particle list without chargedParticles", fset.Position(sel.Pos()), key)
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(particleListReaders.functions)) {
		if !readers[name] {
			t.Errorf("allowed particle-list reader %s reads no particle list", name)
		}
	}
}

// funcKey names fn by its receiver's type name, if any, and its own name.
func funcKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return fn.Name.Name
	}
	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// particleListReads returns the selectors in fn that read the particles field
// other than as the operand of len or indexed by an integer literal.
func particleListReads(fn *ast.FuncDecl, info *types.Info, particles *types.Var) []*ast.SelectorExpr {
	bounded := map[*ast.SelectorExpr]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "len" && len(x.Args) == 1 {
				if sel, ok := x.Args[0].(*ast.SelectorExpr); ok {
					bounded[sel] = true
				}
			}
		case *ast.IndexExpr:
			sel, ok := x.X.(*ast.SelectorExpr)
			if lit, constant := x.Index.(*ast.BasicLit); ok && constant && lit.Kind == token.INT {
				bounded[sel] = true
			}
		}
		return true
	})
	var reads []*ast.SelectorExpr
	ast.Inspect(fn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || bounded[sel] {
			return true
		}
		if selection := info.Selections[sel]; selection != nil && selection.Obj() == particles {
			reads = append(reads, sel)
		}
		return true
	})
	return reads
}
