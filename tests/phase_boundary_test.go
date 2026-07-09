package tests_test

import (
	"go/ast"
	"go/parser"
	"go/token"
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

func callsSelector(file *ast.File, receiver, name string) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
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
