package tests_test

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestStreamBorrowedAttributeFieldsStayBehindAccessors(t *testing.T) {
	checkStreamBoundaryHost(t, repoRoot(t))
}

func checkStreamBoundaryHost(t *testing.T, root string) {
	t.Helper()
	fset := token.NewFileSet()
	packages := make(map[string]*streamBoundaryPackage)
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		slashFile := filepath.ToSlash(rel)
		if entry.IsDir() {
			switch slashFile {
			case ".git", ".codex", "docs/spec", "tests/corpus":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(file) != ".go" || strings.HasPrefix(slashFile, "internal/stream/") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return err
		}
		dir := filepath.ToSlash(filepath.Dir(slashFile))
		key := dir + "\x00" + parsed.Name.Name
		pkg := packages[key]
		if pkg == nil {
			pkg = &streamBoundaryPackage{dir: dir, name: parsed.Name.Name}
			packages[key] = pkg
		}
		match, err := build.Default.MatchFile(filepath.Dir(file), filepath.Base(file))
		if err != nil {
			return err
		}
		if !match {
			pkg.excluded = append(pkg.excluded, slashFile)
			if fileImportsStreamPackage(parsed) {
				t.Fatalf("%s imports internal/stream but is excluded from the host type-check; add target-aware boundary checking", slashFile)
			}
			return nil
		}
		pkg.files = append(pkg.files, parsed)
		return nil
	})
	if err != nil {
		t.Fatalf("walk Go files: %v", err)
	}
	for _, pkg := range packages {
		if len(pkg.excluded) != 0 && importsStreamPackage(pkg) {
			t.Fatalf("package %s imports internal/stream but has files excluded from the host type-check: %s", pkg.dir, strings.Join(pkg.excluded, ", "))
		}
		if strings.HasSuffix(pkg.name, "_test") {
			if importsStreamPackage(pkg) {
				t.Fatalf("external test package %s imports internal/stream but cannot be checked with its test-augmented dependency", pkg.dir)
			}
			continue
		}
		checkStreamBoundaryPackage(t, fset, pkg)
	}
}

type streamBoundaryPackage struct {
	dir      string
	name     string
	files    []*ast.File
	excluded []string
}

func importsStreamPackage(pkg *streamBoundaryPackage) bool {
	return slices.ContainsFunc(pkg.files, fileImportsStreamPackage)
}

func fileImportsStreamPackage(file *ast.File) bool {
	const streamPath = "github.com/jacoelho/xsd/internal/stream"
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err == nil && path == streamPath {
			return true
		}
	}
	return false
}

func checkStreamBoundaryPackage(t *testing.T, fset *token.FileSet, pkg *streamBoundaryPackage) {
	t.Helper()
	info := &types.Info{
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
	}
	conf := types.Config{
		Importer: importer.ForCompiler(fset, "source", nil),
	}
	path := "github.com/jacoelho/xsd"
	if pkg.dir != "." {
		path += "/" + pkg.dir
	}
	if strings.HasSuffix(pkg.name, "_test") {
		path += "_test"
	}
	if _, err := conf.Check(path, fset, pkg.files, info); err != nil {
		t.Fatalf("type-check %s: %v", path, err)
	}
	for _, file := range pkg.files {
		checkStreamBoundaryFile(t, fset, info, file)
	}
}

func checkStreamBoundaryFile(t *testing.T, fset *token.FileSet, info *types.Info, file *ast.File) {
	t.Helper()
	parents := astParentMap(file)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			checkParserInvalidationSites(t, fset, info, n)
		case *ast.FuncLit:
			checkParserInvalidationBody(t, fset, info, n.Body)
		case *ast.SelectorExpr:
			checkStreamBoundarySelector(t, fset, info, parents, n)
		case *ast.ValueSpec:
			if n.Type != nil {
				checkRetainedStreamType(t, fset, n.Type.Pos(), info.TypeOf(n.Type), "declares retained")
			} else {
				for _, name := range n.Names {
					if object := info.Defs[name]; object != nil {
						checkRetainedStreamType(t, fset, name.Pos(), object.Type(), "declares retained")
					}
				}
			}
			checkBorrowedValueSpec(t, fset, info, n)
		case *ast.Field:
			if isFuncParam(n, parents) {
				return true
			}
			if isOwnedStreamConstructorResult(info, n, parents) {
				return true
			}
			checkRetainedStreamType(t, fset, n.Type.Pos(), info.TypeOf(n.Type), "stores")
		case *ast.TypeSpec:
			checkRetainedStreamType(t, fset, n.Type.Pos(), info.TypeOf(n.Type), "aliases")
		case *ast.CompositeLit:
			pos := n.Pos()
			if n.Type != nil {
				pos = n.Type.Pos()
			}
			checkRetainedStreamType(t, fset, pos, info.TypeOf(n.Type), "constructs retained")
			checkBorrowedCompositeValues(t, fset, info, n)
		case *ast.AssignStmt:
			checkBorrowedAssignments(t, fset, info, n)
		case *ast.CallExpr:
			checkBorrowedCallArguments(t, fset, info, n)
		case *ast.SendStmt:
			if name, ok := borrowedRetentionExpr(info, n.Value); ok {
				t.Fatalf("%s sends borrowed stream value %s to a channel", fset.Position(n.Value.Pos()), name)
			}
		case *ast.GoStmt:
			checkDelayedBorrowedCall(t, fset, info, n.Call, "starts goroutine with")
		case *ast.DeferStmt:
			checkDelayedBorrowedCall(t, fset, info, n.Call, "defers")
		case *ast.ReturnStmt:
			for _, result := range n.Results {
				if name, ok := borrowedRetentionExpr(info, result); ok {
					t.Fatalf("%s returns borrowed stream value %s", fset.Position(result.Pos()), name)
				}
			}
		}
		return true
	})
}

func checkRetainedStreamType(t *testing.T, fset *token.FileSet, pos token.Pos, typ types.Type, action string) {
	t.Helper()
	if name, ok := streamBorrowedType(typ); ok {
		t.Fatalf("%s %s borrowed stream type %s", fset.Position(pos), action, name)
	}
}

