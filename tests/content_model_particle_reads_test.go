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
// selects the Particles field, or uses a parameter that the field, or a
// subslice of it, reaches as an argument of static calls within the package,
// including the receiver of a method expression. Using the list only as the
// operand of len, indexed by an integer literal, or as an argument, whole or
// subsliced, that is followed into another function, is not a read.
//
// The guard does not catch an uncharged loop added inside a listed function, a
// list passed through interface method dispatch, a function value, a variadic
// parameter or an ellipsis, a reader that is a function literal nested inside
// a package-level initializer, a list stored in a variable, field or closure,
// or returned, and read by another function, or a list derived from the
// particles, such as a continuation set.
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
	"contentModelAnalysis.leafContinuations":                        "returns as a continuation set the one particle its caller ParticleContinuations charged",
	"restrictionNormalizer.copyPrefix":                              "charges the whole list before copying its prefix",
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
// Particles field into the parameters it is passed to, directly or as a
// subslice, through any chain of static calls in the package, including the
// receiver of a method expression. params holds every parameter so reached.
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
				if !ok {
					return true
				}
				for i, param := range f.argParams(call) {
					if param == nil || f.params[param] || !f.carriesList(call.Args[i]) {
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

// argParams returns, for each argument of a static call to a function of the
// package, the parameter it binds, or nil for an argument the flow does not
// follow: one bound to a variadic parameter or spread with an ellipsis.
func (f *particleListFlow) argParams(call *ast.CallExpr) []*types.Var {
	var id *ast.Ident
	methodExpr := false
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
		if selection := f.info.Selections[fun]; selection != nil {
			methodExpr = selection.Kind() == types.MethodExpr
		}
	default:
		return nil
	}
	fn, ok := f.info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() != f.particles.Pkg() || call.Ellipsis.IsValid() {
		return nil
	}
	sig := fn.Signature()
	var params []*types.Var
	if methodExpr {
		params = append(params, sig.Recv())
	}
	for i := range sig.Params().Len() {
		if sig.Variadic() && i == sig.Params().Len()-1 {
			break
		}
		params = append(params, sig.Params().At(i))
	}
	bound := make([]*types.Var, len(call.Args))
	copy(bound, params)
	return bound
}

// passedOn reports whether expr is an argument that argParams follows.
func (f *particleListFlow) passedOn(call *ast.CallExpr, expr ast.Expr) bool {
	params := f.argParams(call)
	for i, arg := range call.Args {
		if arg == expr {
			return i < len(params) && params[i] != nil
		}
	}
	return false
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
		return ok && f.params[v]
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

// paramReads returns every use, in fn and the closures it contains, of a
// parameter the particle list reached, other than as the operand of len, an
// index by an integer literal, or an argument, whole or subsliced, that the
// flow follows into another function.
func (f *particleListFlow) paramReads(fn ast.Node) []token.Pos {
	var reads []token.Pos
	var stack []ast.Node
	ast.Inspect(fn, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := f.info.Uses[id].(*types.Var); ok && f.params[v] && !f.boundedUse(id, stack) {
				reads = append(reads, id.Pos())
			}
		}
		stack = append(stack, n)
		return true
	})
	return reads
}

func (f *particleListFlow) boundedUse(id *ast.Ident, stack []ast.Node) bool {
	if len(stack) == 0 {
		return false
	}
	var expr ast.Expr = id
	parent := stack[len(stack)-1]
	if slice, ok := parent.(*ast.SliceExpr); ok && slice.X == id && len(stack) > 1 {
		expr, parent = slice, stack[len(stack)-2]
	}
	switch p := parent.(type) {
	case *ast.CallExpr:
		if fun, ok := p.Fun.(*ast.Ident); ok && expr == id && fun.Name == "len" && f.info.Uses[fun] == types.Universe.Lookup("len") {
			return true
		}
		return f.passedOn(p, expr)
	case *ast.IndexExpr:
		lit, ok := p.Index.(*ast.BasicLit)
		return expr == id && p.X == id && ok && lit.Kind == token.INT
	}
	return false
}
