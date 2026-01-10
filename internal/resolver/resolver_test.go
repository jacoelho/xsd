package resolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

func parseW3CSchema(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schemaPath := filepath.Join("..", "..", "testdata", "xsdtests", filepath.FromSlash(relPath))
	file, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("open schema %s: %v", schemaPath, err)
	}
	defer file.Close()

	schema, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("parse schema %s: %v", schemaPath, err)
	}
	return schema
}

func resolveW3CSchema(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schema := parseW3CSchema(t, relPath)
	resolver := NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		t.Fatalf("resolve schema %s: %v", relPath, err)
	}
	return schema
}

func resolveW3CTypeReferences(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schema := parseW3CSchema(t, relPath)
	if err := ResolveTypeReferences(schema); err != nil {
		t.Fatalf("resolve type references %s: %v", relPath, err)
	}
	return schema
}

func requireNoReferenceErrors(t *testing.T, schema *parser.Schema) {
	t.Helper()

	if errs := ValidateReferences(schema); len(errs) > 0 {
		t.Fatalf("validate references: %v", errs[0])
	}
}

func requireReferenceErrorContains(t *testing.T, schema *parser.Schema, substr string) {
	t.Helper()

	errs := ValidateReferences(schema)
	if len(errs) == 0 {
		t.Fatalf("expected reference error containing %q", substr)
	}
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), substr) {
			return
		}
	}
	t.Fatalf("expected reference error containing %q, got %v", substr, errs[0])
}

func TestResolveW3CGroupAndAttributeGroup(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/xsd024/xsd024.xsdmod")
	requireNoReferenceErrors(t, schema)

	ct, ok := schema.TypeDefs[types.QName{Local: "complexType"}].(*types.ComplexType)
	if !ok || ct == nil {
		t.Fatalf("expected complexType to be a complex type")
	}

	var refQName types.QName
	if err := schemacheck.WalkContentParticles(ct.Content(), func(p types.Particle) error {
		if ref, ok := p.(*types.GroupRef); ok {
			refQName = ref.RefQName
		}
		return nil
	}); err != nil {
		t.Fatalf("walk content particles: %v", err)
	}

	if refQName.IsZero() {
		t.Fatalf("expected group reference in complexType content")
	}
	if _, ok := schema.Groups[refQName]; !ok {
		t.Fatalf("group reference %s not found in schema groups", refQName)
	}
	if len(ct.AttrGroups) != 1 {
		t.Fatalf("expected 1 attribute group reference, got %d", len(ct.AttrGroups))
	}
}

func TestResolveW3CComplexTypeBases(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/CType/pSubstitutions/pSubstitutions00101m/pSubstitutions00101m.xsd")
	requireNoReferenceErrors(t, schema)

	baseQName := types.QName{Namespace: schema.TargetNamespace, Local: "A"}
	for _, local := range []string{"B", "C"} {
		ct, ok := schema.TypeDefs[types.QName{Namespace: schema.TargetNamespace, Local: local}].(*types.ComplexType)
		if !ok || ct == nil {
			t.Fatalf("expected %s to be a complex type", local)
		}
		if ct.ResolvedBase == nil {
			t.Fatalf("expected %s to resolve base type", local)
		}
		if ct.ResolvedBase.Name() != baseQName {
			t.Fatalf("expected %s base %s, got %s", local, baseQName, ct.ResolvedBase.Name())
		}
	}
}

func TestResolveW3CUniqueConstraints(t *testing.T) {
	schema := resolveW3CSchema(t, "saxonData/Complex/unique001.xsd")
	requireNoReferenceErrors(t, schema)

	root := schema.ElementDecls[types.QName{Local: "root"}]
	if root == nil {
		t.Fatalf("expected root element declaration")
	}
	if len(root.Constraints) != 1 {
		t.Fatalf("expected 1 identity constraint, got %d", len(root.Constraints))
	}
	if root.Constraints[0].Name != "test" {
		t.Fatalf("expected constraint name 'test', got %q", root.Constraints[0].Name)
	}
}

func TestResolveW3CMissingListItemType(t *testing.T) {
	schema := resolveW3CTypeReferences(t, "saxonData/Missing/missing006.xsd")

	st, ok := schema.TypeDefs[types.QName{Local: "list"}].(*types.SimpleType)
	if !ok || st == nil {
		t.Fatalf("expected list to be a simple type")
	}
	if st.ItemType == nil {
		t.Fatalf("expected list item type to be set")
	}
	if st.ItemType.Name().Local != "absent" {
		t.Fatalf("expected list item type name 'absent', got %s", st.ItemType.Name())
	}
	if st.WhiteSpace() != types.WhiteSpaceCollapse {
		t.Fatalf("expected list whiteSpace collapse, got %v", st.WhiteSpace())
	}
}

func TestResolveW3CMissingSimpleTypeBase(t *testing.T) {
	schema := parseW3CSchema(t, "saxonData/Missing/missing004.xsd")
	if err := ResolveTypeReferences(schema); err == nil {
		t.Fatalf("expected error resolving missing base type")
	}
}

func TestValidateReferencesCyclicSubstitutionGroups(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/xsd010/xsd010.e.xsd")
	requireReferenceErrorContains(t, schema, "cyclic substitution group")
}

func TestValidateReferencesDuplicateIdentityConstraints(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/IdConstrDefs/name/name00101m/name00101m2.xsd")
	requireReferenceErrorContains(t, schema, "not unique")
}

func TestValidateReferencesAttributeReferences(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/AttrDecl/AD_valConstr/AD_valConstr00101m/AD_valConstr00101m.xsd")
	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesKeyref(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/identity/IdentityTestSuite/001/test.xsd")
	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesInlineTypes(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/xsd001/xsd001.xsd")
	requireNoReferenceErrors(t, schema)
}