func streamBorrowedType(typ types.Type) (string, bool) {
	seen := make(map[types.Type]bool)
	var visit func(types.Type) (string, bool)
	visit = func(typ types.Type) (string, bool) {
		if typ == nil {
			return "", false
		}
		typ = types.Unalias(typ)
		if seen[typ] {
			return "", false
		}
		seen[typ] = true
		switch typ := typ.(type) {
		case *types.Named:
			obj := typ.Obj()
			if obj.Pkg() != nil && obj.Pkg().Path() == "github.com/jacoelho/xsd/internal/stream" {
				switch obj.Name() {
				case "Token", "StartElement", "Attr":
					return "stream." + obj.Name(), true
				}
			}
			if arguments := typ.TypeArgs(); arguments != nil {
				for argument := range arguments.Types() {
					if name, ok := visit(argument); ok {
						return name, true
					}
				}
			}
			return visit(typ.Underlying())
		case *types.Pointer:
			return visit(typ.Elem())
		case *types.Array:
			return visit(typ.Elem())
		case *types.Slice:
			return visit(typ.Elem())
		case *types.Map:
			if name, ok := visit(typ.Key()); ok {
				return name, true
			}
			return visit(typ.Elem())
		case *types.Chan:
			return visit(typ.Elem())
		case *types.TypeParam:
			return visit(typ.Constraint())
		case *types.Interface:
			typ = typ.Complete()
			for embedded := range typ.EmbeddedTypes() {
				if name, ok := visit(embedded); ok {
					return name, true
				}
			}
			return "", false
		case *types.Union:
			for term := range typ.Terms() {
				if name, ok := visit(term.Type()); ok {
					return name, true
				}
			}
			return "", false
		default:
			return "", false
		}
	}
	return visit(typ)
}

func checkBorrowedValueSpec(t *testing.T, fset *token.FileSet, info *types.Info, spec *ast.ValueSpec) {
	t.Helper()
	for i, value := range spec.Values {
		if i >= len(spec.Names) {
			return
		}
		object := info.Defs[spec.Names[i]]
		if object == nil || !typeCanHideBorrowed(object.Type()) {
			continue
		}
		if name, ok := borrowedRetentionExpr(info, value); ok {
			t.Fatalf("%s initializes retained value with borrowed stream value %s", fset.Position(value.Pos()), name)
		}
	}
}

func checkBorrowedAssignments(t *testing.T, fset *token.FileSet, info *types.Info, assignment *ast.AssignStmt) {
	t.Helper()
	for _, destination := range assignment.Lhs {
		index, ok := destination.(*ast.IndexExpr)
		if !ok {
			continue
		}
		if name, borrowed := borrowedRetentionExpr(info, index.Index); borrowed {
			t.Fatalf("%s retains borrowed stream value %s as a map key", fset.Position(index.Index.Pos()), name)
		}
	}
	if len(assignment.Rhs) == 1 && len(assignment.Lhs) > 1 {
		value, borrowed := borrowedRetentionValue(info, assignment.Rhs[0])
		tuple, tupleOK := types.Unalias(info.TypeOf(assignment.Rhs[0])).(*types.Tuple)
		if !borrowed || !tupleOK || tuple.Len() != len(assignment.Lhs) {
			return
		}
		for i, destination := range assignment.Lhs {
			if !assignmentDestinationRetains(info, destination) || !typeCanRetainBorrowedType(tuple.At(i).Type(), value.typ) {
				continue
			}
			t.Fatalf("%s assigns borrowed stream value %s from a multi-valued result", fset.Position(assignment.Rhs[0].Pos()), value.name)
		}
		return
	}
	if len(assignment.Lhs) != len(assignment.Rhs) {
		return
	}
	for i, value := range assignment.Rhs {
		if name, ok := borrowedRetentionExpr(info, value); ok && assignmentDestinationRetains(info, assignment.Lhs[i]) {
			t.Fatalf("%s assigns borrowed stream value %s to a retaining type", fset.Position(value.Pos()), name)
		}
	}
}

func assignmentDestinationRetains(info *types.Info, destination ast.Expr) bool {
	switch destination.(type) {
	case *ast.IndexExpr, *ast.SelectorExpr, *ast.StarExpr:
		return true
	default:
		return typeCanHideBorrowed(info.TypeOf(destination))
	}
}

func checkBorrowedCompositeValues(t *testing.T, fset *token.FileSet, info *types.Info, literal *ast.CompositeLit) {
	t.Helper()
	if !typeCanHideBorrowed(info.TypeOf(literal)) {
		return
	}
	_, isMap := types.Unalias(info.TypeOf(literal)).(*types.Map)
	for _, element := range literal.Elts {
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			if isMap {
				if name, borrowed := borrowedRetentionExpr(info, pair.Key); borrowed {
					t.Fatalf("%s retains borrowed stream value %s as a map key", fset.Position(pair.Key.Pos()), name)
				}
			}
			element = pair.Value
		}
		if name, ok := borrowedRetentionExpr(info, element); ok {
			t.Fatalf("%s stores borrowed stream value %s in a retaining composite", fset.Position(element.Pos()), name)
		}
	}
}

func checkBorrowedCallArguments(t *testing.T, fset *token.FileSet, info *types.Info, call *ast.CallExpr) {
	t.Helper()
	if builtinCallName(info, call) == "append" {
		for _, argument := range call.Args[1:] {
			if name, borrowed := borrowedRetentionExpr(info, argument); borrowed {
				t.Fatalf("%s appends borrowed stream value %s", fset.Position(argument.Pos()), name)
			}
		}
	}
	if builtinCallName(info, call) == "copy" {
		for _, argument := range call.Args {
			if name, borrowed := borrowedRetentionExpr(info, argument); borrowed {
				t.Fatalf("%s copies borrowed stream value %s", fset.Position(argument.Pos()), name)
			}
		}
	}
	for i, argument := range call.Args {
		parameter, ok := callParameterType(info, call, i)
		if !ok || !typeCanHideBorrowed(parameter) {
			continue
		}
		if name, borrowed := borrowedRetentionExpr(info, argument); borrowed {
			t.Fatalf("%s passes borrowed stream value %s through a retaining parameter", fset.Position(argument.Pos()), name)
		}
	}
}

