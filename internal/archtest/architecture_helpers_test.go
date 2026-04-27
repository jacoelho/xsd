package archtest_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/jacoelho/xsd"

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repository root with go.mod not found from %s", dir)
		}
		dir = parent
	}
}

func internalPkg(name string) string {
	return modulePath + "/internal/" + strings.TrimPrefix(name, "/")
}

func hasPkgPrefix(pkg, prefix string) bool {
	return pkg == prefix || strings.HasPrefix(pkg, prefix+"/")
}

func collectPackageImports(t *testing.T) map[string]map[string]struct{} {
	t.Helper()
	return collectPackageImportsForFiles(t, false)
}

func collectPackageImportsWithTests(t *testing.T) map[string]map[string]struct{} {
	t.Helper()
	return collectPackageImportsForFiles(t, true)
}

func collectPackageImportsForFiles(t *testing.T, includeTests bool) map[string]map[string]struct{} {
	t.Helper()

	root := repoRoot(t)
	internalRoot := filepath.Join(root, "internal")

	graph := make(map[string]map[string]struct{})
	fset := token.NewFileSet()

	err := filepath.WalkDir(internalRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		var files []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			if !includeTests && strings.HasSuffix(name, "_test.go") {
				continue
			}
			files = append(files, filepath.Join(path, name))
		}
		if len(files) == 0 {
			return nil
		}

		relDir, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		importPath := modulePath + "/" + filepath.ToSlash(relDir)
		imports := graph[importPath]
		if imports == nil {
			imports = make(map[string]struct{})
			graph[importPath] = imports
		}

		for _, file := range files {
			node, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range node.Imports {
				pathValue, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				imports[pathValue] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collect package imports: %v", err)
	}

	return graph
}

func collectRootExports(t *testing.T) map[string]struct{} {
	t.Helper()

	root := repoRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read repo root: %v", err)
	}

	exports := make(map[string]struct{})
	fset := token.NewFileSet()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(root, name)
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if node.Name.Name != "xsd" {
			continue
		}

		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(s.Name.Name) {
							exports["type "+s.Name.Name] = struct{}{}
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if !ast.IsExported(n.Name) {
								continue
							}
							switch d.Tok {
							case token.CONST:
								exports["const "+n.Name] = struct{}{}
							case token.VAR:
								exports["var "+n.Name] = struct{}{}
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil {
					if ast.IsExported(d.Name.Name) {
						exports["func "+d.Name.Name] = struct{}{}
					}
					continue
				}
				if !ast.IsExported(d.Name.Name) {
					continue
				}
				recvName := receiverTypeName(d.Recv.List[0].Type)
				if recvName == "" || !ast.IsExported(recvName) {
					continue
				}
				exports["method "+recvName+"."+d.Name.Name] = struct{}{}
			}
		}
	}
	pkg := typeCheckRootPackage(t, fset)
	for name := range rootSurfaceTypes() {
		obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			t.Fatalf("root type %s not found in type-checked package", name)
		}
		collectTypeSurface(exports, name, obj.Type())
	}
	for alias := range collectRootInternalAliases(t) {
		obj, ok := pkg.Scope().Lookup(alias).(*types.TypeName)
		if !ok {
			t.Fatalf("root alias %s not found in type-checked package", alias)
		}
		collectTypeSurface(exports, alias, obj.Type())
	}

	return exports
}

func rootSurfaceTypes() map[string]struct{} {
	return map[string]struct{}{
		"Error":          {},
		"Validation":     {},
		"ValidationList": {},
	}
}

func collectRootInternalAliases(t *testing.T) map[string]struct{} {
	t.Helper()

	root := repoRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read repo root: %v", err)
	}

	aliases := make(map[string]struct{})
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(root, name)
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if node.Name.Name != "xsd" {
			continue
		}
		imports := importNames(node)
		for _, decl := range node.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Assign.IsValid() || !ast.IsExported(typeSpec.Name.Name) {
					continue
				}
				if selectorImportsInternal(typeSpec.Type, imports) {
					aliases[typeSpec.Name.Name] = struct{}{}
				}
			}
		}
	}
	return aliases
}

func importNames(node *ast.File) map[string]string {
	imports := make(map[string]string)
	for _, imp := range node.Imports {
		pathValue, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(pathValue)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = pathValue
	}
	return imports
}

func selectorImportsInternal(expr ast.Expr, imports map[string]string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return strings.Contains(imports[pkg.Name], "/internal/")
}

func typeCheckRootPackage(t *testing.T, fset *token.FileSet) *types.Package {
	t.Helper()

	root := repoRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read repo root: %v", err)
	}
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(root, name)
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if node.Name.Name == "xsd" {
			files = append(files, node)
		}
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check(modulePath, fset, files, nil)
	if err != nil {
		t.Fatalf("type-check root package: %v", err)
	}
	return pkg
}

func collectTypeSurface(exports map[string]struct{}, name string, typ types.Type) {
	unalias := types.Unalias(typ)
	if st, ok := unalias.Underlying().(*types.Struct); ok {
		for i := range st.NumFields() {
			field := st.Field(i)
			if field.Exported() {
				exports["field "+name+"."+field.Name()] = struct{}{}
			}
		}
	}
	collectMethodSet(exports, name, typ)
	collectMethodSet(exports, name, types.NewPointer(typ))
}

func collectMethodSet(exports map[string]struct{}, name string, typ types.Type) {
	methods := types.NewMethodSet(typ)
	for i := range methods.Len() {
		method := methods.At(i).Obj()
		if method.Exported() {
			exports["method "+name+"."+method.Name()] = struct{}{}
		}
	}
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}
