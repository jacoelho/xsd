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

func TestInternalImplementationPackagesExist(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{
		"internal/compile",
		"internal/format",
		"internal/runtime",
		"internal/source",
		"internal/stream",
		"internal/validate",
		"xsderrors",
	} {
		info, err := os.Stat(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("missing package directory %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a package directory", dir)
		}
	}
}

func TestRootCompileIsFacade(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filepath.Join(root, "compile.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse compile.go: %v", err)
	}
	if !importsPath(parsed, "github.com/jacoelho/xsd/internal/compile") {
		t.Fatal("compile.go does not import internal/compile")
	}
	if !callsSelector(parsed, "compile", "Compile") {
		t.Fatal("CompileWithOptions does not delegate to compile.Compile")
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if strings.HasPrefix(fn.Name.Name, "compile") && fn.Name.Name != "Compile" {
			t.Fatalf("root compile.go owns compiler helper %s", fn.Name.Name)
		}
	}
}

func TestValidationFacadeOwnsSessionConstruction(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	internal := make([]*ast.File, 0)
	for _, file := range productionGoFiles(t, filepath.Join(root, "internal/validate")) {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		internal = append(internal, parsed)
	}
	for _, name := range []string{"NewSession", "Validate"} {
		if !slices.ContainsFunc(internal, func(file *ast.File) bool { return declaresFunction(file, name) }) {
			t.Fatalf("internal validation facade does not declare %s", name)
		}
	}
	for _, file := range internal {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == "Init" && receiverTypeName(fn) == "Session" {
				t.Fatal("validation Session exposes two-phase Init lifecycle")
			}
		}
	}

	var publicFiles []*ast.File
	var public *ast.File
	for _, path := range productionRootFiles(t, root) {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		publicFiles = append(publicFiles, file)
		if filepath.Base(path) == "session.go" {
			public = file
		}
	}
	if public == nil {
		t.Fatal("missing public validation facade session.go")
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd", fset, publicFiles, info)
	if err != nil {
		t.Fatalf("type-check public validation facade: %v", err)
	}
	validatePkg := importedPackage(pkg, "github.com/jacoelho/xsd/internal/validate")
	if validatePkg == nil {
		t.Fatal("public validation facade does not import internal/validate")
	}
	validateCall := validatePkg.Scope().Lookup("Validate")
	newSessionCall := validatePkg.Scope().Lookup("NewSession")
	publicSession := pkg.Scope().Lookup("Session")
	if validateCall == nil || newSessionCall == nil || publicSession == nil {
		t.Fatal("validation facade types are incomplete")
	}
	validateWithOptions := engineMethodDeclaration(public, "ValidateWithOptions")
	if validateWithOptions == nil || !returnsOnlyExactCall(
		info,
		validateWithOptions,
		validateCall,
		pkg.Scope().Lookup("internalValidateOptions"),
	) {
		t.Fatal("Engine.ValidateWithOptions does not return internal/validate.Validate's result")
	}
	newSession := engineMethodDeclaration(public, "NewSession")
	if newSession == nil || !newSessionResultFlowsToPublicSession(info, newSession, newSessionCall, publicSession.Type()) {
		t.Fatal("Engine.NewSession does not propagate internal/validate.NewSession's results")
	}
}