func builtinCallName(info *types.Info, call *ast.CallExpr) string {
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok {
		return ""
	}
	builtin, ok := info.Uses[identifier].(*types.Builtin)
	if !ok {
		return ""
	}
	return builtin.Name()
}

func checkDelayedBorrowedCall(t *testing.T, fset *token.FileSet, info *types.Info, call *ast.CallExpr, action string) {
	t.Helper()
	if name, ok := borrowedRetentionExpr(info, call.Fun); ok {
		t.Fatalf("%s %s borrowed stream receiver %s", fset.Position(call.Fun.Pos()), action, name)
	}
	for _, argument := range call.Args {
		if name, ok := borrowedRetentionExpr(info, argument); ok {
			t.Fatalf("%s %s borrowed stream value %s", fset.Position(argument.Pos()), action, name)
		}
	}
}

func callParameterType(info *types.Info, call *ast.CallExpr, argument int) (types.Type, bool) {
	if info.Types[call.Fun].IsType() {
		if argument != 0 {
			return nil, false
		}
		return info.TypeOf(call), true
	}
	signature, ok := types.Unalias(info.TypeOf(call.Fun)).(*types.Signature)
	if !ok || signature.Params().Len() == 0 {
		return nil, false
	}
	parameter := argument
	if signature.Variadic() && parameter >= signature.Params().Len()-1 {
		parameter = signature.Params().Len() - 1
	}
	if parameter >= signature.Params().Len() {
		return nil, false
	}
	typ := signature.Params().At(parameter).Type()
	if signature.Variadic() && argument >= signature.Params().Len()-1 && call.Ellipsis == token.NoPos {
		if slice, ok := types.Unalias(typ).(*types.Slice); ok {
			typ = slice.Elem()
		}
	}
	return typ, true
}

type borrowedStreamValue struct {
	name string
	typ  types.Type
}

func borrowedRetentionExpr(info *types.Info, expression ast.Expr) (string, bool) {
	value, ok := borrowedRetentionValue(info, expression)
	return value.name, ok
}

func borrowedRetentionValue(info *types.Info, expression ast.Expr) (borrowedStreamValue, bool) {
	if expression == nil {
		return borrowedStreamValue{}, false
	}
	if call, ok := expression.(*ast.CallExpr); ok && isOwnedStreamConstructorCall(info, call) {
		return borrowedStreamValue{}, false
	}
	if name, ok := streamBorrowedType(info.TypeOf(expression)); ok {
		return borrowedStreamValue{name: name, typ: info.TypeOf(expression)}, true
	}
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return borrowedRetentionValue(info, expression.X)
	case *ast.UnaryExpr:
		return borrowedRetentionValue(info, expression.X)
	case *ast.CompositeLit:
		for _, element := range expression.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				if _, isMap := types.Unalias(info.TypeOf(expression)).(*types.Map); isMap {
					if value, ok := borrowedRetentionValue(info, pair.Key); ok {
						return value, true
					}
				}
				element = pair.Value
			}
			if value, ok := borrowedRetentionValue(info, element); ok {
				return value, true
			}
		}
	case *ast.CallExpr:
		for _, argument := range expression.Args {
			value, ok := borrowedRetentionValue(info, argument)
			if ok && typeCanRetainBorrowedType(info.TypeOf(expression), value.typ) {
				return value, true
			}
		}
	case *ast.FuncLit:
		var captured borrowedStreamValue
		ast.Inspect(expression.Body, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			object, ok := info.Uses[identifier].(*types.Var)
			if !ok || object.Pos() >= expression.Pos() && object.Pos() <= expression.End() {
				return true
			}
			if name, ok := streamBorrowedType(object.Type()); ok {
				captured = borrowedStreamValue{name: name, typ: object.Type()}
				return false
			}
			return true
		})
		if captured.name != "" {
			return captured, true
		}
	case *ast.SelectorExpr:
		selection := info.Selections[expression]
		if selection != nil && selection.Kind() == types.MethodVal {
			if name, ok := streamBorrowedType(selection.Recv()); ok {
				return borrowedStreamValue{name: name, typ: selection.Recv()}, true
			}
		}
	}
	return borrowedStreamValue{}, false
}

func typeCanRetainBorrowedType(destination, borrowed types.Type) bool {
	seen := make(map[types.Type]bool)
	var visit func(types.Type) bool
	visit = func(destination types.Type) bool {
		if destination == nil {
			return false
		}
		destination = types.Unalias(destination)
		if seen[destination] {
			return false
		}
		seen[destination] = true
		if types.AssignableTo(borrowed, destination) {
			return true
		}
		switch destination := destination.(type) {
		case *types.Named:
			return visit(destination.Underlying())
		case *types.Pointer:
			return visit(destination.Elem())
		case *types.Array:
			return visit(destination.Elem())
		case *types.Slice:
			return visit(destination.Elem())
		case *types.Map:
			return visit(destination.Key()) || visit(destination.Elem())
		case *types.Chan:
			return visit(destination.Elem())
		case *types.Struct:
			for field := range destination.Fields() {
				if visit(field.Type()) {
					return true
				}
			}
		case *types.Tuple:
			for variable := range destination.Variables() {
				if visit(variable.Type()) {
					return true
				}
			}
		case *types.Signature:
			return true
		}
		return false
	}
	return visit(destination)
}

func typeCanHideBorrowed(typ types.Type) bool {
	seen := make(map[types.Type]bool)
	var visit func(types.Type) bool
	visit = func(typ types.Type) bool {
		if typ == nil {
			return false
		}
		typ = types.Unalias(typ)
		if seen[typ] {
			return false
		}
		seen[typ] = true
		switch typ := typ.(type) {
		case *types.Named:
			return visit(typ.Underlying())
		case *types.Interface:
			return true
		case *types.TypeParam:
			return visit(typ.Constraint())
		case *types.Pointer:
			return visit(typ.Elem())
		case *types.Array:
			return visit(typ.Elem())
		case *types.Slice:
			return visit(typ.Elem())
		case *types.Map:
			return visit(typ.Key()) || visit(typ.Elem())
		case *types.Chan:
			return visit(typ.Elem())
		case *types.Struct:
			for field := range typ.Fields() {
				if visit(field.Type()) {
					return true
				}
			}
		case *types.Signature:
			return true
		case *types.Tuple:
			for variable := range typ.Variables() {
				if visit(variable.Type()) {
					return true
				}
			}
		}
		return false
	}
	return visit(typ)
}

func astParentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	return parents
}

func checkStreamBoundarySelector(t *testing.T, fset *token.FileSet, info *types.Info, parents map[ast.Node]ast.Node, sel *ast.SelectorExpr) {
	t.Helper()
	selection := info.Selections[sel]
	if selection == nil {
		return
	}
	if selection.Kind() == types.MethodVal || selection.Kind() == types.MethodExpr {
		switch {
		case isStreamNamed(selection.Recv(), "Parser") && sel.Sel.Name == "Next":
			if !allowedParserNext(sel, parents) {
				t.Fatalf("%s consumes a parser token outside a fresh local assignment", fset.Position(sel.Sel.Pos()))
			}
		case isStreamNamed(selection.Recv(), "Attr") && sel.Sel.Name == "RawValue":
			if !allowedRawValueFastPath(info, sel, parents) {
				t.Fatalf("%s exposes a borrowed attribute value outside the validation fast path", fset.Position(sel.Sel.Pos()))
			}
		}
		return
	}
	if selection.Kind() != types.FieldVal {
		return
	}
	switch {
	case isStreamNamed(selection.Recv(), "Token"):
		switch sel.Sel.Name {
		case "Data", "Directive", "Start":
			if !allowedBorrowedTokenFieldUse(info, sel, parents) {
				t.Fatalf("%s uses borrowed token field %s outside an allowed immediate consumer", fset.Position(sel.Sel.Pos()), sel.Sel.Name)
			}
		}
	case isStreamNamed(selection.Recv(), "StartElement") && sel.Sel.Name == "Attr":
		if !allowedBorrowedStartAttrUse(info, sel, parents) {
			t.Fatalf("%s accesses borrowed token start attributes outside an allowed immediate consumer", fset.Position(sel.Sel.Pos()))
		}
	}
}

func allowedParserNext(selector *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	call, ok := parents[selector].(*ast.CallExpr)
	if !ok || call.Fun != selector {
		return false
	}
	assignment, ok := parents[call].(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Rhs) != 1 || assignment.Rhs[0] != call || len(assignment.Lhs) != 2 {
		return false
	}
	_, ok = assignment.Lhs[0].(*ast.Ident)
	return ok
}

func allowedRawValueFastPath(info *types.Info, selector *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	call, ok := parents[selector].(*ast.CallExpr)
	if !ok || call.Fun != selector {
		return false
	}
	assignment, ok := parents[call].(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Rhs) != 1 || assignment.Rhs[0] != call || len(assignment.Lhs) != 2 {
		return false
	}
	conditional, ok := parents[assignment].(*ast.IfStmt)
	if !ok || conditional.Init != assignment || len(conditional.Body.List) == 0 {
		return false
	}
	raw, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok {
		return false
	}
	rawObject := info.Defs[raw]
	if rawObject == nil {
		return false
	}
	var uses []*ast.Ident
	for identifier, object := range info.Uses {
		if object == rawObject {
			uses = append(uses, identifier)
		}
	}
	if len(uses) != 1 {
		return false
	}
	consumer, ok := parents[uses[0]].(*ast.CallExpr)
	if !ok || len(consumer.Args) != 2 || consumer.Args[1] != uses[0] {
		return false
	}
	consumerAssignment, ok := parents[consumer].(*ast.AssignStmt)
	if !ok || conditional.Body.List[0] != consumerAssignment {
		return false
	}
	identity, ok := streamBoundaryCallIdentity(info, consumer)
	return ok && identity == (streamBoundaryCall{
		pkgPath:  "github.com/jacoelho/xsd/internal/validate",
		receiver: "session",
		name:     "validateRawSimpleValue",
	})
}

func checkParserInvalidationSites(t *testing.T, fset *token.FileSet, info *types.Info, declaration *ast.FuncDecl) {
	t.Helper()
	checkParserInvalidationBody(t, fset, info, declaration.Body)
}

func checkParserInvalidationBody(t *testing.T, fset *token.FileSet, info *types.Info, body *ast.BlockStmt) {
	t.Helper()
	pos, message := parserInvalidationBodyIssue(info, body)
	if pos != token.NoPos {
		t.Fatalf("%s %s", fset.Position(pos), message)
	}
}

func parserInvalidationIssue(info *types.Info, declaration *ast.FuncDecl) (token.Pos, string) {
	return parserInvalidationBodyIssue(info, declaration.Body)
}

func parserInvalidationBodyIssue(info *types.Info, body *ast.BlockStmt) (token.Pos, string) {
	if body == nil {
		return token.NoPos, ""
	}
	var nextCalls []*ast.CallExpr
	var invalidators []*ast.CallExpr
	ast.Inspect(body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok && literal != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identity, ok := streamBoundaryCallIdentity(info, call)
		if !ok || identity.pkgPath != "github.com/jacoelho/xsd/internal/stream" || identity.receiver != "Parser" {
			return true
		}
		switch identity.name {
		case "Next":
			nextCalls = append(nextCalls, call)
		case "Reset", "ResetWithLimits", "Detach":
			invalidators = append(invalidators, call)
		}
		return true
	})
	if len(nextCalls) > 1 {
		return nextCalls[1].Pos(), "calls Parser.Next at more than one site; prior borrowed tokens can be invalidated"
	}
	if len(nextCalls) == 0 {
		return token.NoPos, ""
	}
	for _, invalidator := range invalidators {
		if invalidator.Pos() > nextCalls[0].Pos() {
			return invalidator.Pos(), "invalidates parser storage after acquiring a borrowed token"
		}
	}
	return token.NoPos, ""
}

