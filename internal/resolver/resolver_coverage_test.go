package resolver

import (
	"testing"

	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveTypeReferencesAnonymousContent(t *testing.T) {
	schema := parseW3CSchema(t, "sunData/combined/xsd001/xsd001.xsd")
	if err := ResolveTypeReferences(schema); err != nil {
		t.Fatalf("resolve type references: %v", err)
	}
}

func TestValidateSimpleTypeFinalList(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/SType/ST_final/ST_final00102m/ST_final00102m1.xsd")
	requireReferenceErrorContains(t, schema, "final for list")
}

func TestValidateSimpleTypeFinalUnion(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/SType/ST_final/ST_final00103m/ST_final00103m1.xsd")
	requireReferenceErrorContains(t, schema, "final for union")
}

func TestValidateTypeQNameReferenceMissing(t *testing.T) {
	schema := parseW3CSchema(t, "saxonData/Missing/missing001.xsd")
	err := validateTypeQNameReference(schema, types.QName{Local: "absent"}, schema.TargetNamespace)
	if err == nil {
		t.Fatalf("expected missing type reference error")
	}
}

func TestResolveUnionAndListItemTypes(t *testing.T) {
	unionSchema := parseW3CSchema(t, "saxonData/Simple/simple085.xsd")
	unionType, ok := unionSchema.TypeDefs[types.QName{Local: "myUnion"}].(*types.SimpleType)
	if !ok || unionType == nil {
		t.Fatalf("expected myUnion simple type")
	}
	if members := typeops.ResolveUnionMemberTypes(unionSchema, unionType); len(members) == 0 {
		t.Fatalf("expected union member types")
	}

	listSchema := parseW3CSchema(t, "ibmData/instance_invalid/S3_3_4/s3_3_4ii08.xsd")
	listType, ok := listSchema.TypeDefs[types.QName{Local: "listOfIDs"}].(*types.SimpleType)
	if !ok || listType == nil {
		t.Fatalf("expected listOfIDs simple type")
	}
	item := typeops.ResolveListItemType(listSchema, listType)
	if item == nil || item.Name().Local != "ID" {
		t.Fatalf("expected listOfIDs item type ID, got %v", item)
	}
}

func TestValidateValueAgainstFacets(t *testing.T) {
	schema := parseW3CSchema(t, "sunData/combined/xsd001/xsd001.xsd")
	st, ok := schema.TypeDefs[types.QName{Namespace: schema.TargetNamespace, Local: "mytype"}].(*types.SimpleType)
	if !ok || st == nil {
		t.Fatalf("expected mytype simple type")
	}
	facets, err := typeops.CollectSimpleTypeFacets(schema, st, nil)
	if err != nil {
		t.Fatalf("collect simple type facets: %v", err)
	}
	if err := types.ValidateValueAgainstFacets("abcd", st, facets, nil); err != nil {
		t.Fatalf("expected valid facet value, got %v", err)
	}
	if err := types.ValidateValueAgainstFacets("ab", st, facets, nil); err == nil {
		t.Fatalf("expected facet violation for short value")
	}
}

func TestResolveSimpleContentBaseType(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/CType/baseTD/baseTD00101m/baseTD00101m1.xsd")
	ct, ok := schema.TypeDefs[types.QName{Namespace: schema.TargetNamespace, Local: "Test2"}].(*types.ComplexType)
	if !ok || ct == nil {
		t.Fatalf("expected Test2 complex type")
	}
	sc, ok := ct.Content().(*types.SimpleContent)
	if !ok || sc == nil {
		t.Fatalf("expected simpleContent on Test2")
	}
	textType := typeops.ResolveSimpleContentBaseTypeFromContent(schema, sc)
	if textType == nil || textType.Name().Local != "int" {
		t.Fatalf("expected text content type xsd:int")
	}
}

func TestTypeDerivationHelpers(t *testing.T) {
	schema := parseW3CSchema(t, "sunData/combined/006/test.xsd")
	bQName := types.QName{Namespace: schema.TargetNamespace, Local: "B"}
	drrQName := types.QName{Namespace: schema.TargetNamespace, Local: "Drr"}
	bType, ok := schema.TypeDefs[bQName].(*types.ComplexType)
	if !ok || bType == nil {
		t.Fatalf("expected B complex type")
	}
	if !typesAreEqual(bQName, bType) {
		t.Fatalf("expected typesAreEqual to match B")
	}
	if !isTypeInDerivationChain(schema, drrQName, bType) {
		t.Fatalf("expected B in derivation chain for Drr")
	}

	xsdSchema := parseW3CSchema(t, "sunData/combined/xsd001/xsd001.xsd")
	if err := ResolveTypeReferences(xsdSchema); err != nil {
		t.Fatalf("resolve type references: %v", err)
	}
	derived, ok := xsdSchema.TypeDefs[types.QName{Namespace: xsdSchema.TargetNamespace, Local: "mytype"}].(*types.SimpleType)
	if !ok || derived == nil {
		t.Fatalf("expected mytype simple type")
	}
	base := types.GetBuiltin(types.TypeNameString)
	if base == nil {
		t.Fatalf("expected builtin string type")
	}
	if !types.IsDerivedFrom(derived, base) {
		t.Fatalf("expected mytype derived from string")
	}
	if prim := getPrimitiveType(derived); prim == nil || prim.Name().Local != "string" {
		t.Fatalf("expected primitive type string")
	}
	if !areFieldTypesCompatible(derived, base) {
		t.Fatalf("expected field types compatible for derived and base")
	}
}

func TestValidateElementDefaultValues(t *testing.T) {
	valid := resolveW3CSchema(t, "sunData/ElemDecl/valueConstraint/valueConstraint00101m/valueConstraint00101m1.xsd")
	requireNoReferenceErrors(t, valid)

	invalid := resolveW3CSchema(t, "sunData/ElemDecl/valueConstraint/valueConstraint00101m/valueConstraint00101m2.xsd")
	requireReferenceErrorContains(t, invalid, "invalid default value")
}

func TestValidateSubstitutionGroupFinal(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/ElemDecl/substGroupExclusions/substGrpExcl00202m/substGrpExcl00202m2.xsd")
	requireReferenceErrorContains(t, schema, "final for extension")
}

func TestValidateSimpleTypeFinalRestriction(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/SType/ST_final/ST_final00101m/ST_final00101m1.xsd")
	requireReferenceErrorContains(t, schema, "final for restriction")
}
