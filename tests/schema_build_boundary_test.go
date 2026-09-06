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

// The schema package owns both compilation and publication. Its mutable
// topology is deliberately unexported so callers can only obtain a sealed
// Schema through the package's compile entrypoint.
func TestSchemaBuildIsPrivate(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "schema")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		files = append(files, file)
	}

	info := &types.Info{}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/jacoelho/xsd/internal/schema", fset, files, info)
	if err != nil {
		t.Fatalf("type-check internal/schema: %v", err)
	}
	if pkg.Scope().Lookup("SchemaBuild") != nil {
		t.Fatal("internal/schema exports mutable SchemaBuild")
	}
	build := pkg.Scope().Lookup("schemaBuild")
	if build == nil {
		t.Fatal("internal/schema does not declare private schemaBuild")
	}

	schema := pkg.Scope().Lookup("Schema")
	schemaType, ok := schema.Type().Underlying().(*types.Struct)
	if !ok {
		t.Fatal("schema.Schema is not a struct")
	}
	program := fieldByName(schemaType, "program")
	if program == nil || program.Exported() {
		t.Fatal("schema.Schema must retain its published program privately")
	}
}

func fieldByName(structType *types.Struct, name string) *types.Var {
	for field := range structType.Fields() {
		if field.Name() == name {
			return field
		}
	}
	return nil
}
