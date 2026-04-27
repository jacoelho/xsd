package archtest_test

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaIRValueConstraintHasNoExportedDataFields(t *testing.T) {
	pkg := goPackageByImportPath(t, internalPkg("schemair"))
	files, _, fset := parsePackageGoFiles(t, pkg, pkg.GoFiles, nil)
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	checked, err := conf.Check(pkg.ImportPath, fset, files, nil)
	if err != nil {
		t.Fatalf("type-check %s: %v", pkg.ImportPath, err)
	}
	obj := checked.Scope().Lookup("ValueConstraint")
	if obj == nil {
		t.Fatalf("schemair.ValueConstraint not found")
	}
	named, ok := types.Unalias(obj.Type()).(*types.Named)
	if !ok {
		t.Fatalf("schemair.ValueConstraint has type %T, want named type", obj.Type())
	}
	strct, ok := named.Underlying().(*types.Struct)
	if !ok {
		t.Fatalf("schemair.ValueConstraint has underlying type %T, want struct", named.Underlying())
	}
	for i := range strct.NumFields() {
		field := strct.Field(i)
		if field.Exported() {
			t.Errorf("schemair.ValueConstraint.%s is an exported data field", field.Name())
		}
	}
}

func TestSchemaIRValueConstraintStorageAccessIsConfined(t *testing.T) {
	pkgs := listGoPackages(t)
	fields := map[string]struct{}{
		"Present": {},
		"Lexical": {},
		"Context": {},
		"present": {},
		"lexical": {},
		"context": {},
	}
	allowed := map[string]struct{}{
		"internal/schemair/types.go":      {},
		"internal/schemair/types_test.go": {},
	}

	for _, pkg := range pkgs {
		pkg := pkg
		if len(pkg.GoFiles) != 0 {
			t.Run(strings.TrimPrefix(pkg.ImportPath, modulePath+"/"), func(t *testing.T) {
				t.Parallel()
				assertValueConstraintAccessesAllowed(t,
					valueConstraintAccesses(t, pkg.ImportPath, pkg, pkg.GoFiles, pkg.GoFiles, fields),
					allowed,
				)
			})
		}
		if len(pkg.TestGoFiles) != 0 {
			t.Run(strings.TrimPrefix(pkg.ImportPath, modulePath+"/")+"_tests", func(t *testing.T) {
				t.Parallel()
				typeFiles := append([]string{}, pkg.GoFiles...)
				typeFiles = append(typeFiles, pkg.TestGoFiles...)
				assertValueConstraintAccessesAllowed(t,
					valueConstraintAccesses(t, pkg.ImportPath, pkg, typeFiles, pkg.TestGoFiles, fields),
					allowed,
				)
			})
		}
		if len(pkg.XTestGoFiles) != 0 {
			t.Run(strings.TrimPrefix(pkg.ImportPath, modulePath+"/")+"_xtests", func(t *testing.T) {
				t.Parallel()
				assertValueConstraintAccessesAllowed(t,
					valueConstraintAccesses(t, pkg.ImportPath+"_test", pkg, pkg.XTestGoFiles, pkg.XTestGoFiles, fields),
					allowed,
				)
			})
		}
	}
}

type valueConstraintAccess struct {
	pos   token.Position
	file  string
	field string
}

func (a valueConstraintAccess) description() string {
	if a.field == "" {
		return "constructs schemair.ValueConstraint literal directly"
	}
	return "accesses schemair.ValueConstraint." + a.field + " directly"
}

func valueConstraintAccesses(t *testing.T, importPath string, pkg goPackage, typeFileNames, inspectFileNames []string, fields map[string]struct{}) []valueConstraintAccess {
	t.Helper()

	if !potentialValueConstraintAccess(t, pkg, inspectFileNames, fields) {
		return nil
	}

	files, inspectFiles, fset := parsePackageGoFiles(t, pkg, typeFileNames, inspectFileNames)
	info := &types.Info{
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Types:      make(map[ast.Expr]types.TypeAndValue),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check(importPath, fset, files, info); err != nil {
		t.Fatalf("type-check %s: %v", importPath, err)
	}

	var accesses []valueConstraintAccess
	for _, file := range inspectFiles {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if _, ok := fields[n.Sel.Name]; !ok {
					return true
				}
				sel := info.Selections[n]
				if sel == nil || sel.Kind() != types.FieldVal {
					return true
				}
				if isSchemaIRValueConstraintType(sel.Recv()) {
					pos := fset.Position(n.Pos())
					accesses = append(accesses, valueConstraintAccess{
						pos:   pos,
						file:  repoRelPath(t, pos.Filename),
						field: n.Sel.Name,
					})
				}
			case *ast.CompositeLit:
				if isSchemaIRValueConstraintType(info.TypeOf(n)) {
					pos := fset.Position(n.Pos())
					accesses = append(accesses, valueConstraintAccess{
						pos:  pos,
						file: repoRelPath(t, pos.Filename),
					})
				}
			}
			return true
		})
	}
	return accesses
}

func potentialValueConstraintAccess(t *testing.T, pkg goPackage, fileNames []string, fields map[string]struct{}) bool {
	t.Helper()

	for _, name := range fileNames {
		data, err := os.ReadFile(filepath.Join(pkg.Dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		content := string(data)
		if strings.Contains(content, "ValueConstraint") {
			return true
		}
		for field := range fields {
			if strings.Contains(content, "."+field) {
				return true
			}
		}
	}
	return false
}

func assertValueConstraintAccessesAllowed(t *testing.T, accesses []valueConstraintAccess, allowed map[string]struct{}) {
	t.Helper()
	for _, access := range accesses {
		if _, ok := allowed[access.file]; ok {
			continue
		}
		t.Errorf("%s %s outside approved file", access.pos, access.description())
	}
}

func isSchemaIRValueConstraintType(typ types.Type) bool {
	typ = types.Unalias(typ)
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(ptr.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "ValueConstraint" && obj.Pkg() != nil && obj.Pkg().Path() == internalPkg("schemair")
}
