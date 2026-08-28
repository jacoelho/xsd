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
	"strconv"
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
	var rootFiles []*ast.File
	var compileFile *ast.File
	var sourceFile *ast.File
	var nonSourceFacadeFiles []*ast.File
	for _, path := range productionRootFiles(t, root) {
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		rootFiles = append(rootFiles, parsed)
		if filepath.Base(path) == "compile.go" {
			compileFile = parsed
		}
		if filepath.Base(path) == "source.go" {
			sourceFile = parsed
		} else {
			nonSourceFacadeFiles = append(nonSourceFacadeFiles, parsed)
		}
		if filepath.Base(path) != "compile.go" && importsPath(parsed, "github.com/jacoelho/xsd/internal/compile") {
			t.Fatalf("%s imports compiler implementation outside compile.go", path)
		}
	}
	if compileFile == nil {
		t.Fatal("missing compile.go")
	}
	if !importsPath(compileFile, "github.com/jacoelho/xsd/internal/compile") {
		t.Fatal("compile.go does not import internal/compile")
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd", fset, rootFiles, info)
	if err != nil {
		t.Fatalf("type-check root compile facade: %v", err)
	}
	compilePkg := importedPackage(pkg, "github.com/jacoelho/xsd/internal/compile")
	if compilePkg == nil {
		t.Fatal("root facade does not import internal/compile")
	}
	compileCall := compilePkg.Scope().Lookup("CompileMappedSources")
	if uses := objectUseCount(info, compileCall); uses != 1 {
		t.Fatalf("root facade references compile.CompileMappedSources %d times, want exactly one", uses)
	}
	compileWithOptions := packageFunctionDeclaration(compileFile, "CompileWithOptions")
	if compileWithOptions == nil {
		t.Fatal("compile.go does not declare CompileWithOptions")
	}
	compile := packageFunctionDeclaration(compileFile, "Compile")
	compileWithOptionsObject := info.Defs[compileWithOptions.Name]
	if compile == nil || objectUseCount(info, compileWithOptionsObject) != 1 {
		t.Fatal("Compile does not have one delegation to CompileWithOptions")
	}
	compileDelegation := callToObject(info, compile.Body, compileWithOptionsObject)
	compileSources := functionParameterObject(info, compile, "sources")
	compileOptionsObject := pkg.Scope().Lookup("CompileOptions")
	if compileDelegation == nil || compileSources == nil || len(compileDelegation.Args) != 2 ||
		!emptyCompositeOfObject(info, compileDelegation.Args[0], compileOptionsObject) ||
		!expressionUsesObject(info, compileDelegation.Args[1], compileSources) || !compileDelegation.Ellipsis.IsValid() {
		t.Fatal("Compile does not pass empty options and original sources to CompileWithOptions")
	}
	compileInvocation := callToObject(info, compileWithOptions.Body, compileCall)
	if compileInvocation == nil {
		t.Fatal("CompileWithOptions does not delegate to compile.CompileMappedSources")
	}
	if sourceFile == nil {
		t.Fatal("missing source.go")
	}
	sourcesParameter := functionParameterObject(info, compileWithOptions, "sources")
	optionsParameter := functionParameterObject(info, compileWithOptions, "opts")
	mapper := packageFunctionDeclaration(sourceFile, "internalSchemaSource")
	optionsMapper := packageFunctionDeclaration(compileFile, "internalCompileOptions")
	optionsMapperParameter := functionParameterObject(info, optionsMapper, "opts")
	if sourcesParameter == nil || optionsParameter == nil || mapper == nil || optionsMapper == nil ||
		optionsMapperParameter == nil ||
		len(compileInvocation.Args) != 3 ||
		!callPassesObject(info, compileInvocation.Args[0], info.Defs[optionsMapper.Name], optionsParameter) ||
		!expressionUsesObject(info, compileInvocation.Args[1], sourcesParameter) ||
		!expressionUsesObject(info, compileInvocation.Args[2], info.Defs[mapper.Name]) {
		t.Fatal("CompileWithOptions does not pass the original options, sources, and mapper to compile.CompileMappedSources")
	}
	assertCompileOptionsMap(t, info, optionsMapper, optionsMapperParameter, pkg, compilePkg)
	for _, file := range nonSourceFacadeFiles {
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			selection := info.Selections[selector]
			if selection == nil || !isInternalSourceObject(selection.Obj()) {
				return true
			}
			t.Fatalf("root facade outside source.go references internal/source.%s", selection.Obj().Name())
			return false
		})
	}
	for _, decl := range compileFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if strings.HasPrefix(fn.Name.Name, "compile") && fn.Name.Name != "Compile" {
			t.Fatalf("root compile.go owns compiler helper %s", fn.Name.Name)
		}
	}
}

func objectUseCount(info *types.Info, expected types.Object) int {
	count := 0
	for _, obj := range info.Uses {
		if obj == expected {
			count++
		}
	}
	return count
}

func functionParameterObject(info *types.Info, fn *ast.FuncDecl, name string) types.Object {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return nil
	}
	for _, field := range fn.Type.Params.List {
		for _, ident := range field.Names {
			if ident.Name == name {
				return info.Defs[ident]
			}
		}
	}
	return nil
}

func expressionUsesObject(info *types.Info, expression ast.Expr, expected types.Object) bool {
	ident, ok := ast.Unparen(expression).(*ast.Ident)
	return ok && expected != nil && info.Uses[ident] == expected
}