func TestValidationFacadeFlowRejectsDeadAndUnrelatedCalls(t *testing.T) {
	const source = `package xsd
import validate "github.com/jacoelho/xsd/internal/validate"
type Engine struct{}
type Session struct { session validate.Session }
type otherValidation struct{}
var validateAlias = validate.Validate
func (otherValidation) Validate() error { return nil }
func (otherValidation) NewSession() (validate.Session, error) { return validate.Session{}, nil }
func (*Engine) ValidateWithOptions() error {
	validate.Validate(nil, nil, validate.Options{})
	return (otherValidation{}).Validate()
}
func (*Engine) ValidateWithDuplicateCall() error {
	_ = validate.Validate(nil, nil, validate.Options{})
	return validate.Validate(nil, nil, validate.Options{})
}
func (*Engine) ValidateWithDeferredOverwrite() (err error) {
	defer func() { err = nil }()
	return validate.Validate(nil, nil, validate.Options{})
}
func (*Engine) ValidateThroughAlias() error {
	alias := validate.Validate
	_ = alias(nil, nil, validate.Options{})
	return validate.Validate(nil, nil, validate.Options{})
}
func (*Engine) ValidateThroughPackageAlias() error {
	_ = validateAlias(nil, nil, validate.Options{})
	return validate.Validate(nil, nil, validate.Options{})
}
func (*Engine) NewSession() (*Session, error) {
	inner, err := validate.NewSession(nil, validate.Options{})
	if err != nil { return nil, err }
	if false { return &Session{session: inner}, nil }
	other, err := (otherValidation{}).NewSession()
	if err != nil { return nil, err }
	return &Session{session: other}, nil
}
func (*Engine) NewSessionWithDeadErrorReturn() (*Session, error) {
	inner, err := validate.NewSession(nil, validate.Options{})
	if err != nil {
		if false { return nil, err }
	}
	return &Session{session: inner}, nil
}
func (*Engine) NewSessionWithOverwrittenResults() (*Session, error) {
	inner, err := validate.NewSession(nil, validate.Options{})
	inner, err = validate.Session{}, nil
	if err != nil { return nil, err }
	return &Session{session: inner}, nil
}
func (*Engine) NewSessionWithEarlyReturn() (*Session, error) {
	if false { return &Session{}, nil }
	inner, err := validate.NewSession(nil, validate.Options{})
	if err != nil { return nil, err }
	return &Session{session: inner}, nil
}
func (*Engine) NewSessionWithDeferredOverwrite() (result *Session, resultErr error) {
	defer func() { result = &Session{} }()
	inner, err := validate.NewSession(nil, validate.Options{})
	if err != nil { return nil, err }
	return &Session{session: inner}, nil
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "session.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("type-check synthetic facade: %v", err)
	}
	validatePkg := importedPackage(pkg, "github.com/jacoelho/xsd/internal/validate")
	validateWithOptions := engineMethodDeclaration(file, "ValidateWithOptions")
	if returnsOnlyExactCall(info, validateWithOptions, validatePkg.Scope().Lookup("Validate")) {
		t.Fatal("dead exact Validate call satisfied validation facade boundary")
	}
	duplicateValidate := engineMethodDeclaration(file, "ValidateWithDuplicateCall")
	if returnsOnlyExactCall(info, duplicateValidate, validatePkg.Scope().Lookup("Validate")) {
		t.Fatal("duplicate exact Validate calls satisfied validation facade boundary")
	}
	deferredValidate := engineMethodDeclaration(file, "ValidateWithDeferredOverwrite")
	if returnsOnlyExactCall(info, deferredValidate, validatePkg.Scope().Lookup("Validate")) {
		t.Fatal("deferred named-result overwrite satisfied validation facade boundary")
	}
	aliasedValidate := engineMethodDeclaration(file, "ValidateThroughAlias")
	if returnsOnlyExactCall(info, aliasedValidate, validatePkg.Scope().Lookup("Validate")) {
		t.Fatal("aliased Validate call satisfied validation facade boundary")
	}
	packageAliasedValidate := engineMethodDeclaration(file, "ValidateThroughPackageAlias")
	if returnsOnlyExactCall(info, packageAliasedValidate, validatePkg.Scope().Lookup("Validate")) {
		t.Fatal("package-aliased Validate call satisfied validation facade boundary")
	}
	newSession := engineMethodDeclaration(file, "NewSession")
	if newSessionResultFlowsToPublicSession(info, newSession, validatePkg.Scope().Lookup("NewSession"), pkg.Scope().Lookup("Session").Type()) {
		t.Fatal("alternate NewSession result path satisfied validation facade boundary")
	}
	deadErrorReturn := engineMethodDeclaration(file, "NewSessionWithDeadErrorReturn")
	if newSessionResultFlowsToPublicSession(info, deadErrorReturn, validatePkg.Scope().Lookup("NewSession"), pkg.Scope().Lookup("Session").Type()) {
		t.Fatal("dead nested error return satisfied validation facade boundary")
	}
	overwrittenResults := engineMethodDeclaration(file, "NewSessionWithOverwrittenResults")
	if newSessionResultFlowsToPublicSession(info, overwrittenResults, validatePkg.Scope().Lookup("NewSession"), pkg.Scope().Lookup("Session").Type()) {
		t.Fatal("overwritten constructor results satisfied validation facade boundary")
	}
	earlyReturn := engineMethodDeclaration(file, "NewSessionWithEarlyReturn")
	if newSessionResultFlowsToPublicSession(info, earlyReturn, validatePkg.Scope().Lookup("NewSession"), pkg.Scope().Lookup("Session").Type()) {
		t.Fatal("early constructor bypass satisfied validation facade boundary")
	}
	deferredOverwrite := engineMethodDeclaration(file, "NewSessionWithDeferredOverwrite")
	if newSessionResultFlowsToPublicSession(info, deferredOverwrite, validatePkg.Scope().Lookup("NewSession"), pkg.Scope().Lookup("Session").Type()) {
		t.Fatal("deferred named-result overwrite satisfied validation facade boundary")
	}
}

func importedPackage(pkg *types.Package, path string) *types.Package {
	for _, imported := range pkg.Imports() {
		if imported.Path() == path {
			return imported
		}
	}
	return nil
}

func returnsOnlyExactCall(info *types.Info, fn *ast.FuncDecl, expected types.Object, allowed ...types.Object) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, result := range fn.Type.Results.List {
		if len(result.Names) != 0 {
			return false
		}
	}
	returns := 0
	matches := 0
	exactCalls := 0
	exactReferences := 0
	allowedCalls := true
	inspectFunctionBody(fn.Body, func(node ast.Node) {
		if ident, ok := node.(*ast.Ident); ok && info.Uses[ident] == expected {
			exactReferences++
		}
		if call, ok := node.(*ast.CallExpr); ok {
			object := calledObject(info, call)
			if object == expected {
				exactCalls++
			} else if object == nil || !slices.Contains(allowed, object) {
				allowedCalls = false
			}
		}
		stmt, ok := node.(*ast.ReturnStmt)
		if !ok {
			return
		}
		returns++
		if len(stmt.Results) != 1 {
			return
		}
		call, ok := unparen(stmt.Results[0]).(*ast.CallExpr)
		if ok && calledObject(info, call) == expected {
			matches++
		}
	})
	return returns != 0 && matches == returns && exactCalls == 1 && exactReferences == 1 && allowedCalls
}

func newSessionResultFlowsToPublicSession(info *types.Info, fn *ast.FuncDecl, expected types.Object, sessionType types.Type) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, result := range fn.Type.Results.List {
		if len(result.Names) != 0 {
			return false
		}
	}
	var inner, callErr types.Object
	callPosition := -1
	exactCalls := 0
	for position, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := unparen(assign.Rhs[0]).(*ast.CallExpr)
		if !ok || calledObject(info, call) != expected {
			continue
		}
		first, firstOK := assign.Lhs[0].(*ast.Ident)
		second, secondOK := assign.Lhs[1].(*ast.Ident)
		if !firstOK || !secondOK {
			continue
		}
		exactCalls++
		callPosition = position
		inner = identObject(info, first)
		callErr = identObject(info, second)
	}
	if exactCalls != 1 || inner == nil || callErr == nil || callPosition != len(fn.Body.List)-3 ||
		statementsContainReturn(fn.Body.List[:callPosition]) {
		return false
	}
	errorBranch, ok := fn.Body.List[callPosition+1].(*ast.IfStmt)
	if !ok || errorBranch.Else != nil || !isNonNilCheck(info, errorBranch.Cond, callErr) ||
		len(errorBranch.Body.List) != 1 || !returnsNilAndObject(info, errorBranch.Body, callErr) {
		return false
	}
	success, ok := fn.Body.List[callPosition+2].(*ast.ReturnStmt)
	return ok && returnWrapsSessionObject(info, success, sessionType, inner)
}

func statementsContainReturn(statements []ast.Stmt) bool {
	found := false
	for _, statement := range statements {
		ast.Inspect(statement, func(node ast.Node) bool {
			if found {
				return false
			}
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}
			if _, ok := node.(*ast.ReturnStmt); ok {
				found = true
				return false
			}
			return true
		})
	}
	return found
}

func inspectFunctionBody(body *ast.BlockStmt, visit func(ast.Node)) {
	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		if node != nil {
			visit(node)
		}
		return true
	})
}

func calledObject(info *types.Info, call *ast.CallExpr) types.Object {
	switch fun := unparen(call.Fun).(type) {
	case *ast.Ident:
		return info.Uses[fun]
	case *ast.SelectorExpr:
		return info.Uses[fun.Sel]
	default:
		return nil
	}
}

func identObject(info *types.Info, ident *ast.Ident) types.Object {
	if object := info.Defs[ident]; object != nil {
		return object
	}
	return info.Uses[ident]
}

func isNonNilCheck(info *types.Info, expr ast.Expr, object types.Object) bool {
	binary, ok := unparen(expr).(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return false
	}
	return identIsObject(info, binary.X, object) && isNil(binary.Y) ||
		isNil(binary.X) && identIsObject(info, binary.Y, object)
}

func returnsNilAndObject(info *types.Info, body *ast.BlockStmt, object types.Object) bool {
	if len(body.List) == 0 {
		return false
	}
	stmt, ok := body.List[len(body.List)-1].(*ast.ReturnStmt)
	return ok && len(stmt.Results) == 2 && isNil(stmt.Results[0]) && identIsObject(info, stmt.Results[1], object)
}

func returnWrapsSessionObject(info *types.Info, stmt *ast.ReturnStmt, sessionType types.Type, object types.Object) bool {
	if len(stmt.Results) != 2 || !isNil(stmt.Results[1]) {
		return false
	}
	address, ok := unparen(stmt.Results[0]).(*ast.UnaryExpr)
	if !ok || address.Op != token.AND {
		return false
	}
	literal, ok := unparen(address.X).(*ast.CompositeLit)
	if !ok || !types.Identical(info.TypeOf(literal), sessionType) {
		return false
	}
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		name, ok := field.Key.(*ast.Ident)
		if ok && name.Name == "session" && identIsObject(info, field.Value, object) {
			return true
		}
	}
	return false
}

func identIsObject(info *types.Info, expr ast.Expr, object types.Object) bool {
	ident, ok := unparen(expr).(*ast.Ident)
	return ok && identObject(info, ident) == object
}

func isNil(expr ast.Expr) bool {
	ident, ok := unparen(expr).(*ast.Ident)
	return ok && ident.Name == "nil"
}

func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func TestRootRuntimeImportIsConfinedToEngineAndSession(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	for _, file := range productionRootFiles(t, root) {
		parsed, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		base := filepath.Base(file)
		if importsPath(parsed, "github.com/jacoelho/xsd/internal/runtime") && base != "compile.go" && base != "session.go" {
			t.Fatalf("%s imports internal/runtime implementation", file)
		}
	}
}

func TestRootDoesNotExposeOldPublicAPIs(t *testing.T) {
	root := repoRoot(t)
	forbidden := []string{
		"Error",
		"ErrSchemaNotFound",
		"Errors",
		"FormatOptions",
		"FormatXML",
		"IsUnsupported",
		"XMLFormatError",
	}
	fset := token.NewFileSet()
	for _, file := range productionRootFiles(t, root) {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil && slices.Contains(forbidden, decl.Name.Name) {
					t.Fatalf("%s exposes forbidden function %s", file, decl.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if slices.Contains(forbidden, spec.Name.Name) {
							t.Fatalf("%s exposes forbidden type %s", file, spec.Name.Name)
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if slices.Contains(forbidden, name.Name) {
								t.Fatalf("%s exposes forbidden value %s", file, name.Name)
							}
						}
					}
				}
			}
		}
	}
}

func TestRemovedPublicPackagesAbsent(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{"format", "schema", "xsdruntime"} {
		path := filepath.Join(root, dir)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			t.Fatalf("removed public package directory %s exists", dir)
		}
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", dir, err)
		}
	}
}

func productionRootFiles(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(root, name))
	}
	return files
}

func importsPath(file *ast.File, path string) bool {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == path {
			return true
		}
	}
	return false
}

func callsSelector(node ast.Node, receiver, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != name {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if ok && id.Name == receiver {
			found = true
		}
		return true
	})
	return found
}

func productionGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

func engineMethodDeclaration(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name && receiverTypeName(fn) == "Engine" {
			return fn
		}
	}
	return nil
}

func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return ""
	}
	typ := fn.Recv.List[0].Type
	if ptr, ok := typ.(*ast.StarExpr); ok {
		typ = ptr.X
	}
	ident, ok := typ.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func declaresFunction(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == name {
			return true
		}
	}
	return false
}
