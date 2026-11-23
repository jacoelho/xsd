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
// reads. A function is a function declaration or a package-level variable
// declaration, keyed by its first variable's name, with every function
// literal either contains. It reads a list when it selects the Particles
// field other than as the operand of len or indexed by an integer literal,
// or uses a holder the list reaches, as particleListFlow defines, other than
// in the ways particleListFlow.reads exempts. A list passed to an interface
// method, a function value, or a variadic parameter without an ellipsis is
// read by the caller.
//
// The guard does not catch an uncharged loop added inside a listed function;
// TestContentModelCompileWorkGrowsWithChargedSteps catches such a loop only
// when its work grows faster than the charged steps. Nor does it follow a list
// stored as an element of an array, map or slice, such as a continuationSets
// array or the continuation cache, and read through that element by another
// function, or a list whose particles are copied into another slice, such as
// a merged continuation set.
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
	"restrictionNormalizer.appendChild":                             "charges a spliced group's members, or the one kept particle, before appending them to the normalized list",
	"orderedGroupRestrictionMatcher.match":                          "charges each base particle it reads through compareParticleRestriction or ParticleEmptiable",
	"orderedGroupRestrictionMatcher.validateRemainder":              "charges each base particle it reads through ParticleEmptiable",
	"contentModelAnalysis.groupContinuations":                       "stores the start particles that startParticles charged and a shared continuation set in its sets, reading no particle",
	"contentModelAnalysis.ModelContinuations":                       "caches the continuation set continuationsOf charged, reading no particle",
	"contentModelAnalysis.nestedContinuations":                      "caller continuationsOf charges each of the same particles through ParticleContinuations",
	"continuationMerge.copy":                                        "charges the whole set before copying it",
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
		Types:      map[ast.Expr]types.TypeAndValue{},
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
	charged, ok := pkg.Scope().Lookup("chargedParticles").(*types.Func)
	if !ok {
		t.Fatal("internal/schema has no function chargedParticles")
	}

	functions := packageFunctions(files)
	lists := newParticleListFlow(info, particles, charged)
	lists.findEscapingFuncs(files)
	lists.propagate(functions)
	readers := map[string]bool{}
	for _, fn := range functions {
		for _, read := range lists.reads(fn.node) {
			readers[fn.key] = true
			if _, ok := particleListReaders[fn.key]; !ok {
				t.Errorf("%s: %s %s without chargedParticles", fset.Position(read.at), fn.key, read.how)
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(particleListReaders)) {
		if !readers[name] {
			t.Errorf("allowed particle-list reader %s does not exist or reads no particle list", name)
		}
	}
}

// packageFunction is a function declaration, or a package-level variable
// declaration with its initializer and the function literals it contains,
// with the name the allowlist knows it by: the first variable's name for a
// declaration.
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
					if value, ok := spec.(*ast.ValueSpec); ok && len(value.Values) > 0 {
						functions = append(functions, packageFunction{node: value, key: value.Names[0].Name})
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
	switch r := recv.(type) {
	case *ast.IndexExpr:
		recv = r.X
	case *ast.IndexListExpr:
		recv = r.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// resultSlot is one result of a function, by position.
type resultSlot struct {
	fn    *types.Func
	index int
}

// particleListFlow follows a content model's particle list from the
// Particles field, to a fixpoint, into every holder: a parameter it is passed
// to as an argument of a static call within the package, including the
// receiver of a method expression, a variadic parameter it is spread into
// with an ellipsis, and the parameter that a result of a multi-value call
// binds when that call is the only argument; and a variable, struct field or
// function result it is assigned, stored in a composite literal, or returned
// as. A subslice of a
// holder holds the list too. The result of chargedParticles holds none: it is
// the charged accessor. A call through an interface method or a function
// value is not followed, so the caller reads the argument, and a function
// whose results such a call, or another package, may receive reads a list it
// returns.
type particleListFlow struct {
	info      *types.Info
	particles *types.Var
	charged   *types.Func
	holders   map[*types.Var]bool
	results   map[resultSlot]bool
	escaping  map[*types.Func]bool
}

func newParticleListFlow(info *types.Info, particles *types.Var, charged *types.Func) *particleListFlow {
	return &particleListFlow{info: info, particles: particles, charged: charged, holders: map[*types.Var]bool{}, results: map[resultSlot]bool{}, escaping: map[*types.Func]bool{}}
}

// findEscapingFuncs records the functions whose results the flow cannot
// follow to every caller: an exported function, a function referenced other
// than as the callee of a call, and a method whose name an interface of the
// package declares, which a call through that interface may reach. Returning
// a list from one of them is a read.
func (f *particleListFlow) findEscapingFuncs(files []*ast.File) {
	interfaceMethods := map[string]bool{}
	for _, tv := range f.info.Types {
		if iface, ok := tv.Type.Underlying().(*types.Interface); ok {
			for m := range iface.Methods() {
				interfaceMethods[m.Name()] = true
			}
		}
	}
	for _, obj := range f.info.Defs {
		fn, ok := obj.(*types.Func)
		if !ok || fn.Pkg() != f.particles.Pkg() {
			continue
		}
		if fn.Exported() || (fn.Signature().Recv() != nil && interfaceMethods[fn.Name()]) {
			f.escaping[fn.Origin()] = true
		}
	}
	for _, file := range files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if id, ok := n.(*ast.Ident); ok {
				if fn, ok := f.info.Uses[id].(*types.Func); ok && !calledAt(id, stack) {
					f.escaping[fn.Origin()] = true
				}
			}
			stack = append(stack, n)
			return true
		})
	}
}

