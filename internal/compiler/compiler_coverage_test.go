package compiler_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/types"
)

func compileW3CSchema(t *testing.T, relPath string) (types.NamespaceURI, *grammar.CompiledSchema) {
	t.Helper()

	schema := parseW3CSchema(t, relPath)
	res := resolver.NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve %s: %v", relPath, err)
	}
	if errs := resolver.ValidateReferences(schema); len(errs) > 0 {
		t.Fatalf("validate references %s: %v", relPath, errs[0])
	}

	compiled, err := compiler.NewCompiler(schema).Compile()
	if err != nil {
		t.Fatalf("compile %s: %v", relPath, err)
	}
	return schema.TargetNamespace, compiled
}

func requireCompiledElementType(t *testing.T, compiled *grammar.CompiledSchema, qname types.QName) *grammar.CompiledType {
	t.Helper()

	elem := compiled.Elements[qname]
	if elem == nil || elem.Type == nil {
		t.Fatalf("expected compiled element %s with type", qname)
	}
	return elem.Type
}

func TestCompileAnyAttributeIntersections(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "sunData/combined/007/test.xsd")

	empty := requireCompiledElementType(t, compiled, types.QName{Namespace: targetNS, Local: "emptywc"})
	if empty.AnyAttribute != nil {
		t.Fatalf("expected emptywc to have no anyAttribute after intersection")
	}

	justA := requireCompiledElementType(t, compiled, types.QName{Namespace: targetNS, Local: "justA"})
	if justA.AnyAttribute == nil {
		t.Fatalf("expected justA to have anyAttribute")
	}
	if !justA.AnyAttribute.AllowsQName(types.QName{Namespace: "urn:a", Local: "x"}) {
		t.Fatalf("expected justA anyAttribute to allow urn:a")
	}
	if justA.AnyAttribute.AllowsQName(types.QName{Namespace: "urn:b", Local: "x"}) {
		t.Fatalf("expected justA anyAttribute to reject urn:b")
	}
}

func TestCompileAnyAttributeDerivation(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "sunData/combined/008/test.xsd")

	extension := requireCompiledElementType(t, compiled, types.QName{Namespace: targetNS, Local: "extension"})
	if extension.AnyAttribute == nil {
		t.Fatalf("expected extension to have anyAttribute")
	}
	for _, ns := range []string{"urn:a", "urn:b", "urn:c"} {
		if !extension.AnyAttribute.AllowsQName(types.QName{Namespace: types.NamespaceURI(ns), Local: "x"}) {
			t.Fatalf("expected extension anyAttribute to allow %s", ns)
		}
	}
	if extension.AnyAttribute.AllowsQName(types.QName{Namespace: "urn:foo", Local: "x"}) {
		t.Fatalf("expected extension anyAttribute to reject urn:foo")
	}

	restriction := requireCompiledElementType(t, compiled, types.QName{Namespace: targetNS, Local: "restriction"})
	if restriction.AnyAttribute == nil {
		t.Fatalf("expected restriction to have anyAttribute")
	}
	if !restriction.AnyAttribute.AllowsQName(types.QName{Namespace: "urn:a", Local: "x"}) {
		t.Fatalf("expected restriction anyAttribute to allow urn:a")
	}
	for _, ns := range []string{"urn:b", "urn:c"} {
		if restriction.AnyAttribute.AllowsQName(types.QName{Namespace: types.NamespaceURI(ns), Local: "x"}) {
			t.Fatalf("expected restriction anyAttribute to reject %s", ns)
		}
	}

	alias := requireCompiledElementType(t, compiled, types.QName{Namespace: targetNS, Local: "alias"})
	if alias.AnyAttribute != nil {
		t.Fatalf("expected alias to have no anyAttribute")
	}
}

func TestCompileSimpleContentDerivation(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "sunData/CType/baseTD/baseTD00101m/baseTD00101m1.xsd")

	base := compiled.Types[types.QName{Namespace: targetNS, Local: "Test2"}]
	if base == nil {
		t.Fatalf("expected compiled Test2 type")
	}
	if base.SimpleContentType == nil || base.SimpleContentType.QName.Local != "int" {
		t.Fatalf("expected Test2 simple content type to be xsd:int")
	}

	derived := compiled.Types[types.QName{Namespace: targetNS, Local: "Test"}]
	if derived == nil {
		t.Fatalf("expected compiled Test type")
	}
	if derived.SimpleContentType == nil || derived.SimpleContentType.QName.Local != "int" {
		t.Fatalf("expected Test simple content type to be xsd:int")
	}
}

