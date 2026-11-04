package tests_test

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"slices"
)

type borrowViolation struct {
	pos  token.Pos
	kind string
}

type borrowNode struct {
	borrowed bool
	next     []int
}

type borrowExpression struct {
	expr   ast.Expr
	result int
}

type borrowSink struct {
	node      int
	violation borrowViolation
}

// The graph is deliberately flow-insensitive: once a local can borrow, later
// assignments cannot erase that fact. Each node enters the worklist once.
// Synchronous calls are reviewed non-retaining boundaries, not interprocedural
// proofs; this guard also does not model Reader.Next/Reset/Detach invalidation.
type borrowGraph struct {
	info        *types.Info
	nodes       []borrowNode
	expressions map[borrowExpression]int
	objects     map[types.Object]int
	sinks       []borrowSink
	violations  []borrowViolation
}

func analyzeXMLStreamPackage(fset *token.FileSet, path string, files []*ast.File) ([]borrowViolation, error) {
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection), Types: make(map[ast.Expr]types.TypeAndValue),
		Implicits: make(map[ast.Node]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check(path, fset, files, info); err != nil {
		return nil, err
	}
	graph := borrowGraph{info: info, expressions: make(map[borrowExpression]int), objects: make(map[types.Object]int)}
	for _, file := range files {
		graph.file(file)
	}
	return graph.solve(), nil
}

func (g *borrowGraph) newNode(typ types.Type) int {
	id := len(g.nodes)
	g.nodes = append(g.nodes, borrowNode{borrowed: borrowedXMLStreamType(typ)})
	return id
}

func (g *borrowGraph) object(object types.Object) int {
	if id, ok := g.objects[object]; ok {
		return id
	}
	var typ types.Type
	if object != nil {
		typ = object.Type()
	}
	id := g.newNode(typ)
	g.objects[object] = id
	return id
}

func (g *borrowGraph) edge(from, to int) { g.nodes[from].next = append(g.nodes[from].next, to) }
func (g *borrowGraph) sink(node int, pos token.Pos, kind string) {
	g.sinks = append(g.sinks, borrowSink{node, borrowViolation{pos, kind}})
}
func (g *borrowGraph) structural(pos token.Pos, kind string, typ types.Type) {
	if borrowedXMLStreamType(typ) {
		g.violations = append(g.violations, borrowViolation{pos, kind})
	}
}

func (g *borrowGraph) file(file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.StructType:
			for _, field := range node.Fields.List {
				g.structural(field.Pos(), "field type", g.info.TypeOf(field.Type))
			}
		case *ast.TypeSpec:
			g.structural(node.Pos(), "type alias", g.info.TypeOf(node.Type))
		case *ast.FuncType:
			if node.Results != nil {
				for _, field := range node.Results.List {
					g.structural(field.Pos(), "result type", g.info.TypeOf(field.Type))
					for _, name := range field.Names {
						g.sink(g.object(g.info.Defs[name]), name.Pos(), "return")
					}
				}
			}
		case *ast.ValueSpec:
			for i, name := range node.Names {
				object := g.info.Defs[name]
				if isPackageVariable(object) {
					g.structural(name.Pos(), "global type", object.Type())
				}
				if len(node.Values) != 0 {
					g.assign(name, g.value(node.Values, i))
				}
			}
		case *ast.AssignStmt:
			for i, destination := range node.Lhs {
				g.assign(destination, g.value(node.Rhs, i))
			}
		case *ast.IncDecStmt:
			if index, ok := ast.Unparen(node.X).(*ast.IndexExpr); ok {
				g.indexKey(index)
			}
		case *ast.RangeStmt:
			for _, destination := range []ast.Expr{node.Key, node.Value} {
				if destination != nil && carriesBorrowedReference(g.info.TypeOf(destination)) {
					g.assign(destination, g.expression(node.X, 0))
				}
			}
		case *ast.CallExpr:
			g.mutatingCall(node)
		case *ast.TypeSwitchStmt:
			assignment, ok := node.Assign.(*ast.AssignStmt)
			if ok {
				if assertion, ok := assignment.Rhs[0].(*ast.TypeAssertExpr); ok {
					for _, statement := range node.Body.List {
						if object := g.info.Implicits[statement]; object != nil && carriesBorrowedReference(object.Type()) {
							g.edge(g.expression(assertion.X, 0), g.object(object))
						}
					}
				}
			}
		case *ast.ReturnStmt:
			for _, result := range node.Results {
				count := 1
				if tuple, ok := g.info.TypeOf(result).(*types.Tuple); ok {
					count = tuple.Len()
				}
				for i := range count {
					g.sink(g.expression(result, i), result.Pos(), "return")
				}
			}
		case *ast.SendStmt:
			g.sink(g.expression(node.Value, 0), node.Value.Pos(), "channel send")
		case *ast.GoStmt:
			g.delayed(node.Call)
		case *ast.DeferStmt:
			g.delayed(node.Call)
		}
		return true
	})
}

