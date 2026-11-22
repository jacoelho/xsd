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
// reads. A function is a function declaration or a function literal bound to a
// package-level variable, keyed by the variable's name. It reads a list when it
// selects the Particles field, or when it ranges over or indexes a
// []contentParticle parameter that the field, or a subslice of it, reaches
// through static calls within the package; reading only the length, or an
// element at an integer-literal index, is not a read. The guard does not catch
// an uncharged loop added inside a listed function, a list stored in a
// variable, field or closure, or returned, before it is read, a list passed to
// a variadic parameter or through a function value, or a derived list such as
// a continuation set.
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
	"contentModelCompiler.sequenceHasWildcardEquivalentOverlap": "ParticleContinuations charges each member but the last, and wildcardCandidateOverlaps each following member it reads for a wildcard candidate",
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

	"contentModelCompiler.particlesNeedUPACheck":                    "callee particleSetNeedsUPACheck charges each pair it reads",
	"contentModelCompiler.checkPairwiseUPA":                         "particlesNeedUPACheck and checkParticleAgainstFollowingUPA charge each pair",
	"contentModelCompiler.checkParticleAgainstFollowingUPA":         "charges each following particle it reads",
	"contentModelCompiler.wildcardCandidateOverlaps":                "charges each following member it reads",
	"dfaBuilder.sequenceNode":                                       "callee particleNode charges each particle",
	"dfaBuilder.choiceNode":                                         "callee particleNode charges each particle",
	"elementDeclarationConsistencyChecker.collectParticles":         "charges each particle, once per model",
	"elementDeclarationConsistencyChecker.particleDeclarationsHint": "caller collectParticles charges the same particles, once per model",
	"contentRestrictionValidator.matchDerivedChoiceSpan":            "callee matchChoiceSpanBranch compares and charges a base particle for each derived particle it reads",
	"contentRestrictionValidator.matchChoiceSpanBranch":             "compareParticleRestriction charges each base particle compared",
	"contentRestrictionValidator.choiceRestrictionBranchAllowed":    "compareParticleRestriction charges each base particle compared",
	"contentRestrictionValidator.matchChoiceRestrictionBranch":      "compareParticleRestriction charges each base particle compared",
	"contentRestrictionValidator.findMappedGroupParticle":           "reads only unmapped base particles, each charged by compareParticleRestriction",
	"contentRestrictionValidator.validateUnmappedGroupParticles":    "reads only unmapped base particles, each charged by ParticleEmptiable",
	"contentModelAnalysis.ParticleContinuations":                    "charges the one particle it reads",
	"contentModelAnalysis.sequenceStart":                            "charges each particle it reads",
	"contentModelAnalysis.sequenceCountRange":                       "callee ParticleCountRange charges each particle",
	"contentModelAnalysis.choiceCountRange":                         "callee ParticleCountRange charges each particle",
	"contentModelAnalysis.allParticlesEmptiable":                    "callee cachedParticleEmptiable charges each particle",
	"contentModelAnalysis.anyParticleEmptiable":                     "callee cachedParticleEmptiable charges each particle",
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
	info := &types.Info{
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
	}
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

	functions := packageFunctions(files)
	lists := particleListFlow{info: info, particles: particles, params: map[*types.Var]bool{}}
	lists.taintParams(functions)
	readers := map[string]bool{}
	report := func(key string, at token.Pos, how string) {
		readers[key] = true
		if _, ok := particleListReaders[key]; !ok {
			t.Errorf("%s: %s %s without chargedParticles", fset.Position(at), key, how)
		}
	}
	for _, fn := range functions {
		for _, sel := range lists.fieldReads(fn.node) {
			report(fn.key, sel.Pos(), "reads a content model's particle list")
		}
		for _, at := range lists.paramReads(fn.node) {
			report(fn.key, at, "reads a particle list handed to it")
		}
	}
	for _, name := range slices.Sorted(maps.Keys(particleListReaders)) {
		if !readers[name] {
			t.Errorf("allowed particle-list reader %s does not exist or reads no particle list", name)
		}
	}
}

// packageFunction is a function declaration, or a function literal bound to a
// package-level variable, with the name the allowlist knows it by.
type packageFunction struct {
	node ast.Node
	key  string
}

func packageFunctions(files []*ast.File) []packageFunction {
	var functions []packageFunction
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				functions = append(functions, packageFunction{node: d, key: funcKey(d)})
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, v := range value.Values {
						if lit, ok := v.(*ast.FuncLit); ok && i < len(value.Names) {
							functions = append(functions, packageFunction{node: lit, key: value.Names[i].Name})
						}
					}
				}
			}
		}
	}
	return functions
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

// particleListFlow follows a content model's particle list from the
// Particles field into the []contentParticle parameters it is passed to,
// directly or as a subslice, through any chain of static calls in the
// package. params holds every parameter so reached.
type particleListFlow struct {
	info      *types.Info
	particles *types.Var
	params    map[*types.Var]bool
}

func (f *particleListFlow) taintParams(functions []packageFunction) {
	for changed := true; changed; {
		changed = false
		for _, fn := range functions {
			ast.Inspect(fn.node, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || call.Ellipsis.IsValid() {
					return true
				}
				callee := f.callee(call)
				if callee == nil {
					return true
				}
				sig := callee.Signature()
				for i, arg := range call.Args {
					if i >= sig.Params().Len() || (sig.Variadic() && i >= sig.Params().Len()-1) {
						break
					}
					param := sig.Params().At(i)
					if _, done := f.params[param]; done || !f.carriesList(arg) {
						continue
					}
					f.params[param] = true
					changed = true
				}
				return true
			})
		}
	}
}

func (f *particleListFlow) callee(call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	default:
		return nil
	}
	fn, ok := f.info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg() != f.particles.Pkg() {
		return nil
	}
	return fn
}

// carriesList reports whether expr is a particle list: the Particles field, a
// parameter the list reached, or a subslice of either.
func (f *particleListFlow) carriesList(expr ast.Expr) bool {
	if slice, ok := expr.(*ast.SliceExpr); ok {
		expr = slice.X
	}
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		selection := f.info.Selections[x]
		return selection != nil && selection.Obj() == f.particles
	case *ast.Ident:
		v, ok := f.info.Uses[x].(*types.Var)
		_, reached := f.params[v]
		return ok && reached
	}
	return false
}

// fieldReads returns the selectors in fn that read the particles field other
// than as the operand of len or indexed by an integer literal.
func (f *particleListFlow) fieldReads(fn ast.Node) []*ast.SelectorExpr {
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
		if selection := f.info.Selections[sel]; selection != nil && selection.Obj() == f.particles {
			reads = append(reads, sel)
		}
		return true
	})
	return reads
}

// paramReads returns where fn, including the closures it contains, ranges
// over a parameter the particle list reached, or a subslice of one, or indexes
// one other than by an integer literal.
func (f *particleListFlow) paramReads(fn ast.Node) []token.Pos {
	var reads []token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.RangeStmt:
			if f.isParamList(x.X) {
				reads = append(reads, x.X.Pos())
			}
		case *ast.IndexExpr:
			if _, constant := x.Index.(*ast.BasicLit); !constant && f.isParamList(x.X) {
				reads = append(reads, x.Pos())
			}
		}
		return true
	})
	return reads
}

func (f *particleListFlow) isParamList(expr ast.Expr) bool {
	_, field := expr.(*ast.SelectorExpr)
	return !field && f.carriesList(expr)
}