func TestCompileComplexContentExtensionUsesBaseContent(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "sunData/combined/006/test.xsd")

	base := compiled.Types[types.QName{Namespace: targetNS, Local: "B"}]
	if base == nil || base.ContentModel == nil || base.ContentModel.Empty {
		t.Fatalf("expected base type B to have content")
	}

	extended := compiled.Types[types.QName{Namespace: targetNS, Local: "De"}]
	if extended == nil || extended.ContentModel == nil || extended.ContentModel.Empty {
		t.Fatalf("expected extension type De to reuse base content")
	}
	if extended.ContentModel.ElementIndex == nil {
		t.Fatalf("expected extension content model element index")
	}
	if extended.ContentModel.ElementIndex[types.QName{Namespace: targetNS, Local: "foo"}] == nil {
		t.Fatalf("expected extension content model to include element foo")
	}
}

func TestCompileAttributeReferencesAndForm(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "sunData/AttrDecl/AD_valConstr/AD_valConstr00101m/AD_valConstr00101m.xsd")

	elemType := requireCompiledElementType(t, compiled, types.QName{Namespace: targetNS, Local: "elementWithAttr"})

	var numberAttr *grammar.CompiledAttribute
	var priceAttr *grammar.CompiledAttribute
	for _, attr := range elemType.AllAttributes {
		if attr == nil {
			continue
		}
		switch attr.QName.Local {
		case "number":
			numberAttr = attr
		case "price":
			priceAttr = attr
		}
	}
	if numberAttr == nil || !numberAttr.HasFixed || numberAttr.Fixed != "12" {
		t.Fatalf("expected referenced attribute number to inherit fixed value 12")
	}
	if priceAttr == nil {
		t.Fatalf("expected local attribute price to be compiled")
	}
	if !priceAttr.QName.Namespace.IsEmpty() {
		t.Fatalf("expected price attribute to be unqualified")
	}
}

func TestCompileAllGroupContent(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "msData/additional/addB089.xsd")

	ct := compiled.Types[types.QName{Namespace: targetNS, Local: "base"}]
	if ct == nil || ct.ContentModel == nil {
		t.Fatalf("expected compiled type base with content model")
	}
	if ct.ContentModel.AllElements == nil || len(ct.ContentModel.AllElements) != 2 {
		t.Fatalf("expected all group to produce 2 elements")
	}
}

func TestCompileGroupRefMaxOccursZero(t *testing.T) {
	_, compiled := compileW3CSchema(t, "msData/group/groupL008.xsd")

	elemType := requireCompiledElementType(t, compiled, types.QName{Local: "elem"})
	if elemType.ContentModel == nil || !elemType.ContentModel.Empty {
		t.Fatalf("expected group ref with maxOccurs=0 to yield empty content")
	}
	if elemType.ContentModel.RejectAll {
		t.Fatalf("expected empty content model, not reject-all")
	}
}

func TestCompileListSimpleType(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "ibmData/instance_invalid/S3_3_4/s3_3_4ii08.xsd")

	listType := compiled.Types[types.QName{Namespace: targetNS, Local: "listOfIDs"}]
	if listType == nil {
		t.Fatalf("expected compiled listOfIDs type")
	}
	if listType.DerivationMethod != types.DerivationList {
		t.Fatalf("expected listOfIDs to be list derivation")
	}
	if listType.ItemType == nil || listType.ItemType.IDTypeName == "" {
		t.Fatalf("expected listOfIDs item type to be ID-based")
	}
}

func TestCompileIdentityConstraints(t *testing.T) {
	targetNS, compiled := compileW3CSchema(t, "sunData/combined/identity/IdentityTestSuite/001/test.xsd")

	root := compiled.Elements[types.QName{Namespace: targetNS, Local: "root"}]
	if root == nil {
		t.Fatalf("expected root element")
	}
	if len(root.Constraints) != 2 {
		t.Fatalf("expected 2 identity constraints, got %d", len(root.Constraints))
	}
	for _, constraint := range root.Constraints {
		if constraint == nil || len(constraint.SelectorPaths) == 0 || len(constraint.FieldPaths) == 0 {
			t.Fatalf("expected compiled constraint with selector and fields")
		}
	}
}