// append can mutate existing backing even when its result stays local or is
// discarded. Treat that write like an indexed store; an allocation in the
// destination expression has no preexisting alias, so only its result can escape.
func (g *borrowGraph) mutatingCall(call *ast.CallExpr) {
	builtin, ok := borrowCalledObject(g.info, call.Fun).(*types.Builtin)
	if !ok || (builtin.Name() != "copy" && builtin.Name() != "append") || isByteSlice(g.info.TypeOf(call.Args[0])) {
		return
	}
	if builtin.Name() == "append" && g.freshSlice(call.Args[0]) {
		return
	}
	for _, source := range call.Args[1:] {
		g.sink(g.expression(source, 0), call.Args[0].Pos(), "indexed store")
	}
}

func (g *borrowGraph) freshSlice(expression ast.Expr) bool {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.CompositeLit:
		return true
	case *ast.Ident:
		_, isNil := g.info.ObjectOf(expression).(*types.Nil)
		return isNil
	case *ast.CallExpr:
		if g.info.Types[expression.Fun].IsType() {
			return len(expression.Args) == 1 && g.freshSlice(expression.Args[0])
		}
		if builtin, ok := borrowCalledObject(g.info, expression.Fun).(*types.Builtin); ok {
			return builtin.Name() == "make"
		}
	}
	return false
}

func (g *borrowGraph) delayed(call *ast.CallExpr) {
	g.sink(g.expression(call.Fun, 0), call.Fun.Pos(), "delayed call")
	for _, argument := range call.Args {
		g.sink(g.expression(argument, 0), argument.Pos(), "delayed call")
	}
}

func (g *borrowGraph) value(values []ast.Expr, index int) int {
	if len(values) == 1 {
		return g.expression(values[0], index)
	}
	return g.expression(values[index], 0)
}

func (g *borrowGraph) assign(destination ast.Expr, value int) {
	destination = ast.Unparen(destination)
	if name, ok := destination.(*ast.Ident); ok {
		if name.Name == "_" {
			return
		}
		object := g.info.ObjectOf(name)
		g.edge(value, g.object(object))
		if isPackageVariable(object) {
			g.sink(value, name.Pos(), "global store")
		}
		return
	}
	kind := "indirect store"
	switch destination := destination.(type) {
	case *ast.SelectorExpr:
		kind = "field store"
	case *ast.IndexExpr:
		kind = "indexed store"
		g.indexKey(destination)
	}
	g.sink(value, destination.Pos(), kind)
}

// indexKey sinks an index operand that a store retains. Map keys outlive the
// statement; slice and array indexes are integers, which carry no reference, so
// the sink needs no map-type gate, which would miss type-parameter maps.
// The sink shares the destination position so the value sink deduplicates.
func (g *borrowGraph) indexKey(index *ast.IndexExpr) {
	g.sink(g.expression(index.Index, 0), index.Pos(), "indexed store")
}

func isPackageVariable(object types.Object) bool {
	variable, ok := object.(*types.Var)
	return ok && variable.Pkg() != nil && variable.Parent() == variable.Pkg().Scope()
}

func (g *borrowGraph) expression(expression ast.Expr, result int) int {
	key := borrowExpression{expression, result}
	if id, ok := g.expressions[key]; ok {
		return id
	}
	typ := g.info.TypeOf(expression)
	if tuple, ok := typ.(*types.Tuple); ok {
		typ = tuple.At(result).Type()
	} else if result != 0 {
		// The second result of comma-ok assertions, indexes, and receives is scalar.
		typ = types.Typ[types.Bool]
	}
	id := g.newNode(typ)
	g.expressions[key] = id
	if !carriesBorrowedReference(typ) {
		return id
	}
	link := func(expr ast.Expr) { g.edge(g.expression(expr, 0), id) }
	switch expression := expression.(type) {
	case *ast.Ident:
		g.edge(g.object(g.info.ObjectOf(expression)), id)
	case *ast.ParenExpr:
		g.edge(g.expression(expression.X, result), id)
	case *ast.SelectorExpr:
		// Field selection copies names/scalars, but retains reference-bearing fields;
		// a method value also captures its receiver.
		if g.info.Selections[expression] != nil {
			link(expression.X)
		}
	case *ast.IndexExpr:
		link(expression.X)
	case *ast.SliceExpr:
		link(expression.X)
	case *ast.StarExpr:
		link(expression.X)
	case *ast.UnaryExpr:
		link(expression.X)
		if expression.Op == token.AND {
			// Taking an address retains the containing storage even when reading
			// the addressed scalar or name would copy an independent value.
			link(borrowAddressBase(expression.X))
		}
	case *ast.TypeAssertExpr:
		link(expression.X)
	case *ast.CompositeLit:
		for _, element := range expression.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				if _, isMap := g.info.TypeOf(expression).Underlying().(*types.Map); isMap {
					link(pair.Key)
				}
				link(pair.Value)
			} else {
				link(element)
			}
		}
	case *ast.FuncLit:
		ast.Inspect(expression.Body, func(node ast.Node) bool {
			name, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			object, ok := g.info.Uses[name].(*types.Var)
			if ok && (object.Pos() < expression.Pos() || object.Pos() >= expression.End()) {
				g.edge(g.object(object), id)
			}
			return true
		})
	case *ast.CallExpr:
		g.call(expression, result, id)
	}
	return id
}