// calledAt reports whether id, whose ancestors are parents, names the callee
// of a call, directly, as a selector, or instantiated.
func calledAt(id *ast.Ident, parents []ast.Node) bool {
	var expr ast.Expr = id
	for _, parent := range slices.Backward(parents) {
		switch p := parent.(type) {
		case *ast.SelectorExpr:
			if p.Sel != expr {
				return false
			}
			expr = p
		case *ast.IndexExpr:
			if p.X != expr {
				return false
			}
			expr = p
		case *ast.IndexListExpr:
			if p.X != expr {
				return false
			}
			expr = p
		case *ast.ParenExpr:
			expr = p
		case *ast.CallExpr:
			return p.Fun == expr
		default:
			return false
		}
	}
	return false
}

func (f *particleListFlow) propagate(functions []packageFunction) {
	for changed := true; changed; {
		changed = false
		hold := func(v *types.Var) {
			if v != nil && !f.holders[v] {
				f.holders[v] = true
				changed = true
			}
		}
		for _, fn := range functions {
			ast.Inspect(fn.node, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					if inner, ok := f.spreadResults(x); ok {
						params := f.calleeParams(x)
						for i, param := range params {
							if param != nil && inner != nil && f.results[resultSlot{inner, i}] {
								hold(param)
							}
						}
						break
					}
					for i, param := range f.argParams(x) {
						if param != nil && f.carriesList(x.Args[i]) {
							hold(param)
						}
					}
				case *ast.AssignStmt:
					f.assign(x.Lhs, x.Rhs, hold)
				case *ast.ValueSpec:
					f.assign(identExprs(x.Names), x.Values, hold)
				case *ast.CompositeLit:
					f.storeFields(x, hold)
				}
				return true
			})
			decl, ok := fn.node.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			obj, ok := f.info.Defs[decl.Name].(*types.Func)
			if !ok || obj == f.charged {
				continue
			}
			for slot := range f.returnedLists(obj, decl) {
				if !f.results[slot] {
					f.results[slot] = true
					changed = true
				}
			}
		}
	}
}

func identExprs(ids []*ast.Ident) []ast.Expr {
	exprs := make([]ast.Expr, len(ids))
	for i, id := range ids {
		exprs[i] = id
	}
	return exprs
}

// assign records as holders the targets of lhs that receive a list from rhs,
// one value each or the results of one call.
func (f *particleListFlow) assign(lhs, rhs []ast.Expr, hold func(*types.Var)) {
	if len(lhs) == len(rhs) {
		for i := range lhs {
			if f.carriesList(rhs[i]) {
				hold(f.target(lhs[i]))
			}
		}
		return
	}
	if len(rhs) != 1 {
		return
	}
	call, ok := ast.Unparen(rhs[0]).(*ast.CallExpr)
	if !ok {
		return
	}
	fn, _ := f.staticCallee(call)
	for i := range lhs {
		if fn != nil && f.results[resultSlot{fn, i}] {
			hold(f.target(lhs[i]))
		}
	}
}

func (f *particleListFlow) storeFields(lit *ast.CompositeLit, hold func(*types.Var)) {
	tv, ok := f.info.Types[lit]
	if !ok {
		return
	}
	st, ok := tv.Type.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			key, isIdent := kv.Key.(*ast.Ident)
			if !isIdent {
				continue
			}
			if v, isVar := f.info.Uses[key].(*types.Var); isVar && f.carriesList(kv.Value) {
				hold(v.Origin())
			}
			continue
		}
		if i < st.NumFields() && f.carriesList(elt) {
			hold(st.Field(i).Origin())
		}
	}
}

