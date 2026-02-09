package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestSemanticRegistryDoesNotExposePointerKeyMaps(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	path := filepath.Join(root, "internal", "semantic", "registry.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse registry.go: %v", err)
	}

	var registryStruct *ast.StructType
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil || typeSpec.Name.Name != "Registry" {
				continue
			}
			if st, ok := typeSpec.Type.(*ast.StructType); ok {
				registryStruct = st
				break
			}
		}
		if registryStruct != nil {
			break
		}
	}
	if registryStruct == nil {
		t.Fatal("semantic registry boundary violation: type Registry not found")
	}

	for _, field := range registryStruct.Fields.List {
		for _, name := range field.Names {
			if name == nil || !name.IsExported() {
				continue
			}
			if name.Name == "AnonymousTypes" || name.Name == "LocalElements" || name.Name == "LocalAttributes" {
				t.Fatalf("semantic registry boundary violation: exported identity map field %s", name.Name)
			}
		}

		mapType, ok := field.Type.(*ast.MapType)
		if !ok {
			continue
		}
		if _, ok := mapType.Key.(*ast.StarExpr); !ok {
			continue
		}
		for _, name := range field.Names {
			if name == nil {
				continue
			}
			if !name.IsExported() {
				continue
			}
			t.Fatalf("semantic registry boundary violation: exported pointer-key map field %s", name.Name)
		}
	}
}
