package archtest_test

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeSchemaProductionCodeUsesAccessors(t *testing.T) {
	pkgs := listGoPackages(t)
	fields := runtimeSchemaDataFields()
	runtimePkg := internalPkg("runtime")

	for _, pkg := range pkgs {
		pkg := pkg
		if len(pkg.GoFiles) == 0 || hasPkgPrefix(pkg.ImportPath, runtimePkg) {
			continue
		}
		t.Run(strings.TrimPrefix(pkg.ImportPath, modulePath+"/"), func(t *testing.T) {
			t.Parallel()
			checkRuntimeSchemaAccess(t, pkg.ImportPath, pkg, pkg.GoFiles, pkg.GoFiles, fields)
		})
	}
}

func TestRuntimeSchemaTestsUseAccessorsOrBuilders(t *testing.T) {
	pkgs := listGoPackages(t)
	fields := runtimeSchemaDataFields()
	runtimePkg := internalPkg("runtime")

	for _, pkg := range pkgs {
		pkg := pkg
		if hasPkgPrefix(pkg.ImportPath, runtimePkg) {
			continue
		}
		if len(pkg.TestGoFiles) != 0 {
			t.Run(strings.TrimPrefix(pkg.ImportPath, modulePath+"/"), func(t *testing.T) {
				t.Parallel()
				typeFiles := append([]string{}, pkg.GoFiles...)
				typeFiles = append(typeFiles, pkg.TestGoFiles...)
				checkRuntimeSchemaAccess(t, pkg.ImportPath, pkg, typeFiles, pkg.TestGoFiles, fields)
			})
		}
		if len(pkg.XTestGoFiles) != 0 {
			t.Run(strings.TrimPrefix(pkg.ImportPath, modulePath+"/")+"_test", func(t *testing.T) {
				t.Parallel()
				checkRuntimeSchemaAccess(t, pkg.ImportPath+"_test", pkg, pkg.XTestGoFiles, pkg.XTestGoFiles, fields)
			})
		}
	}
}

func TestRuntimeSchemaHasNoExportedDataFields(t *testing.T) {
	pkg := goPackageByImportPath(t, internalPkg("runtime"))
	files, _, fset := parsePackageGoFiles(t, pkg, pkg.GoFiles, nil)
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	checked, err := conf.Check(pkg.ImportPath, fset, files, nil)
	if err != nil {
		t.Fatalf("type-check %s: %v", pkg.ImportPath, err)
	}
	obj := checked.Scope().Lookup("Schema")
	if obj == nil {
		t.Fatalf("runtime.Schema not found")
	}
	named, ok := types.Unalias(obj.Type()).(*types.Named)
	if !ok {
		t.Fatalf("runtime.Schema has type %T, want named type", obj.Type())
	}
	strct, ok := named.Underlying().(*types.Struct)
	if !ok {
		t.Fatalf("runtime.Schema has underlying type %T, want struct", named.Underlying())
	}
	for i := range strct.NumFields() {
		field := strct.Field(i)
		if field.Exported() {
			t.Errorf("runtime.Schema.%s is an exported data field", field.Name())
		}
	}
}

func TestRuntimeSchemaInternalStorageAccessIsConfined(t *testing.T) {
	pkg := goPackageByImportPath(t, internalPkg("runtime"))
	fields := runtimeSchemaDataFields()

	productionAllowed := map[string]struct{}{
		"internal/runtime/assembler.go":        {},
		"internal/runtime/schema_accessors.go": {},
	}
	assertRuntimeSchemaAccessesAllowed(t,
		runtimeSchemaAccesses(t, pkg.ImportPath, pkg, pkg.GoFiles, pkg.GoFiles, fields),
		productionAllowed,
	)

	testAllowed := map[string]struct{}{
		"internal/runtime/assembler_test.go":        {},
		"internal/runtime/builder_test.go":          {},
		"internal/runtime/schema_accessors_test.go": {},
		"internal/runtime/session_plan_test.go":     {},
		"internal/runtime/wildcards_match_test.go":  {},
	}
	typeFiles := append([]string{}, pkg.GoFiles...)
	typeFiles = append(typeFiles, pkg.TestGoFiles...)
	assertRuntimeSchemaAccessesAllowed(t,
		runtimeSchemaAccesses(t, pkg.ImportPath, pkg, typeFiles, pkg.TestGoFiles, fields),
		testAllowed,
	)
}

type goPackage struct {
	ImportPath   string
	Dir          string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
}

