package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinRegistryCacheMutationBoundary(t *testing.T) {
	t.Parallel()

	const allowedMutationFile = "internal/types/builtin.go"
	mutationFields := map[string]struct{}{
		"fundamentalFacets": {},
	}

	root := repoRoot(t)
	typesDir := filepath.Join(root, "internal", "types")
	fset := token.NewFileSet()

	err := filepath.WalkDir(typesDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		isAllowedFile := filepath.Clean(relPath) == filepath.Clean(allowedMutationFile)
		ast.Inspect(file, func(node ast.Node) bool {
			typed, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range typed.Lhs {
				selector, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if _, ok := mutationFields[selector.Sel.Name]; !ok {
					continue
				}
				if isAllowedFile {
					continue
				}
				pos := fset.Position(selector.Sel.Pos())
				t.Fatalf("builtin cache mutation boundary violation: %s writes %q at %s", relPath, selector.Sel.Name, pos)
			}
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("scan internal/types files: %v", err)
	}
}

func TestBuiltinRegistryAccessorsAreFunctions(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	path := filepath.Join(root, "internal", "types", "builtin.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse builtin.go: %v", err)
	}

	sawGetBuiltin := false
	sawGetBuiltinNS := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		switch fn.Name.Name {
		case "GetBuiltin":
			sawGetBuiltin = true
		case "GetBuiltinNS":
			sawGetBuiltinNS = true
		}
	}

	if !sawGetBuiltin {
		t.Fatal("builtin registry boundary violation: GetBuiltin must be a function")
	}
	if !sawGetBuiltinNS {
		t.Fatal("builtin registry boundary violation: GetBuiltinNS must be a function")
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range valueSpec.Names {
				if name.Name == "GetBuiltin" || name.Name == "GetBuiltinNS" {
					t.Fatalf("builtin registry boundary violation: %s must not be a package var", name.Name)
				}
			}
		}
	}
}
