package tests_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerSchemaBuildTopologyHasOneOwner(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal/compile")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		files = append(files, file)
	}

	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd/internal/compile", fset, files, info)
	if err != nil {
		t.Fatalf("type-check internal/compile: %v", err)
	}

	compilerObject := pkg.Scope().Lookup("compiler")
	compilerType, ok := compilerObject.Type().Underlying().(*types.Struct)
	if !ok {
		t.Fatal("compile.compiler is not a struct")
	}
	rtField := fieldByName(compilerType, "rt")
	if rtField == nil || typeName(rtField.Type()) != "compilerSchemaBuild" {
		t.Fatalf("compiler.rt type = %v, want compilerSchemaBuild", rtField)
	}
	buildObject := pkg.Scope().Lookup("compilerSchemaBuild")
	buildType, ok := buildObject.Type().(*types.Named)
	if !ok {
		t.Fatal("compile.compilerSchemaBuild is not a named type")
	}
	buildStruct, ok := buildType.Underlying().(*types.Struct)
	if !ok || buildStruct.NumFields() != 1 {
		t.Fatalf("compilerSchemaBuild fields = %v, want one private build field", buildStruct)
	}
	buildField := buildStruct.Field(0)
	fieldType, ok := buildField.Type().(*types.Named)
	if !ok || buildField.Name() != "build" || buildField.Exported() || fieldType.Obj().Name() != "SchemaBuild" ||
		fieldType.Obj().Pkg().Path() != "github.com/jacoelho/xsd/internal/runtime" {
		t.Fatalf("compilerSchemaBuild field = %s %v, want private runtime.SchemaBuild", buildField.Name(), buildField.Type())
	}
	for method := range types.NewMethodSet(types.NewPointer(buildType)).Methods() {
		if forbiddenSchemaBuildAccessor(method.Obj().Name()) {
			t.Fatalf("compilerSchemaBuild exposes mutable schema aggregate through %s", method.Obj().Name())
		}
		signature, ok := method.Obj().Type().(*types.Signature)
		if !ok {
			continue
		}
		for result := range signature.Results().Variables() {
			if mutableSchemaBuildResult(result.Type()) && method.Obj().Name() != "attributeUsesAndWildcard" {
				t.Fatalf("compilerSchemaBuild.%s returns mutable state %v", method.Obj().Name(), result.Type())
			}
		}
	}

	owner := filepath.Join(dir, "schema_build.go")
	for _, file := range files {
		if fset.Position(file.Pos()).Filename == owner {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				selection := info.Selections[node]
				if selection != nil && selection.Obj().Name() == "build" && typeName(selection.Recv()) == "compilerSchemaBuild" {
					t.Errorf("%s accesses compiler schema topology outside schema_build.go", fset.Position(node.Pos()))
				}
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if isCompilerRTSelector(info, lhs) {
						t.Errorf("%s replaces compiler schema owner outside schema_build.go", fset.Position(lhs.Pos()))
					}
				}
			}
			return true
		})
	}
}

func forbiddenSchemaBuildAccessor(name string) bool {
	switch name {
	case "simpleType", "element", "attribute", "attributeUseSet", "identity", "newNameInterner":
		return true
	default:
		return false
	}
}

func mutableSchemaBuildResult(typ types.Type) bool {
	if types.Identical(typ, types.Universe.Lookup("error").Type()) {
		return false
	}
	typ = types.Unalias(typ)
	if named, ok := typ.(*types.Named); ok {
		typ = named.Underlying()
	}
	switch typ.(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		return true
	default:
		return false
	}
}

func isCompilerRTSelector(info *types.Info, expr ast.Expr) bool {
	selector, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	selection := info.Selections[selector]
	return selection != nil && selection.Obj().Name() == "rt" && typeName(selection.Recv()) == "compiler"
}

func fieldByName(structType *types.Struct, name string) *types.Var {
	for field := range structType.Fields() {
		if field.Name() == name {
			return field
		}
	}
	return nil
}

func typeName(typ types.Type) string {
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj() == nil {
		return ""
	}
	return named.Obj().Name()
}