func isStreamNamed(typ types.Type, name string) bool {
	typ = types.Unalias(typ)
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(ptr.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Name() != name || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "github.com/jacoelho/xsd/internal/stream"
}

func allowedBorrowedTokenFieldUse(info *types.Info, sel *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	parent := parents[sel]
	if delayedStreamBoundaryCall(parent, parents) {
		return false
	}
	if sel.Sel.Name == "Start" {
		if selector, ok := parent.(*ast.SelectorExpr); ok {
			return selector.Sel.Name == "XMLStartElement" || selector.Sel.Name == "Attr"
		}
		return allowedStreamBoundaryCall(info, parent, allowedTokenStartCalls)
	}
	return allowedStreamBoundaryCall(info, parent, allowedTokenDataCalls)
}

func allowedBorrowedStartAttrUse(info *types.Info, sel *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	parent := parents[sel]
	if delayedStreamBoundaryCall(parent, parents) {
		return false
	}
	switch parent.(type) {
	case *ast.RangeStmt, *ast.IndexExpr:
		return true
	}
	return allowedStreamBoundaryCall(info, parent, allowedStartAttrCalls)
}

func delayedStreamBoundaryCall(node ast.Node, parents map[ast.Node]ast.Node) bool {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch parents[call].(type) {
	case *ast.GoStmt, *ast.DeferStmt:
		return true
	default:
		return false
	}
}

type streamBoundaryCall struct {
	pkgPath  string
	receiver string
	name     string
	builtin  bool
}

var allowedTokenStartCalls = map[streamBoundaryCall]bool{
	{pkgPath: "github.com/jacoelho/xsd/internal/compile", receiver: "schemaParseState", name: "start"}:      true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", receiver: "session", name: "start"}:              true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", receiver: "xmlWellFormedChecker", name: "start"}: true,
}

var allowedTokenDataCalls = map[streamBoundaryCall]bool{
	{pkgPath: "github.com/jacoelho/xsd/internal/compile", receiver: "schemaParseState", name: "chars"}:             true,
	{pkgPath: "github.com/jacoelho/xsd/internal/compile", receiver: "schemaParseState", name: "ValidateDirective"}: true,
	{pkgPath: "github.com/jacoelho/xsd/internal/lex", name: "IsXMLWhitespaceBytes"}:                                true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", receiver: "session", name: "chars"}:                     true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", receiver: "xmlWellFormedChecker", name: "chars"}:        true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", name: "ValidateDirective"}:                              true,
}

var allowedStartAttrCalls = map[streamBoundaryCall]bool{
	{name: "len", builtin: true}: true,
	{pkgPath: "github.com/jacoelho/xsd/internal/xmlns", receiver: "Stack", name: "PushStream"}:                     true,
	{pkgPath: "github.com/jacoelho/xsd/internal/xmlns", name: "ValidateUniqueAttributes"}:                          true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", receiver: "session", name: "assessElementStart"}:        true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", receiver: "session", name: "recordSchemaLocationHints"}: true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", receiver: "session", name: "validateStartAttributes"}:   true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", name: "RootStart"}:                                      true,
	{pkgPath: "github.com/jacoelho/xsd/internal/validate", name: "xsiStartAttributeFlagsFor"}:                      true,
}

func allowedStreamBoundaryCall(info *types.Info, parent ast.Node, allowed map[streamBoundaryCall]bool) bool {
	call, ok := parent.(*ast.CallExpr)
	if !ok {
		return false
	}
	identity, ok := streamBoundaryCallIdentity(info, call)
	return ok && allowed[identity]
}

func streamBoundaryCallIdentity(info *types.Info, call *ast.CallExpr) (streamBoundaryCall, bool) {
	var obj types.Object
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		obj = info.Uses[fn]
	case *ast.SelectorExpr:
		obj = info.Uses[fn.Sel]
	default:
		return streamBoundaryCall{}, false
	}
	switch obj := obj.(type) {
	case *types.Builtin:
		return streamBoundaryCall{name: obj.Name(), builtin: true}, true
	case *types.Func:
		identity := streamBoundaryCall{name: obj.Name()}
		if obj.Pkg() != nil {
			identity.pkgPath = obj.Pkg().Path()
		}
		signature, ok := obj.Type().(*types.Signature)
		if !ok || signature.Recv() == nil {
			return identity, true
		}
		receiver := signature.Recv().Type()
		if pointer, isPointer := receiver.(*types.Pointer); isPointer {
			receiver = pointer.Elem()
		}
		named, ok := receiver.(*types.Named)
		if !ok || named.Obj().Pkg() == nil {
			return streamBoundaryCall{}, false
		}
		identity.receiver = named.Obj().Name()
		return identity, true
	default:
		return streamBoundaryCall{}, false
	}
}

func TestStreamBoundaryCallIdentityRejectsNameCollisions(t *testing.T) {
	for _, tt := range []struct {
		name    string
		source  string
		allowed bool
	}{
		{
			name: "approved method",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
type session struct{}
func (*session) validateStartAttributes([]stream.Attr) {}
func use(s *session, tok stream.Token) { s.validateStartAttributes(tok.Start.Attr) }`,
			allowed: true,
		},
		{
			name: "same-name function",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
var sink any
func validateStartAttributes(attrs []stream.Attr) { sink = attrs }
func use(tok stream.Token) { validateStartAttributes(tok.Start.Attr) }`,
		},
		{
			name: "same-name method on other receiver",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
var sink any
type imposter struct{}
func (*imposter) validateStartAttributes(attrs []stream.Attr) { sink = attrs }
func use(s *imposter, tok stream.Token) { s.validateStartAttributes(tok.Start.Attr) }`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "boundary.go", tt.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{
				Selections: make(map[*ast.SelectorExpr]*types.Selection),
				Uses:       make(map[*ast.Ident]types.Object),
			}
			conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
			if _, err := conf.Check("github.com/jacoelho/xsd/internal/validate", fset, []*ast.File{file}, info); err != nil {
				t.Fatalf("type-check fixture: %v", err)
			}
			parents := astParentMap(file)
			var got bool
			var found bool
			ast.Inspect(file, func(node ast.Node) bool {
				sel, ok := node.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Attr" {
					return true
				}
				selection := info.Selections[sel]
				if selection == nil || !isStreamNamed(selection.Recv(), "StartElement") {
					return true
				}
				found = true
				got = allowedBorrowedStartAttrUse(info, sel, parents)
				return false
			})
			if !found {
				t.Fatal("fixture has no borrowed StartElement.Attr selector")
			}
			if got != tt.allowed {
				t.Fatalf("allowedBorrowedStartAttrUse() = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestStreamBorrowedTypeResolvesImportAliases(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
	}{
		{
			name: "renamed import",
			source: `package validate
import borrowed "github.com/jacoelho/xsd/internal/stream"
var retained borrowed.Token`,
		},
		{
			name: "dot import and type alias",
			source: `package validate
import . "github.com/jacoelho/xsd/internal/stream"
type borrowed = Attr
var retained []borrowed`,
		},
		{
			name: "generic container",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
type holder[T any] struct { value T }
var retained holder[stream.Attr]`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, info := typeCheckStreamBoundaryFixture(t, tt.source)
			object := info.Defs[fixtureIdent(t, info.Defs, "retained")]
			if object == nil {
				t.Fatal("fixture retained variable has no type object")
			}
			if _, ok := streamBorrowedType(object.Type()); !ok {
				t.Fatalf("streamBorrowedType(%s) = false, want true", object.Type())
			}
		})
	}
}