// returnedLists returns the results of fn that a return statement of decl,
// outside the function literals it contains, gives a list, by value or
// through a named result that holds one.
func (f *particleListFlow) returnedLists(fn *types.Func, decl *ast.FuncDecl) map[resultSlot]bool {
	slots := map[resultSlot]bool{}
	results := fn.Signature().Results()
	for i := range results.Len() {
		if f.holders[results.At(i)] {
			slots[resultSlot{fn, i}] = true
		}
	}
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(x.Results) == results.Len() {
				for i, r := range x.Results {
					if f.carriesList(r) {
						slots[resultSlot{fn, i}] = true
					}
				}
				return true
			}
			if len(x.Results) != 1 {
				return true
			}
			call, ok := ast.Unparen(x.Results[0]).(*ast.CallExpr)
			if !ok {
				return true
			}
			if callee, _ := f.staticCallee(call); callee != nil {
				for i := range results.Len() {
					if f.results[resultSlot{callee, i}] {
						slots[resultSlot{fn, i}] = true
					}
				}
			}
		}
		return true
	})
	return slots
}

// target returns the variable or struct field an assignment writes, or nil.
func (f *particleListFlow) target(expr ast.Expr) *types.Var {
	switch x := ast.Unparen(expr).(type) {
	case *ast.Ident:
		if v, ok := f.info.Defs[x].(*types.Var); ok {
			return v.Origin()
		}
		if v, ok := f.info.Uses[x].(*types.Var); ok {
			return v.Origin()
		}
	case *ast.SelectorExpr:
		if selection := f.info.Selections[x]; selection != nil && selection.Kind() == types.FieldVal {
			if v, ok := selection.Obj().(*types.Var); ok {
				return v.Origin()
			}
		}
	}
	return nil
}

// staticCallee returns the function of the package a call statically names,
// and whether it names a method through a method expression. It returns nil
// for a call through an interface method, a function value or a builtin.
func (f *particleListFlow) staticCallee(call *ast.CallExpr) (*types.Func, bool) {
	fun := ast.Unparen(call.Fun)
	switch x := fun.(type) {
	case *ast.IndexExpr:
		fun = x.X
	case *ast.IndexListExpr:
		fun = x.X
	}
	var id *ast.Ident
	methodExpr := false
	switch x := fun.(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
		if selection := f.info.Selections[x]; selection != nil {
			methodExpr = selection.Kind() == types.MethodExpr
		}
	default:
		return nil, false
	}
	fn, ok := f.info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() != f.particles.Pkg() {
		return nil, false
	}
	if recv := fn.Signature().Recv(); recv != nil && types.IsInterface(recv.Type()) {
		return nil, false
	}
	return fn.Origin(), methodExpr
}

// calleeParams returns the parameters of a call's static callee in argument
// order, including the receiver of a method expression, and nil for a call
// staticCallee does not resolve. A variadic parameter is included only when
// the call spreads it with an ellipsis.
func (f *particleListFlow) calleeParams(call *ast.CallExpr) []*types.Var {
	fn, methodExpr := f.staticCallee(call)
	if fn == nil {
		return nil
	}
	sig := fn.Signature()
	var params []*types.Var
	if methodExpr {
		params = append(params, sig.Recv())
	}
	for i := range sig.Params().Len() {
		if sig.Variadic() && i == sig.Params().Len()-1 && !call.Ellipsis.IsValid() {
			break
		}
		params = append(params, sig.Params().At(i))
	}
	return params
}

// spreadResults reports whether call's one argument is a call with several
// results, each bound to a parameter in turn, and returns that inner call's
// static callee, or nil when staticCallee does not resolve it.
func (f *particleListFlow) spreadResults(call *ast.CallExpr) (*types.Func, bool) {
	if len(call.Args) != 1 {
		return nil, false
	}
	inner, ok := ast.Unparen(call.Args[0]).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	tuple, ok := f.info.Types[inner].Type.(*types.Tuple)
	if !ok || tuple.Len() < 2 {
		return nil, false
	}
	fn, _ := f.staticCallee(inner)
	return fn, true
}

