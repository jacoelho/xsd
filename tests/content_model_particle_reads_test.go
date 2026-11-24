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
// when its work grows faster than the charged steps. Nor does it follow a
// list rebuilt particle by particle into another slice, such as a merged
// continuation set, past the function that rebuilds it, which reads the list.
// Storing a container of lists as an element of another container is a read.
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
	"restrictionNormalizer.copyPrefix":                              "charges the whole list before copying its prefix",
	"orderedGroupRestrictionMatcher.match":                          "charges each base particle it reads through compareParticleRestriction or ParticleEmptiable",
	"orderedGroupRestrictionMatcher.validateRemainder":              "charges each base particle it reads through ParticleEmptiable",
	"contentModelAnalysis.ModelContinuations":                       "returns the cached continuation set that continuationsOf charged, reading no particle",
	"contentModelCompiler.checkCandidatesUPA":                       "charges each candidate it reads",
	"contentModelCompiler.wildcardContinuationOverlaps":             "charges each candidate it reads",
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

// holdKind is what a holder holds: a particle list, a container whose
// elements are particle lists, or both.
type holdKind uint8

const (
	holdsList holdKind = 1 << iota
	holdsLists
)

// elementOf returns the kind of a container whose elements are values of
// kind k, or 0 when k is not exactly a list: a container of containers is not
// followed, so storing one is a read.
func elementOf(k holdKind) holdKind {
	if k == holdsList {
		return holdsLists
	}
	return 0
}

// particleListFlow follows a content model's particle list from the
// Particles field, to a fixpoint, into every holder: a parameter it is passed
// to as an argument of a static call within the package, including the
// receiver of a method expression, a variadic parameter it is spread into
// with an ellipsis, and the parameter that a result of a multi-value call
// binds when that call is the only argument; a variable, struct field or
// function result it is assigned, stored in a composite literal, or returned
// as; and a container, an array, slice or map variable, field or result, that
// holds it as an element, stored by an indexed assignment, an append, or an
// array, slice or map literal. A subslice of a holder, an element of a
// container, the value a range loop takes from a container, and a copy made
// by copy or by an append that spreads a holder hold the list too. The result
// of chargedParticles holds none: it is the charged accessor. A call through
// an interface method or a function value is not followed, so the caller
// reads the argument, and a function whose results such a call, or another
// package, may receive reads a list it returns.
type particleListFlow struct {
	info      *types.Info
	particles *types.Var
	charged   *types.Func
	holders   map[*types.Var]holdKind
	results   map[resultSlot]holdKind
	escaping  map[*types.Func]bool
}

func newParticleListFlow(info *types.Info, particles *types.Var, charged *types.Func) *particleListFlow {
	return &particleListFlow{info: info, particles: particles, charged: charged, holders: map[*types.Var]holdKind{}, results: map[resultSlot]holdKind{}, escaping: map[*types.Func]bool{}}
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
		hold := func(v *types.Var, k holdKind) {
			if v != nil && f.holders[v]&k != k {
				f.holders[v] |= k
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
							if param != nil && inner != nil {
								hold(param, f.results[resultSlot{inner, i}])
							}
						}
						break
					}
					for i, param := range f.argParams(x) {
						if param != nil {
							hold(param, f.flowOf(x.Args[i]))
						}
					}
					if f.isBuiltin(x, "copy") && len(x.Args) == 2 {
						hold(f.storeTarget(unslice(x.Args[0]), f.flowOf(x.Args[1])))
					}
				case *ast.RangeStmt:
					if x.Value != nil && f.flowOf(x.X) == holdsLists {
						hold(f.target(x.Value), holdsList)
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
			for slot, k := range f.returnedLists(obj, decl) {
				if f.results[slot]&k != k {
					f.results[slot] |= k
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

// assign records as holders the targets of lhs that receive a list or a
// container from rhs: one value each, the value of a comma-ok index of a
// container, or the results of one call.
func (f *particleListFlow) assign(lhs, rhs []ast.Expr, hold func(*types.Var, holdKind)) {
	if len(lhs) == len(rhs) {
		for i := range lhs {
			hold(f.storeTarget(lhs[i], f.flowOf(rhs[i])))
		}
		return
	}
	if len(rhs) != 1 {
		return
	}
	switch x := ast.Unparen(rhs[0]).(type) {
	case *ast.IndexExpr:
		hold(f.storeTarget(lhs[0], f.flowOf(x)))
	case *ast.CallExpr:
		if fn, _ := f.staticCallee(x); fn != nil {
			for i := range lhs {
				hold(f.storeTarget(lhs[i], f.results[resultSlot{fn, i}]))
			}
		}
	}
}

func (f *particleListFlow) storeFields(lit *ast.CompositeLit, hold func(*types.Var, holdKind)) {
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
			if v, isVar := f.info.Uses[key].(*types.Var); isVar {
				hold(v.Origin(), f.flowOf(kv.Value))
			}
			continue
		}
		if i < st.NumFields() {
			hold(st.Field(i).Origin(), f.flowOf(elt))
		}
	}
}

// returnedLists returns what each result of fn holds that a return statement
// of decl, outside the function literals it contains, gives it, by value or
// through a named result.
func (f *particleListFlow) returnedLists(fn *types.Func, decl *ast.FuncDecl) map[resultSlot]holdKind {
	slots := map[resultSlot]holdKind{}
	results := fn.Signature().Results()
	for i := range results.Len() {
		slots[resultSlot{fn, i}] |= f.holders[results.At(i)]
	}
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(x.Results) == results.Len() {
				for i, r := range x.Results {
					slots[resultSlot{fn, i}] |= f.flowOf(r)
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
					slots[resultSlot{fn, i}] |= f.results[resultSlot{callee, i}]
				}
			}
		}
		return true
	})
	return slots
}