func listGoPackages(t *testing.T) []goPackage {
	t.Helper()

	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list ./...: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("go list ./...: %v", err)
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []goPackage
	for {
		var pkg goPackage
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

func checkRuntimeSchemaAccess(t *testing.T, importPath string, pkg goPackage, typeFileNames, inspectFileNames []string, fields map[string]struct{}) {
	t.Helper()

	for _, access := range runtimeSchemaAccesses(t, importPath, pkg, typeFileNames, inspectFileNames, fields) {
		t.Errorf("%s %s", access.pos, access.description())
	}
}

type runtimeSchemaAccess struct {
	pos   token.Position
	file  string
	field string
}

func (a runtimeSchemaAccess) description() string {
	if a.field == "" {
		return "constructs runtime.Schema literal directly"
	}
	return "accesses runtime.Schema." + a.field + " directly"
}

func runtimeSchemaAccesses(t *testing.T, importPath string, pkg goPackage, typeFileNames, inspectFileNames []string, fields map[string]struct{}) []runtimeSchemaAccess {
	t.Helper()

	if !potentialRuntimeSchemaAccess(t, pkg, inspectFileNames, fields) {
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

	var accesses []runtimeSchemaAccess
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
				if isRuntimeSchemaType(sel.Recv()) {
					pos := fset.Position(n.Pos())
					accesses = append(accesses, runtimeSchemaAccess{
						pos:   pos,
						file:  repoRelPath(t, pos.Filename),
						field: n.Sel.Name,
					})
				}
			case *ast.CompositeLit:
				if isRuntimeSchemaType(info.TypeOf(n)) {
					pos := fset.Position(n.Pos())
					accesses = append(accesses, runtimeSchemaAccess{
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

func potentialRuntimeSchemaAccess(t *testing.T, pkg goPackage, fileNames []string, fields map[string]struct{}) bool {
	t.Helper()

	for _, name := range fileNames {
		data, err := os.ReadFile(filepath.Join(pkg.Dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		content := string(data)
		if strings.Contains(content, "runtime.Schema") || strings.Contains(content, "Schema{") {
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

func assertRuntimeSchemaAccessesAllowed(t *testing.T, accesses []runtimeSchemaAccess, allowed map[string]struct{}) {
	t.Helper()
	for _, access := range accesses {
		if _, ok := allowed[access.file]; ok {
			continue
		}
		t.Errorf("%s %s outside approved file", access.pos, access.description())
	}
}

func goPackageByImportPath(t *testing.T, importPath string) goPackage {
	t.Helper()
	for _, pkg := range listGoPackages(t) {
		if pkg.ImportPath == importPath {
			return pkg
		}
	}
	t.Fatalf("package %s not found", importPath)
	return goPackage{}
}

func repoRelPath(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(repoRoot(t), path)
	if err != nil {
		t.Fatalf("make %s relative to repo root: %v", path, err)
	}
	return filepath.ToSlash(rel)
}

func parsePackageGoFiles(t *testing.T, pkg goPackage, typeFileNames, inspectFileNames []string) ([]*ast.File, []*ast.File, *token.FileSet) {
	t.Helper()

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(typeFileNames))
	inspect := make([]*ast.File, 0, len(inspectFileNames))
	inspectSet := make(map[string]struct{}, len(inspectFileNames))
	for _, name := range inspectFileNames {
		inspectSet[name] = struct{}{}
	}
	for _, name := range typeFileNames {
		path := filepath.Join(pkg.Dir, name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, file)
		if _, ok := inspectSet[name]; ok {
			inspect = append(inspect, file)
		}
	}
	return files, inspect, fset
}

func runtimeSchemaDataFields() map[string]struct{} {
	return map[string]struct{}{
		"Symbols":          {},
		"Namespaces":       {},
		"GlobalTypes":      {},
		"GlobalElements":   {},
		"GlobalAttributes": {},
		"Types":            {},
		"Ancestors":        {},
		"ComplexTypes":     {},
		"Elements":         {},
		"Attributes":       {},
		"AttrIndex":        {},
		"Validators":       {},
		"Facets":           {},
		"Patterns":         {},
		"Enums":            {},
		"Values":           {},
		"Notations":        {},
		"Models":           {},
		"Wildcards":        {},
		"WildcardNS":       {},
		"ICs":              {},
		"ElemICs":          {},
		"ICSelectors":      {},
		"ICFields":         {},
		"Paths":            {},
		"Predef":           {},
		"PredefNS":         {},
		"Builtin":          {},
		"RootPolicy":       {},
		"BuildHash":        {},

		"symbols":                    {},
		"namespaces":                 {},
		"globalTypes":                {},
		"globalElements":             {},
		"globalAttributes":           {},
		"types":                      {},
		"ancestors":                  {},
		"complexTypes":               {},
		"elements":                   {},
		"attributes":                 {},
		"attrIndex":                  {},
		"validators":                 {},
		"facets":                     {},
		"patterns":                   {},
		"enums":                      {},
		"values":                     {},
		"notations":                  {},
		"models":                     {},
		"wildcards":                  {},
		"wildcardNS":                 {},
		"identityConstraints":        {},
		"elementIdentityConstraints": {},
		"identitySelectors":          {},
		"identityFields":             {},
		"paths":                      {},
		"predef":                     {},
		"predefNS":                   {},
		"builtin":                    {},
		"rootPolicy":                 {},
		"buildHash":                  {},
	}
}

func isRuntimeSchemaType(typ types.Type) bool {
	typ = types.Unalias(typ)
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(ptr.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "Schema" && obj.Pkg() != nil && obj.Pkg().Path() == internalPkg("runtime")
}