func borrowAddressBase(expression ast.Expr) ast.Expr {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.SelectorExpr:
		return borrowAddressBase(expression.X)
	case *ast.IndexExpr:
		return borrowAddressBase(expression.X)
	case *ast.StarExpr:
		return expression.X
	}
	return expression
}

func (g *borrowGraph) call(call *ast.CallExpr, result, id int) {
	link := func(expr ast.Expr) { g.edge(g.expression(expr, 0), id) }
	if g.info.Types[call.Fun].IsType() {
		for _, argument := range call.Args {
			link(argument)
		}
		return
	}
	function := borrowCalledObject(g.info, call.Fun)
	if builtin, ok := function.(*types.Builtin); ok {
		if builtin.Name() == "append" {
			link(call.Args[0])
			if !isByteSlice(g.info.TypeOf(call)) {
				for _, argument := range call.Args[1:] {
					link(argument)
				}
			}
		}
		return
	}
	if fn, ok := function.(*types.Func); ok {
		if fn.Pkg() != nil && fn.Pkg().Path() == xmlStreamImport && fn.Name() == "RawValue" {
			signature, ok := fn.Type().(*types.Signature)
			if ok && signature.Recv() != nil && isXMLStreamAttr(signature.Recv().Type()) && result == 0 {
				g.nodes[id].borrowed = true
			}
			return
		}
		if fn.Pkg() != nil && fn.Name() == "Clone" && (fn.Pkg().Path() == "bytes" || fn.Pkg().Path() == "slices") {
			if !isByteSlice(g.info.TypeOf(call)) {
				link(call.Args[0])
			}
			return
		}
	}
	// Calling a captured method or closure may return its captured reference.
	// Ordinary helpers remain synchronous, reviewed non-retaining boundaries.
	link(call.Fun)
}

func borrowCalledObject(info *types.Info, expression ast.Expr) types.Object {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.Ident:
		return info.ObjectOf(expression)
	case *ast.SelectorExpr:
		return info.ObjectOf(expression.Sel)
	case *ast.IndexExpr:
		return borrowCalledObject(info, expression.X)
	case *ast.IndexListExpr:
		return borrowCalledObject(info, expression.X)
	}
	return nil
}

func isXMLStreamAttr(typ types.Type) bool {
	if pointer, ok := types.Unalias(typ).(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := types.Unalias(typ).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == xmlStreamImport && named.Obj().Name() == "Attr"
}

func isByteSlice(typ types.Type) bool {
	slice, ok := typ.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	basic, ok := slice.Elem().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Byte
}

func carriesBorrowedReference(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch typ := typ.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		return true
	case *types.Array:
		return carriesBorrowedReference(typ.Elem())
	case *types.Struct:
		for field := range typ.Fields() {
			if carriesBorrowedReference(field.Type()) {
				return true
			}
		}
	}
	return false
}

func (g *borrowGraph) solve() []borrowViolation {
	queue := make([]int, 0, len(g.nodes))
	for id, node := range g.nodes {
		if node.borrowed {
			queue = append(queue, id)
		}
	}
	for next := 0; next < len(queue); next++ {
		for _, target := range g.nodes[queue[next]].next {
			if !g.nodes[target].borrowed {
				g.nodes[target].borrowed = true
				queue = append(queue, target)
			}
		}
	}
	for _, sink := range g.sinks {
		if g.nodes[sink.node].borrowed {
			g.violations = append(g.violations, sink.violation)
		}
	}
	slices.SortFunc(g.violations, func(a, b borrowViolation) int {
		if a.pos != b.pos {
			return int(a.pos - b.pos)
		}
		if a.kind < b.kind {
			return -1
		}
		if a.kind > b.kind {
			return 1
		}
		return 0
	})
	return slices.Compact(g.violations)
}