// storeTarget returns the holder, and its kind, that writing a value of kind
// k to expr records: the variable or field expr names, or the container expr
// indexes. It returns nil for a write the flow does not follow.
func (f *particleListFlow) storeTarget(expr ast.Expr, k holdKind) (*types.Var, holdKind) {
	if k == 0 {
		return nil, 0
	}
	if index, ok := ast.Unparen(expr).(*ast.IndexExpr); ok {
		if elementOf(k) == 0 {
			return nil, 0
		}
		return f.target(index.X), elementOf(k)
	}
	return f.target(expr), k
}

func unslice(expr ast.Expr) ast.Expr {
	if slice, ok := ast.Unparen(expr).(*ast.SliceExpr); ok {
		return slice.X
	}
	return expr
}

func (f *particleListFlow) isBuiltin(call *ast.CallExpr, name string) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && f.info.Uses[id] == types.Universe.Lookup(name)
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
			if f.results[resultSlot{inner, i}] != 0 && (i >= len(params) || params[i] == nil) {
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

// flowOf returns what expr holds: the Particles field, a holder, or the first
// result of a call that returns one, as they hold; a subslice as its operand
// holds; a list for an element of a container; the union of its operands for
// an append, each element appended counting as a container of lists; and a
// container of lists for an array, slice or map literal with a list among its
// elements.
func (f *particleListFlow) flowOf(expr ast.Expr) holdKind {
	switch x := ast.Unparen(expr).(type) {
	case *ast.SliceExpr:
		return f.flowOf(x.X)
	case *ast.IndexExpr:
		if f.flowOf(x.X) == holdsLists {
			return holdsList
		}
	case *ast.SelectorExpr:
		selection := f.info.Selections[x]
		if selection == nil || selection.Kind() != types.FieldVal {
			return 0
		}
		v, ok := selection.Obj().(*types.Var)
		switch {
		case !ok:
			return 0
		case v == f.particles:
			return holdsList
		}
		return f.holders[v.Origin()]
	case *ast.Ident:
		if v, ok := f.info.Uses[x].(*types.Var); ok {
			return f.holders[v.Origin()]
		}
	case *ast.CallExpr:
		if f.isBuiltin(x, "append") && len(x.Args) > 0 {
			k := f.flowOf(x.Args[0])
			if x.Ellipsis.IsValid() {
				return k | f.flowOf(x.Args[len(x.Args)-1])
			}
			for _, arg := range x.Args[1:] {
				k |= elementOf(f.flowOf(arg))
			}
			return k
		}
		if fn, _ := f.staticCallee(x); fn != nil {
			return f.results[resultSlot{fn, 0}]
		}
	case *ast.CompositeLit:
		if f.isStruct(x) {
			return 0
		}
		var k holdKind
		for _, elt := range x.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				elt = kv.Value
			}
			k |= elementOf(f.flowOf(elt))
		}
		return k
	}
	return 0
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
// literal key, passed whole or subsliced as an argument the flow follows,
// handed to a variable, a struct field, a container, or a result of a
// function that is not escaping, or copied into a holder by copy or append. A
// container is read only through an element that is read, and its range loop
// reads nothing when the value it takes is a variable.
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
			if v, ok := f.info.Uses[x].(*types.Var); ok && v != f.particles && f.holders[v.Origin()] != 0 {
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
// compared with nil, ranged over for indexes only, or handed to a holder,
// subsliced, indexed as a container, or copied, in a way propagate records.
func (f *particleListFlow) handedOn(expr ast.Expr, parents []ast.Node) bool {
	if len(parents) == 0 {
		return false
	}
	if f.bounded(expr, parents, true) {
		return true
	}
	outer := parents[:len(parents)-1]
	switch p := parents[len(parents)-1].(type) {
	case *ast.SliceExpr:
		return p.X == expr && f.handedOn(p, outer)
	case *ast.IndexExpr:
		return p.X == expr && f.flowOf(expr) == holdsLists && (written(p, outer) || f.handedOn(p, outer))
	case *ast.CallExpr:
		return f.copiedBy(p, expr, outer)
	case *ast.BinaryExpr:
		other := p.X
		if other == expr {
			other = p.Y
		}
		id, ok := other.(*ast.Ident)
		return (p.Op == token.EQL || p.Op == token.NEQ) && ok && f.info.Uses[id] == types.Universe.Lookup("nil")
	case *ast.RangeStmt:
		return p.X == expr && (p.Value == nil || (f.flowOf(expr) == holdsLists && f.target(p.Value) != nil))
	case *ast.AssignStmt:
		return f.storedBy(p.Lhs, p.Rhs, expr)
	case *ast.ValueSpec:
		return f.storedBy(identExprs(p.Names), p.Values, expr)
	case *ast.KeyValueExpr:
		if p.Value != expr || len(outer) == 0 {
			return false
		}
		lit, ok := outer[len(outer)-1].(*ast.CompositeLit)
		if ok && !f.isStruct(lit) {
			return f.elementOfLiteral(lit, expr, outer[:len(outer)-1])
		}
		key, isIdent := p.Key.(*ast.Ident)
		_, isField := f.info.Uses[key].(*types.Var)
		return ok && isIdent && isField
	case *ast.CompositeLit:
		return f.isStruct(p) || f.elementOfLiteral(p, expr, outer)
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

// copiedBy reports whether expr, an argument of call, is copied by it into a
// holder: the operand or a spread or appended element of an append whose
// result is handed on, or the source of a copy into a holder.
func (f *particleListFlow) copiedBy(call *ast.CallExpr, expr ast.Expr, outer []ast.Node) bool {
	switch {
	case f.isBuiltin(call, "append") && len(call.Args) > 0:
		last := len(call.Args) - 1
		switch {
		case call.Args[0] == expr, call.Ellipsis.IsValid() && call.Args[last] == expr:
		case !call.Ellipsis.IsValid() && slices.Contains(call.Args[1:], expr) && elementOf(f.flowOf(expr)) != 0:
		default:
			return false
		}
		return f.handedOn(call, outer)
	case f.isBuiltin(call, "copy") && len(call.Args) == 2 && call.Args[1] == expr:
		target, _ := f.storeTarget(unslice(call.Args[0]), f.flowOf(expr))
		return target != nil
	}
	return false
}

// elementOfLiteral reports whether expr, an element of the array, slice or
// map literal lit, is a list the literal holds as a container that is handed
// on.
func (f *particleListFlow) elementOfLiteral(lit *ast.CompositeLit, expr ast.Expr, outer []ast.Node) bool {
	return elementOf(f.flowOf(expr)) != 0 && f.handedOn(lit, outer)
}

// storedBy reports whether expr, one of rhs, is assigned to a target that
// propagate records as a holder.
func (f *particleListFlow) storedBy(lhs, rhs []ast.Expr, expr ast.Expr) bool {
	if len(lhs) == len(rhs) {
		i := slices.Index(rhs, expr)
		if i < 0 {
			return false
		}
		target, _ := f.storeTarget(lhs[i], f.flowOf(expr))
		return target != nil
	}
	if len(rhs) != 1 || rhs[0] != expr {
		return false
	}
	switch x := expr.(type) {
	case *ast.IndexExpr:
		target, _ := f.storeTarget(lhs[0], f.flowOf(x))
		return target != nil
	case *ast.CallExpr:
		fn, _ := f.staticCallee(x)
		if fn == nil {
			return false
		}
		for i := range lhs {
			if target, _ := f.storeTarget(lhs[i], f.results[resultSlot{fn, i}]); f.results[resultSlot{fn, i}] != 0 && target == nil {
				return false
			}
		}
		return true
	}
	return false
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
		if f.results[resultSlot{fn, i}] != 0 {
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
// of len, a list indexed by an integer literal, or, when follow is set, passed whole
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
		return expr == whole && p.X == whole && ok && lit.Kind == token.INT && f.flowOf(whole)&holdsLists == 0
	}
	return false
}