func TestStreamNamedResolvesReceiverAlias(t *testing.T) {
	const source = `package validate
import "github.com/jacoelho/xsd/internal/stream"
type borrowed = stream.Token
func use(token borrowed) { _ = token.Data }`
	_, file, info := typeCheckStreamBoundaryFixture(t, source)
	var found bool
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Data" {
			return true
		}
		selection := info.Selections[selector]
		if selection == nil {
			t.Fatal("fixture Data selector has no type selection")
		}
		found = true
		if !isStreamNamed(selection.Recv(), "Token") {
			t.Fatalf("isStreamNamed(%s, Token) = false, want true", selection.Recv())
		}
		return false
	})
	if !found {
		t.Fatal("fixture has no Data selector")
	}
}

func TestStreamBoundaryRejectsBorrowedResultsExceptOwnedConstructors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		source  string
		allowed bool
	}{
		{
			name: "borrowed pass-through",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func pass(token stream.Token) stream.Token { return token }`,
		},
		{
			name: "owned constructor wrapper",
			source: `package validate
import (
	"encoding/xml"
	"github.com/jacoelho/xsd/internal/stream"
)
func attr(name xml.Name, value string) stream.Attr { return stream.OwnedAttr(name, value) }`,
			allowed: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fset, file, info := typeCheckStreamBoundaryFixture(t, tt.source)
			parents := astParentMap(file)
			var result *ast.Field
			ast.Inspect(file, func(node ast.Node) bool {
				declaration, ok := node.(*ast.FuncDecl)
				if !ok || declaration.Type.Results == nil {
					return true
				}
				result = declaration.Type.Results.List[0]
				return false
			})
			if result == nil {
				t.Fatal("fixture has no function result")
			}
			if _, ok := streamBorrowedType(info.TypeOf(result.Type)); !ok {
				t.Fatalf("fixture result %s is not classified as borrowed", fset.Position(result.Pos()))
			}
			if isFuncParam(result, parents) {
				t.Fatal("function result classified as a parameter")
			}
			if got := isOwnedStreamConstructorResult(info, result, parents); got != tt.allowed {
				t.Fatalf("isOwnedStreamConstructorResult() = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestBorrowedRetentionExprRejectsTypeErasure(t *testing.T) {
	for _, tt := range []struct {
		name     string
		source   string
		borrowed bool
	}{
		{
			name: "empty interface",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(token stream.Token) any { return token }`,
			borrowed: true,
		},
		{
			name: "multi result",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(token stream.Token) (any, error) { return token, nil }`,
			borrowed: true,
		},
		{
			name: "interface slice",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(token stream.Token) []any { return []any{token} }`,
			borrowed: true,
		},
		{
			name: "interface struct",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(token stream.Token) any { return struct{ Value any }{token} }`,
			borrowed: true,
		},
		{
			name: "generic erasure",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func identity[T any](value T) T { return value }
func leak(token stream.Token) any { return identity[any](token) }`,
			borrowed: true,
		},
		{
			name: "constrained type parameter",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak[T interface{ stream.Token }](token T) T { return token }`,
			borrowed: true,
		},
		{
			name: "closure capture",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
var sink any
func leak(token stream.Token) func() { return func() { sink = token } }`,
			borrowed: true,
		},
		{
			name: "owned scalar projection",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(token stream.Token) int { return token.Line }`,
		},
		{
			name: "owned byte copy",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(token stream.Token) []byte { return token.AppendData(nil) }`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, file, info := typeCheckStreamBoundaryFixture(t, tt.source)
			result := fixtureFunctionReturn(t, file, "leak").Results[0]
			if _, got := borrowedRetentionExpr(info, result); got != tt.borrowed {
				t.Fatalf("borrowedRetentionExpr() = %v, want %v", got, tt.borrowed)
			}
		})
	}
}

func TestBorrowedRetentionRejectsAssignmentsAndCallErasure(t *testing.T) {
	const source = `package validate
import "github.com/jacoelho/xsd/internal/stream"
var sink any
func hold(any) {}
func leak(token stream.Token) {
	sink = token
	hold(token)
}`
	_, file, info := typeCheckStreamBoundaryFixture(t, source)
	var assignment *ast.AssignStmt
	var call *ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			assignment = node
		case *ast.CallExpr:
			call = node
		}
		return true
	})
	if assignment == nil || call == nil {
		t.Fatal("fixture is missing assignment or call")
	}
	if !typeCanHideBorrowed(info.TypeOf(assignment.Lhs[0])) {
		t.Fatal("interface assignment destination is not retaining")
	}
	if _, ok := borrowedRetentionExpr(info, assignment.Rhs[0]); !ok {
		t.Fatal("interface assignment source is not borrowed")
	}
	parameter, ok := callParameterType(info, call, 0)
	if !ok || !typeCanHideBorrowed(parameter) {
		t.Fatal("interface call parameter is not retaining")
	}
	if _, ok := borrowedRetentionExpr(info, call.Args[0]); !ok {
		t.Fatal("interface call argument is not borrowed")
	}
}

func TestBorrowedRetentionRejectsBoundMethods(t *testing.T) {
	const source = `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(token stream.Token) { later := token.AppendData; _ = later }`
	_, file, info := typeCheckStreamBoundaryFixture(t, source)
	var method *ast.SelectorExpr
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "AppendData" {
			method = selector
		}
		return true
	})
	if method == nil {
		t.Fatal("fixture has no bound method")
	}
	if _, ok := borrowedRetentionExpr(info, method); !ok {
		t.Fatal("bound method receiver is not classified as borrowed")
	}
}

