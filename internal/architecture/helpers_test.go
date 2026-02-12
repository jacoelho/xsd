package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
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
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
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

	return exports
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
