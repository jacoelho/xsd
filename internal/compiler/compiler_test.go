package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/types"
)

func parseW3CSchema(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schemaPath := filepath.Join("..", "..", "testdata", "xsdtests", filepath.FromSlash(relPath))
	file, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("open schema %s: %v", schemaPath, err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close schema %s: %v", schemaPath, closeErr)
		}
	})

	schema, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("parse schema %s: %v", schemaPath, err)
	}
	return schema
}

func loadW3CSchema(t *testing.T, relDir, entry string) *parser.Schema {
	t.Helper()

	dir := filepath.Join("..", "..", "testdata", "xsdtests", filepath.FromSlash(relDir))
	fsys := os.DirFS(dir)
	l := loader.NewLoader(loader.Config{FS: fsys})
	schema, err := l.Load(entry)
	if err != nil {
		t.Fatalf("load schema %s/%s: %v", relDir, entry, err)
	}
	res := resolver.NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema %s/%s: %v", relDir, entry, err)
	}
	if errs := resolver.ValidateReferences(schema); len(errs) > 0 {
		t.Fatalf("validate references %s/%s: %v", relDir, entry, errs[0])
	}
	return schema
}

func TestCompileW3CIPO3SubstitutionGroups(t *testing.T) {
	schema := loadW3CSchema(t, "boeingData/ipo3", "ipo.xsd")

	compiled, err := compiler.NewCompiler(schema).Compile()
	if err != nil {
		t.Fatalf("compile ipo3: %v", err)
	}

	headQName := types.QName{Namespace: schema.TargetNamespace, Local: "comment"}
	subs := compiled.SubstitutionGroups[headQName]
	if len(subs) != 2 {
		t.Fatalf("expected 2 substitution group members, got %d", len(subs))
	}

	found := make(map[string]bool)
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		found[sub.QName.Local] = true
	}
	for _, name := range []string{"shipComment", "customerComment"} {
		if !found[name] {
			t.Fatalf("expected substitution group member %q", name)
		}
	}

	itemsQName := types.QName{Namespace: schema.TargetNamespace, Local: "ItemsType"}
	items := compiled.Types[itemsQName]
	if items == nil {
		t.Fatalf("expected compiled ItemsType")
	}
	if !items.Mixed {
		t.Fatalf("expected ItemsType to be mixed")
	}
	if items.ContentModel == nil || items.ContentModel.Empty {
		t.Fatalf("expected ItemsType content model")
	}
}

func TestCompileW3CGroupAndAttributeGroup(t *testing.T) {
	schema := parseW3CSchema(t, "sunData/combined/xsd024/xsd024.xsdmod")
	res := resolver.NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve xsd024: %v", err)
	}
	if errs := resolver.ValidateReferences(schema); len(errs) > 0 {
		t.Fatalf("validate references xsd024: %v", errs[0])
	}

	compiled, err := compiler.NewCompiler(schema).Compile()
	if err != nil {
		t.Fatalf("compile xsd024: %v", err)
	}

	typeQName := types.QName{Local: "complexType"}
	ct := compiled.Types[typeQName]
	if ct == nil {
		t.Fatalf("expected compiled complexType")
	}
	if ct.ContentModel == nil || ct.ContentModel.Empty {
		t.Fatalf("expected complexType content model")
	}

	rootQName := types.QName{Local: "root"}
	if ct.ContentModel.ElementIndex == nil || ct.ContentModel.ElementIndex[rootQName] == nil {
		t.Fatalf("expected root element in content model index")
	}

	var hasAttr bool
	for _, attr := range ct.AllAttributes {
		if attr != nil && attr.QName.Local == "att" {
			hasAttr = true
			break
		}
	}
	if !hasAttr {
		t.Fatalf("expected attribute group to contribute attribute 'att'")
	}
}

func TestCompileW3CSimpleUnion(t *testing.T) {
	schema := parseW3CSchema(t, "saxonData/Simple/simple085.xsd")
	res := resolver.NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve simple085: %v", err)
	}

	compiled, err := compiler.NewCompiler(schema).Compile()
	if err != nil {
		t.Fatalf("compile simple085: %v", err)
	}

	unionQName := types.QName{Local: "myUnion"}
	unionType := compiled.Types[unionQName]
	if unionType == nil {
		t.Fatalf("expected compiled myUnion type")
	}
	if unionType.Kind != grammar.TypeKindSimple {
		t.Fatalf("expected myUnion to be simple type, got %v", unionType.Kind)
	}
	if len(unionType.MemberTypes) != 1 || unionType.MemberTypes[0] == nil {
		t.Fatalf("expected myUnion to have 1 member type")
	}
}