// argParams returns, for each argument of a call, the parameter of its static
// callee it binds, or nil for an argument the flow does not follow: one of a
// call staticCallee does not resolve, or one bound to a variadic parameter
// without an ellipsis. A call whose one argument spreads several results
// binds no single parameter; spreadResults covers it.
func (f *particleListFlow) argParams(call *ast.CallExpr) []*types.Var {
	if _, ok := f.spreadResults(call); ok {
		return make([]*types.Var, len(call.Args))
	}
	bound := make([]*types.Var, len(call.Args))
	copy(bound, f.calleeParams(call))
	return bound
}

// passedOn reports whether expr is an argument that the flow follows: one
// argParams binds, or a call spreading several results of which every one
// that holds a list binds a parameter.
func (f *particleListFlow) passedOn(call *ast.CallExpr, expr ast.Expr) bool {
	if inner, ok := f.spreadResults(call); ok {
		if inner == nil || call.Args[0] != expr {
			return false
		}
		params := f.calleeParams(call)
		for i := range inner.Signature().Results().Len() {
			if f.results[resultSlot{inner, i}] && (i >= len(params) || params[i] == nil) {
				return false
			}
		}
		return true
	}
	params := f.argParams(call)
	for i, arg := range call.Args {
		if arg == expr {
			return i < len(params) && params[i] != nil
		}
	}
	return false
}

// carriesList reports whether expr holds a particle list: the Particles
// field, a holder, the first result of a call that returns one, or a subslice
// of any of them.
func (f *particleListFlow) carriesList(expr ast.Expr) bool {
	expr = ast.Unparen(expr)
	if slice, ok := expr.(*ast.SliceExpr); ok {
		expr = ast.Unparen(slice.X)
	}
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		selection := f.info.Selections[x]
		if selection == nil || selection.Kind() != types.FieldVal {
			return false
		}
		v, ok := selection.Obj().(*types.Var)
		return ok && (v == f.particles || f.holders[v.Origin()])
	case *ast.Ident:
		v, ok := f.info.Uses[x].(*types.Var)
		return ok && f.holders[v.Origin()]
	case *ast.CallExpr:
		fn, _ := f.staticCallee(x)
		return fn != nil && f.results[resultSlot{fn, 0}]
	}
	return false
}

// particleListRead is a read of a particle list and how the list reached it.
type particleListRead struct {
	at  token.Pos
	how string
}

// reads returns every read of a particle list in fn and the functions it
// contains. A selector of the Particles field reads it other than as the
// operand of len or indexed by an integer literal. A use of any other holder,
// or a call whose result holds a list, reads it other than as the operand of
// len, a comparison with nil, the range of a loop that takes only indexes,
// indexed by an integer literal, written by an assignment or as a composite
// literal key, passed whole or subsliced as an argument the flow follows, or
// handed to a variable, a struct field, or a result of a function that is not
// escaping.
func (f *particleListFlow) reads(fn ast.Node) []particleListRead {
	var reads []particleListRead
	var stack []ast.Node
	ast.Inspect(fn, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if selection := f.info.Selections[x]; selection != nil && selection.Obj() == f.particles && !f.bounded(x, stack, false) {
				reads = append(reads, particleListRead{x.Pos(), "reads a content model's particle list"})
			}
		case *ast.Ident:
			if v, ok := f.info.Uses[x].(*types.Var); ok && v != f.particles && f.holders[v.Origin()] {
				var expr ast.Expr = x
				parents := stack
				if len(parents) > 0 {
					if sel, ok := parents[len(parents)-1].(*ast.SelectorExpr); ok && sel.Sel == x {
						expr, parents = sel, parents[:len(parents)-1]
					}
				}
				if !written(expr, parents) && !f.handedOn(expr, parents) {
					reads = append(reads, particleListRead{x.Pos(), "reads a particle list held by " + v.Name()})
				}
			}
		case *ast.CallExpr:
			if callee, _ := f.staticCallee(x); callee != nil && f.returnsList(callee) && !f.handedOn(x, stack) {
				reads = append(reads, particleListRead{x.Pos(), "reads a particle list returned by " + callee.Name()})
			}
		}
		stack = append(stack, n)
		return true
	})
	return reads
}