func emptyCompositeOfObject(info *types.Info, expression ast.Expr, expected types.Object) bool {
	literal, ok := ast.Unparen(expression).(*ast.CompositeLit)
	if !ok || len(literal.Elts) != 0 {
		return false
	}
	typeName, ok := literal.Type.(*ast.Ident)
	return ok && expected != nil && info.Uses[typeName] == expected
}

func callPassesObject(info *types.Info, expression ast.Expr, expectedCall, expectedArgument types.Object) bool {
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	return ok && calledObject(info, call) == expectedCall && len(call.Args) == 1 &&
		expressionUsesObject(info, call.Args[0], expectedArgument)
}

func assertCompileOptionsMap(
	t *testing.T,
	info *types.Info,
	mapper *ast.FuncDecl,
	optionsParameter types.Object,
	rootPkg, compilePkg *types.Package,
) {
	t.Helper()
	publicObject := rootPkg.Scope().Lookup("CompileOptions")
	internalObject := compilePkg.Scope().Lookup("Options")
	if publicObject == nil || internalObject == nil || mapper.Body == nil || len(mapper.Body.List) != 1 {
		t.Fatal("compile option adapter types or body are missing")
	}
	publicType, publicOK := publicObject.Type().Underlying().(*types.Struct)
	if !publicOK {
		t.Fatal("CompileOptions is not a struct")
	}
	result, ok := mapper.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(result.Results) != 1 {
		t.Fatal("internalCompileOptions is not one direct return")
	}
	literal, ok := ast.Unparen(result.Results[0]).(*ast.CompositeLit)
	if !ok || len(literal.Elts) != publicType.NumFields() {
		t.Fatalf("internalCompileOptions maps %d fields, want %d", len(literal.Elts), publicType.NumFields())
	}
	typeSelector, ok := literal.Type.(*ast.SelectorExpr)
	if !ok || info.Uses[typeSelector.Sel] != internalObject {
		t.Fatal("internalCompileOptions does not return compile.Options directly")
	}
	seen := make(map[string]bool, publicType.NumFields())
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			t.Fatal("internalCompileOptions contains an unkeyed field")
		}
		key, keyOK := field.Key.(*ast.Ident)
		value, valueOK := ast.Unparen(field.Value).(*ast.SelectorExpr)
		if !keyOK || !valueOK {
			t.Fatal("internalCompileOptions field is not a direct field mapping")
		}
		base, baseOK := value.X.(*ast.Ident)
		selection := info.Selections[value]
		internalField := info.Uses[key]
		if !baseOK || selection == nil || info.Uses[base] != optionsParameter {
			t.Fatalf("internal option %s is not sourced directly from opts", key.Name)
		}
		if internalField == nil || selection.Obj().Name() != key.Name || internalField.Name() != key.Name {
			t.Fatalf("internal option %s is mapped from public option %s", key.Name, selection.Obj().Name())
		}
		if !types.Identical(selection.Obj().Type(), internalField.Type()) {
			t.Fatalf("compile option %s changes type during adaptation", key.Name)
		}
		if seen[key.Name] {
			t.Fatalf("compile option %s is mapped more than once", key.Name)
		}
		seen[key.Name] = true
	}
	for field := range publicType.Fields() {
		if !seen[field.Name()] {
			t.Fatalf("public compile option %s is not mapped", field.Name())
		}
	}
}

func isInternalSourceObject(obj types.Object) bool {
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "github.com/jacoelho/xsd/internal/source"
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
		Uses: make(map[*ast.Ident]types.Object),
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
	if validateCall == nil || newSessionCall == nil {
		t.Fatal("validation facade types are incomplete")
	}
	validateWithOptions := engineMethodDeclaration(public, "ValidateWithOptions")
	if validateWithOptions == nil || !callsObject(info, validateWithOptions.Body, validateCall) {
		t.Fatal("Engine.ValidateWithOptions does not call internal/validate.Validate")
	}
	newSession := engineMethodDeclaration(public, "NewSession")
	if newSession == nil || !callsObject(info, newSession.Body, newSessionCall) {
		t.Fatal("Engine.NewSession does not call internal/validate.NewSession")
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

func callsObject(info *types.Info, node ast.Node, expected types.Object) bool {
	return callToObject(info, node, expected) != nil
}

func callToObject(info *types.Info, node ast.Node, expected types.Object) *ast.CallExpr {
	if node == nil || expected == nil {
		return nil
	}
	var found *ast.CallExpr
	ast.Inspect(node, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if ok && calledObject(info, call) == expected {
			found = call
			return false
		}
		return true
	})
	return found
}

func calledObject(info *types.Info, call *ast.CallExpr) types.Object {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		if obj := info.Uses[fun.Sel]; obj != nil {
			return obj
		}
		if selection := info.Selections[fun]; selection != nil {
			return selection.Obj()
		}
		return nil
	case *ast.Ident:
		return info.Uses[fun]
	default:
		return nil
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
		if imported, ok := goImportPath(imp); ok && imported == path {
			return true
		}
	}
	return false
}

func goImportPath(imp *ast.ImportSpec) (string, bool) {
	path, err := strconv.Unquote(imp.Path.Value)
	return path, err == nil
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

func packageFunctionDeclaration(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == name {
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
