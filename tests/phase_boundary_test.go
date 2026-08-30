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

func TestRootCompileHasSingleInternalExecutionEdge(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var rootFiles []*ast.File
	compileImporters := 0
	for _, path := range productionRootFiles(t, root) {
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		rootFiles = append(rootFiles, parsed)
		if importsPath(parsed, "github.com/jacoelho/xsd/internal/compile") {
			compileImporters++
		}
	}
	if compileImporters != 1 {
		t.Fatalf("root files importing internal/compile = %d, want 1", compileImporters)
	}
	info := &types.Info{
		Uses: make(map[*ast.Ident]types.Object),
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
	expected := compilePkg.Scope().Lookup("CompileMappedSources")
	var compileCallables []types.Object
	for _, object := range info.Uses {
		owner := object.Pkg()
		_, callable := object.Type().Underlying().(*types.Signature)
		if owner != nil && owner.Path() == compilePkg.Path() && callable {
			compileCallables = append(compileCallables, object)
		}
	}
	if len(compileCallables) != 1 {
		t.Fatalf("root facade uses internal/compile callables %d times, want exactly one", len(compileCallables))
	}
	if compileCallables[0] != expected {
		t.Fatalf("sole internal/compile callable targets %s, want CompileMappedSources", compileCallables[0].Name())
	}
	directCalls := 0
	for _, file := range rootFiles {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && calledObject(info, call) == expected {
				directCalls++
			}
			return true
		})
	}
	if directCalls != 1 {
		t.Fatalf("root facade directly calls compile.CompileMappedSources %d times, want exactly one", directCalls)
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