// handedOn reports whether expr, a holder or a call returning a list, whose
// ancestors are parents, is used without reading the list's elements: bounded,
// compared with nil, ranged over for indexes only, or handed to a holder.
func (f *particleListFlow) handedOn(expr ast.Expr, parents []ast.Node) bool {
	if len(parents) == 0 {
		return false
	}
	if f.bounded(expr, parents, true) {
		return true
	}
	switch p := parents[len(parents)-1].(type) {
	case *ast.BinaryExpr:
		other := p.X
		if other == expr {
			other = p.Y
		}
		id, ok := other.(*ast.Ident)
		return (p.Op == token.EQL || p.Op == token.NEQ) && ok && f.info.Uses[id] == types.Universe.Lookup("nil")
	case *ast.RangeStmt:
		return p.X == expr && p.Value == nil
	case *ast.AssignStmt:
		return f.storedBy(p.Lhs, p.Rhs, expr)
	case *ast.ValueSpec:
		return f.storedBy(identExprs(p.Names), p.Values, expr)
	case *ast.KeyValueExpr:
		if p.Value != expr || len(parents) < 2 {
			return false
		}
		lit, ok := parents[len(parents)-2].(*ast.CompositeLit)
		key, isIdent := p.Key.(*ast.Ident)
		_, isField := f.info.Uses[key].(*types.Var)
		return ok && isIdent && isField && f.isStruct(lit)
	case *ast.CompositeLit:
		return f.isStruct(p)
	case *ast.ReturnStmt:
		return f.returnedByDecl(p, parents)
	}
	return false
}

func (f *particleListFlow) isStruct(lit *ast.CompositeLit) bool {
	tv, ok := f.info.Types[lit]
	if !ok {
		return false
	}
	_, ok = tv.Type.Underlying().(*types.Struct)
	return ok
}

// storedBy reports whether expr, one of rhs, is assigned to a target that
// propagate records as a holder.
func (f *particleListFlow) storedBy(lhs, rhs []ast.Expr, expr ast.Expr) bool {
	if len(lhs) == len(rhs) {
		i := slices.Index(rhs, expr)
		return i >= 0 && f.target(lhs[i]) != nil
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(rhs) != 1 || rhs[0] != expr {
		return false
	}
	fn, _ := f.staticCallee(call)
	for i := range lhs {
		if fn == nil || (f.results[resultSlot{fn, i}] && f.target(lhs[i]) == nil) {
			return false
		}
	}
	return true
}

// returnedByDecl reports whether ret returns from a function declaration,
// not a function literal, with one value per result or the results of one
// call, so that propagate records the results it returns.
func (f *particleListFlow) returnedByDecl(ret *ast.ReturnStmt, parents []ast.Node) bool {
	for _, n := range slices.Backward(parents) {
		switch fn := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.FuncDecl:
			obj, ok := f.info.Defs[fn.Name].(*types.Func)
			if !ok || f.escaping[obj.Origin()] {
				return false
			}
			results := obj.Signature().Results().Len()
			if len(ret.Results) == results {
				return true
			}
			_, isCall := ast.Unparen(ret.Results[0]).(*ast.CallExpr)
			return len(ret.Results) == 1 && isCall
		}
	}
	return false
}

func (f *particleListFlow) returnsList(fn *types.Func) bool {
	for i := range fn.Signature().Results().Len() {
		if f.results[resultSlot{fn, i}] {
			return true
		}
	}
	return false
}

// written reports whether expr, whose ancestors are parents, is the target of
// an assignment or the key of a composite literal element.
func written(expr ast.Expr, parents []ast.Node) bool {
	if len(parents) == 0 {
		return false
	}
	switch p := parents[len(parents)-1].(type) {
	case *ast.AssignStmt:
		return slices.Contains(p.Lhs, expr)
	case *ast.KeyValueExpr:
		if p.Key != expr || len(parents) < 2 {
			return false
		}
		_, inLiteral := parents[len(parents)-2].(*ast.CompositeLit)
		return inLiteral
	}
	return false
}

// bounded reports whether expr, whose ancestors are parents, is the operand
// of len, indexed by an integer literal, or, when follow is set, passed whole
// or subsliced as an argument the flow follows.
func (f *particleListFlow) bounded(expr ast.Expr, parents []ast.Node, follow bool) bool {
	if len(parents) == 0 {
		return false
	}
	whole := expr
	parent := parents[len(parents)-1]
	if slice, ok := parent.(*ast.SliceExpr); ok && slice.X == expr && len(parents) > 1 {
		expr, parent = slice, parents[len(parents)-2]
	}
	switch p := parent.(type) {
	case *ast.CallExpr:
		if fun, ok := p.Fun.(*ast.Ident); ok && expr == whole && fun.Name == "len" && f.info.Uses[fun] == types.Universe.Lookup("len") {
			return true
		}
		return follow && f.passedOn(p, expr)
	case *ast.IndexExpr:
		lit, ok := p.Index.(*ast.BasicLit)
		return expr == whole && p.X == whole && ok && lit.Kind == token.INT
	}
	return false
}
