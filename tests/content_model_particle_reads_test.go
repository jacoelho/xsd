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

// particleListReaders are the functions of internal/schema, keyed by receiver
// type and name, allowed to read a content model's particle list other than
// through chargedParticles, each with the owner that charges the particles it
// reads. Reading only the length, or an element at an integer-literal index,
// is not a read. The guard catches a function that starts reading a particle
// list; it does not catch an uncharged loop added inside a listed function, or
// a helper that is handed the particle slice and reads it uncharged.
var particleListReaders = map[string]string{
	"chargedParticles":           "the charged accessor",
	"appendModelParticles":       "the table writer that charges each particle it appends",
	"compiler.appendModel":       "the table writer",
	"compiler.completeModel":     "the table writer",
	"validateContentModelShape":  "called by the restriction graph check before its charged loop over the same particles",
	"modelParticlePreservesBase": "called once per extension type, whose root automaton is compiled from the base particles and charged first",

	"normalizeSingleParticleModel":                              "reads the one particle that its caller executionModel yielded and charged",
	"elementDeclarationConsistencyChecker.collectModel":         "callee collectParticles charges each particle, once per model",
	"contentModelCompiler.checkAllUPA":                          "callee checkPairwiseUPA charges each pair",
	"contentModelCompiler.checkDirectUPA":                       "callees checkPairwiseUPA and checkSequenceUPA charge each pair and member",
	"contentModelCompiler.checkSequenceUPA":                     "callee ParticleContinuations charges each member",
	"contentModelCompiler.checkSequenceContinuationUPA":         "charges each following member it reads",
	"contentModelCompiler.compileAllModel":                      "callee checkAllUPA charges every pair before the term loop reads the same particles",
	"contentModelCompiler.modelNeedsRuntimeSplitSeen":           "callee particleNeedsRuntimeSplit charges each particle, once per model per root",
	"contentModelCompiler.sequenceHasWildcardEquivalentOverlap": "ParticleContinuations charges each member, and wildcardCandidateOverlaps, through candidatesHaveWildcardEquivalentOverlap, each following member it reads",
	"contentModelCompiler.yieldExecutionParticles":              "callee yieldExecutionParticle charges each particle",
	"dfaBuilder.groupNode":                                      "callees sequenceNode and choiceNode charge each particle through particleNode",

	"restrictionNormalizer.children":     "callee child charges each particle through particle",
	"restrictionNormalizer.child":        "takes a spliced group's list unread; appendChild, reached from children through replaceChild, charges each member",
	"restrictionNormalizer.replaceChild": "callee copyPrefix charges the whole list",

	"contentRestrictionValidator.modelContainsWildcard":               "callee particleContainsWildcard charges each particle",
	"contentRestrictionValidator.validateChoiceRestriction":           "compareParticleRestriction charges each base particle compared, and validateChoiceSpans charges the base list",
	"contentRestrictionValidator.validateContentModelGraphWithStates": "charges each particle, once per model per restriction",
	"contentRestrictionValidator.validateMappedGroupRestriction":      "callees findMappedGroupParticle and validateUnmappedGroupParticles read only unmapped base particles, each charged by compareParticleRestriction or ParticleEmptiable",
	"contentRestrictionValidator.validateModelRestrictsWildcard":      "callee compareParticleRestriction charges each member",
	"contentRestrictionValidator.validateOrderedGroupRestriction":     "orderedGroupRestrictionMatcher charges each base particle it reads through compareParticleRestriction or ParticleEmptiable",
	"contentRestrictionValidator.validateSequenceRestrictsChoice":     "compareParticleRestriction charges each base particle compared",

	"contentModelAnalysis.lastParticles":       "charges each sequence member through cachedParticleEmptiable; caller continuationsOf charges each choice member, once per model",
	"contentModelAnalysis.startParticles":      "charges each particle it returns",
	"contentModelAnalysis.modelEmptiableValue": "callees allParticlesEmptiable and anyParticleEmptiable charge each particle through cachedParticleEmptiable, once per model",
	"contentModelAnalysis.modelStartOverlap":   "callee modelStartParticleOverlap charges each particle",
	"contentModelAnalysis.modelTermCountRange": "callees sequenceCountRange and choiceCountRange charge each particle through ParticleCountRange",
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
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			key := funcKey(fn)
			for _, sel := range particleListReads(fn, info, particles) {
				readers[key] = true
				if _, ok := particleListReaders[key]; !ok {
					t.Errorf("%s: %s reads a content model's particle list without chargedParticles", fset.Position(sel.Pos()), key)
				}
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(particleListReaders)) {
		if !readers[name] {
			t.Errorf("allowed particle-list reader %s does not exist or reads no particle list", name)
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