func TestBorrowedRetentionRejectsDelayedConsumers(t *testing.T) {
	for _, keyword := range []string{"defer", "go"} {
		t.Run(keyword, func(t *testing.T) {
			source := `package validate
import "github.com/jacoelho/xsd/internal/stream"
type session struct{}
func (*session) chars([]byte) {}
func leak(s *session, token stream.Token) { ` + keyword + ` s.chars(token.Data) }`
			_, file, info := typeCheckStreamBoundaryFixture(t, source)
			parents := astParentMap(file)
			var data *ast.SelectorExpr
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "Data" {
					data = selector
				}
				return true
			})
			if data == nil {
				t.Fatal("fixture has no borrowed Data selector")
			}
			if !delayedStreamBoundaryCall(parents[data], parents) {
				t.Fatal("delayed call was not recognized")
			}
			if allowedBorrowedTokenFieldUse(info, data, parents) {
				t.Fatal("delayed borrowed-field consumer was allowed")
			}
		})
	}
}

func TestBorrowedRetentionRejectsMapKeys(t *testing.T) {
	const source = `package validate
import "github.com/jacoelho/xsd/internal/stream"
var retained = map[any]struct{}{}
func leak(token stream.Token) {
	retained[&token] = struct{}{}
	_ = map[any]struct{}{&token: {}}
}`
	_, file, info := typeCheckStreamBoundaryFixture(t, source)
	var assignmentKey ast.Expr
	var literalKey ast.Expr
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if index, ok := node.Lhs[0].(*ast.IndexExpr); ok {
				assignmentKey = index.Index
			}
		case *ast.CompositeLit:
			if len(node.Elts) != 0 {
				if pair, ok := node.Elts[0].(*ast.KeyValueExpr); ok {
					literalKey = pair.Key
				}
			}
		}
		return true
	})
	for label, key := range map[string]ast.Expr{"assignment": assignmentKey, "literal": literalKey} {
		if key == nil {
			t.Fatalf("fixture has no %s map key", label)
		}
		if _, ok := borrowedRetentionExpr(info, key); !ok {
			t.Fatalf("%s map key is not classified as borrowed", label)
		}
	}
}

func TestBorrowedRetentionRejectsGenericTupleAssignment(t *testing.T) {
	const source = `package validate
import "github.com/jacoelho/xsd/internal/stream"
var retained any
func erase[T any](value T) (any, bool) { return value, true }
func leak(token stream.Token) { retained, _ = erase[stream.Token](token) }`
	_, file, info := typeCheckStreamBoundaryFixture(t, source)
	var assignment *ast.AssignStmt
	ast.Inspect(file, func(node ast.Node) bool {
		candidate, ok := node.(*ast.AssignStmt)
		if ok && len(candidate.Lhs) == 2 {
			assignment = candidate
		}
		return true
	})
	if assignment == nil || len(assignment.Rhs) != 1 {
		t.Fatal("fixture has no multi-valued assignment")
	}
	value, ok := borrowedRetentionValue(info, assignment.Rhs[0])
	if !ok {
		t.Fatal("generic tuple result is not classified as borrowed")
	}
	tuple, ok := types.Unalias(info.TypeOf(assignment.Rhs[0])).(*types.Tuple)
	if !ok || tuple.Len() != 2 {
		t.Fatal("generic call does not have a two-value tuple type")
	}
	if !assignmentDestinationRetains(info, assignment.Lhs[0]) || !typeCanRetainBorrowedType(tuple.At(0).Type(), value.typ) {
		t.Fatal("generic tuple assignment destination was not recognized as retaining")
	}
}

func TestParserNextRequiresFreshLocalAndOneAcquisitionSite(t *testing.T) {
	for _, tt := range []struct {
		name             string
		body             string
		allowedNext      bool
		invalidationFail bool
	}{
		{name: "fresh local", body: `token, err := parser.Next(); _, _ = token, err`, allowedNext: true},
		{name: "existing destination", body: `retained, _ = parser.Next()`},
		{name: "second acquisition", body: `token, _ := parser.Next(); _, _ = parser.Next(); _ = token`, allowedNext: true, invalidationFail: true},
		{name: "reset with limits", body: `names, values := new(stream.Cache), new(stream.Cache); token, _ := parser.Next(); _ = parser.ResetWithLimits(nil, names, values, stream.Limits{}); _ = token`, allowedNext: true, invalidationFail: true},
		{name: "stored method expression", body: `next := (*stream.Parser).Next; token, _ := next(parser); _ = token`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := `package validate
import "github.com/jacoelho/xsd/internal/stream"
var retained any
func use(parser *stream.Parser) { ` + tt.body + ` }`
			_, file, info := typeCheckStreamBoundaryFixture(t, source)
			parents := astParentMap(file)
			var next []*ast.SelectorExpr
			var function *ast.FuncDecl
			ast.Inspect(file, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.FuncDecl:
					if node.Name.Name == "use" {
						function = node
					}
				case *ast.SelectorExpr:
					if node.Sel.Name == "Next" {
						next = append(next, node)
					}
				}
				return true
			})
			if function == nil || len(next) == 0 {
				t.Fatal("fixture has no function or Parser.Next call")
			}
			if got := allowedParserNext(next[0], parents); got != tt.allowedNext {
				t.Fatalf("allowedParserNext() = %v, want %v", got, tt.allowedNext)
			}
			pos, _ := parserInvalidationIssue(info, function)
			if got := pos != token.NoPos; got != tt.invalidationFail {
				t.Fatalf("parserInvalidationIssue() = %v, want %v", got, tt.invalidationFail)
			}
		})
	}
}

func TestParserInvalidationChecksFunctionLiteralScopes(t *testing.T) {
	const source = `package validate
import "github.com/jacoelho/xsd/internal/stream"
func use(parser *stream.Parser) {
	func() {
		first, _ := parser.Next()
		_, _ = parser.Next()
		_ = first
	}()
}`
	_, file, info := typeCheckStreamBoundaryFixture(t, source)
	var literal *ast.FuncLit
	ast.Inspect(file, func(node ast.Node) bool {
		if candidate, ok := node.(*ast.FuncLit); ok {
			literal = candidate
			return false
		}
		return true
	})
	if literal == nil {
		t.Fatal("fixture has no function literal")
	}
	if pos, _ := parserInvalidationBodyIssue(info, literal.Body); pos == token.NoPos {
		t.Fatal("function literal's second Parser.Next site was not rejected")
	}
}

func TestRawValueIsRestrictedToExactImmediateFastPath(t *testing.T) {
	for _, tt := range []struct {
		name    string
		source  string
		allowed bool
	}{
		{
			name: "escape",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
var retained []byte
func leak(attr stream.Attr) { retained, _ = attr.RawValue() }`,
		},
		{
			name: "stored method expression",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
func leak(attr *stream.Attr) { rawValue := (*stream.Attr).RawValue; _, _ = rawValue(attr) }`,
		},
		{
			name: "direct method expression fast path",
			source: `package validate
import "github.com/jacoelho/xsd/internal/stream"
type session struct{}
func (*session) validateRawSimpleValue(_ int, raw []byte) (bool, error) { return len(raw) != 0, nil }
func use(s *session, attr *stream.Attr) {
	if raw, ok := (*stream.Attr).RawValue(attr); ok {
		handled, err := s.validateRawSimpleValue(0, raw)
		_, _ = handled, err
	}
}`,
			allowed: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, file, info := typeCheckStreamBoundaryFixture(t, tt.source)
			parents := astParentMap(file)
			var rawValue *ast.SelectorExpr
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "RawValue" {
					rawValue = selector
				}
				return true
			})
			if rawValue == nil {
				t.Fatal("fixture has no RawValue selector")
			}
			if got := allowedRawValueFastPath(info, rawValue, parents); got != tt.allowed {
				t.Fatalf("allowedRawValueFastPath() = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestOwnedStreamConstructorRequiresExactTopLevelFunction(t *testing.T) {
	const source = `package stream
import "encoding/xml"
type Attr struct{}
type helper struct{}
func (helper) OwnedAttr(xml.Name, string) Attr { return Attr{} }
func wrapper(name xml.Name, value string) Attr { return helper{}.OwnedAttr(name, value) }`
	fset := token.NewFileSet()
	file := fixtureFile(t, fset, source)
	info := &types.Info{
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/stream", fset, []*ast.File{file}, info); err != nil {
		t.Fatalf("type-check fixture: %v", err)
	}
	returned := fixtureFunctionReturn(t, file, "wrapper")
	call, ok := returned.Results[0].(*ast.CallExpr)
	if !ok {
		t.Fatal("fixture return is not a call")
	}
	if isOwnedStreamConstructorCall(info, call) {
		t.Fatal("same-named method accepted as an owned stream constructor")
	}
}

func TestNonHostStreamConsumerFailsClosed(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "consumer_js.go")
	const source = `//go:build js && wasm

package consumer
import "github.com/jacoelho/xsd/internal/stream"
var _ stream.Token`
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	match, err := build.Default.MatchFile(dir, filepath.Base(file))
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Fatal("js/wasm fixture unexpectedly matches host build")
	}
	fset := token.NewFileSet()
	parsed := fixtureFile(t, fset, source)
	if !fileImportsStreamPackage(parsed) {
		t.Fatal("excluded stream consumer was not recognized")
	}
}

func TestStreamBoundaryIgnoresUnrelatedRawFields(t *testing.T) {
	const source = `package validate
type local struct { raw []byte }
func use(value local) { _ = value.raw; _ = local{raw: nil} }`
	fset, file, info := typeCheckStreamBoundaryFixture(t, source)
	checkStreamBoundaryFile(t, fset, info, file)
}

func typeCheckStreamBoundaryFixture(t *testing.T, source string) (*token.FileSet, *ast.File, *types.Info) {
	t.Helper()
	fset := token.NewFileSet()
	file := fixtureFile(t, fset, source)
	info := &types.Info{
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/validate", fset, []*ast.File{file}, info); err != nil {
		t.Fatalf("type-check fixture: %v", err)
	}
	return fset, file, info
}

func fixtureFile(t *testing.T, fset *token.FileSet, source string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(fset, "boundary.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func fixtureIdent(t *testing.T, objects map[*ast.Ident]types.Object, name string) *ast.Ident {
	t.Helper()
	for ident := range objects {
		if ident.Name == name {
			return ident
		}
	}
	t.Fatalf("fixture has no identifier %q", name)
	return nil
}

func fixtureFunctionReturn(t *testing.T, file *ast.File, name string) *ast.ReturnStmt {
	t.Helper()
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != name || function.Body == nil {
			continue
		}
		for _, statement := range function.Body.List {
			if returned, ok := statement.(*ast.ReturnStmt); ok {
				return returned
			}
		}
	}
	t.Fatalf("fixture function %q has no direct return", name)
	return nil
}

func isFuncParam(field *ast.Field, parents map[ast.Node]ast.Node) bool {
	list, ok := parents[field].(*ast.FieldList)
	if !ok {
		return false
	}
	function, ok := parents[list].(*ast.FuncType)
	return ok && function.Params == list
}

func isOwnedStreamConstructorResult(info *types.Info, field *ast.Field, parents map[ast.Node]ast.Node) bool {
	list, ok := parents[field].(*ast.FieldList)
	if !ok {
		return false
	}
	function, ok := parents[list].(*ast.FuncType)
	if !ok || function.Results != list || len(list.List) != 1 {
		return false
	}
	declaration, ok := parents[function].(*ast.FuncDecl)
	if !ok || declaration.Body == nil || len(declaration.Body.List) != 1 {
		return false
	}
	returned, ok := declaration.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(returned.Results) != 1 {
		return false
	}
	call, ok := returned.Results[0].(*ast.CallExpr)
	return ok && isOwnedStreamConstructorCall(info, call)
}

func isOwnedStreamConstructorCall(info *types.Info, call *ast.CallExpr) bool {
	identity, ok := streamBoundaryCallIdentity(info, call)
	if !ok || identity.pkgPath != "github.com/jacoelho/xsd/internal/stream" || identity.receiver != "" || identity.builtin {
		return false
	}
	switch identity.name {
	case "OwnedAttr", "OwnedAttrs", "OwnedStartElement":
		return true
	default:
		return false
	}
}
