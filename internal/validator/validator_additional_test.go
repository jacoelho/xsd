package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestCheckFixedValueDecimal(t *testing.T) {
	run := &validationRun{schema: newBaseSchemaView(nil)}
	decimalType := &grammar.CompiledType{
		QName:    types.QName{Namespace: types.XSDNamespace, Local: "decimal"},
		Original: types.GetBuiltin(types.TypeName("decimal")),
		Kind:     grammar.TypeKindBuiltin,
	}

	if errs := run.checkFixedValue("1.0", "1.00", decimalType); len(errs) != 0 {
		t.Fatalf("expected decimal fixed values to match")
	}
	if errs := run.checkFixedValue("1.0", "2.0", decimalType); len(errs) == 0 {
		t.Fatalf("expected decimal fixed value mismatch")
	}
	if errs := run.checkFixedValue("a", "b", nil); len(errs) == 0 {
		t.Fatalf("expected string fallback mismatch")
	}
}

func TestCheckFixedValueUnionMemberTypes(t *testing.T) {
	run := &validationRun{schema: newBaseSchemaView(nil)}
	memberInt := &grammar.CompiledType{Original: types.GetBuiltin(types.TypeName("int")), Kind: grammar.TypeKindBuiltin}
	memberBool := &grammar.CompiledType{Original: types.GetBuiltin(types.TypeName("boolean")), Kind: grammar.TypeKindBuiltin}
	unionType := &grammar.CompiledType{
		Original:    types.GetBuiltin(types.TypeName("string")),
		MemberTypes: []*grammar.CompiledType{memberInt, memberBool},
	}

	if errs := run.checkFixedValue("1", "1", unionType); len(errs) != 0 {
		t.Fatalf("expected union fixed values to match")
	}
	if errs := run.checkFixedValue("1", "true", unionType); len(errs) == 0 {
		t.Fatalf("expected union fixed value mismatch")
	}
}

func TestCheckFixedValueUnionSimpleType(t *testing.T) {
	run := &validationRun{schema: newBaseSchemaView(nil)}
	union := &types.SimpleType{}
	union.Union = &types.UnionType{
		MemberTypes: []types.QName{
			{Namespace: types.XSDNamespace, Local: "string"},
			{Namespace: types.XSDNamespace, Local: "int"},
		},
	}
	unionType := &grammar.CompiledType{Original: union}

	if errs := run.checkFixedValue("10", "10", unionType); len(errs) != 0 {
		t.Fatalf("expected union simpleType fixed values to match")
	}
	if errs := run.checkFixedValue("10", "true", unionType); len(errs) == 0 {
		t.Fatalf("expected union simpleType mismatch")
	}
}

func TestCompareTypedValues(t *testing.T) {
	decimal := types.GetBuiltin(types.TypeName("decimal"))
	leftDec, _ := decimal.ParseValue("1.0")
	rightDec, _ := decimal.ParseValue("1.00")
	if !compareTypedValues(leftDec, rightDec) {
		t.Fatalf("expected decimal values to match")
	}

	boolean := types.GetBuiltin(types.TypeName("boolean"))
	leftBool, _ := boolean.ParseValue("true")
	rightBool, _ := boolean.ParseValue("1")
	if !compareTypedValues(leftBool, rightBool) {
		t.Fatalf("expected boolean values to match")
	}

	stringType := types.GetBuiltin(types.TypeName("string"))
	leftStr, _ := stringType.ParseValue("a")
	rightStr, _ := stringType.ParseValue("b")
	if compareTypedValues(leftStr, rightStr) {
		t.Fatalf("expected string values to differ")
	}

	longType := types.GetBuiltin(types.TypeName("long"))
	leftLong, _ := longType.ParseValue("10")
	rightLong, _ := longType.ParseValue("10")
	if !compareTypedValues(leftLong, rightLong) {
		t.Fatalf("expected long values to match")
	}
}

func TestSchemaViewLookup(t *testing.T) {
	headQName := types.QName{Namespace: "urn:ex", Local: "head"}
	subQName := types.QName{Namespace: "urn:ex", Local: "sub"}
	attrQName := types.QName{Namespace: "urn:ex", Local: "attr"}
	typeQName := types.QName{Namespace: "urn:ex", Local: "type"}
	notationQName := types.QName{Namespace: "urn:ex", Local: "note"}

	headElem := &grammar.CompiledElement{QName: headQName}
	subElem := &grammar.CompiledElement{QName: subQName}
	attr := &grammar.CompiledAttribute{QName: attrQName}
	typ := &grammar.CompiledType{QName: typeQName}
	notation := &types.NotationDecl{Name: notationQName}

	schema := &grammar.CompiledSchema{
		Elements:                map[types.QName]*grammar.CompiledElement{headQName: headElem},
		LocalElements:           map[types.QName]*grammar.CompiledElement{subQName: subElem},
		Types:                   map[types.QName]*grammar.CompiledType{typeQName: typ},
		Attributes:              map[types.QName]*grammar.CompiledAttribute{attrQName: attr},
		NotationDecls:           map[types.QName]*types.NotationDecl{notationQName: notation},
		SubstitutionGroups:      map[types.QName][]*grammar.CompiledElement{headQName: {subElem}},
		ElementsWithConstraints: []*grammar.CompiledElement{headElem},
	}

	base := newBaseSchemaView(schema)
	if base.Element(headQName) != headElem {
		t.Fatalf("expected base Element lookup")
	}
	if base.LocalElement(subQName) != subElem {
		t.Fatalf("expected base LocalElement lookup")
	}
	if base.Type(typeQName) != typ {
		t.Fatalf("expected base Type lookup")
	}
	if base.Attribute(attrQName) != attr {
		t.Fatalf("expected base Attribute lookup")
	}
	if base.Notation(notationQName) != notation {
		t.Fatalf("expected base Notation lookup")
	}
	if base.SubstitutionGroupHead(subQName) != headElem {
		t.Fatalf("expected substitution group head lookup")
	}

	if !containsCompiledElement([]*grammar.CompiledElement{headElem}, headElem) {
		t.Fatalf("expected containsCompiledElement to return true")
	}
}

func TestSubstitutionMatcher(t *testing.T) {
	headQName := types.QName{Namespace: "urn:ex", Local: "head"}
	subQName := types.QName{Namespace: "urn:ex", Local: "sub"}

	headType := &grammar.CompiledType{QName: types.QName{Namespace: "urn:ex", Local: "base"}}
	subType := &grammar.CompiledType{
		QName:            types.QName{Namespace: "urn:ex", Local: "subType"},
		DerivationMethod: types.DerivationExtension,
	}
	subType.DerivationChain = []*grammar.CompiledType{subType, headType}

	headElem := &grammar.CompiledElement{QName: headQName, Type: headType}
	subElem := &grammar.CompiledElement{QName: subQName, Type: subType}

	schema := &grammar.CompiledSchema{
		Elements:           map[types.QName]*grammar.CompiledElement{headQName: headElem, subQName: subElem},
		SubstitutionGroups: map[types.QName][]*grammar.CompiledElement{headQName: {subElem}},
	}

	run := &validationRun{schema: newBaseSchemaView(schema)}
	matcher := run.matcher()
	if !matcher.IsSubstitutable(subQName, headQName) {
		t.Fatalf("expected substitution to be allowed")
	}
	if resolved := run.resolveSubstitutionDecl(subQName, headElem); resolved != subElem {
		t.Fatalf("expected substitution to resolve to actual element")
	}

	headType.Block = types.DerivationSet(types.DerivationExtension)
	if matcher.IsSubstitutable(subQName, headQName) {
		t.Fatalf("expected substitution to be blocked by derivation")
	}
	if resolved := run.resolveSubstitutionDecl(subQName, headElem); resolved != headElem {
		t.Fatalf("expected substitution to remain at head")
	}
}

func TestElementHelpers(t *testing.T) {
	simpleType := &grammar.CompiledType{
		QName:    types.QName{Namespace: types.XSDNamespace, Local: "string"},
		Original: types.GetBuiltin(types.TypeName("string")),
		Kind:     grammar.TypeKindBuiltin,
	}
	decl := &grammar.CompiledElement{Type: simpleType}
	if textTypeForFixedValue(decl) != simpleType {
		t.Fatalf("expected simple text type for fixed value")
	}

	mixedType := &grammar.CompiledType{
		Kind:     grammar.TypeKindComplex,
		Mixed:    true,
		Original: &types.ComplexType{},
	}
	decl = &grammar.CompiledElement{Type: mixedType}
	if textTypeForFixedValue(decl) != nil {
		t.Fatalf("expected nil text type for mixed complex type")
	}

	if !isAnyType(&grammar.CompiledType{QName: types.QName{Namespace: types.XSDNamespace, Local: "anyType"}}) {
		t.Fatalf("expected anyType match")
	}
	if !isAnySimpleType(&grammar.CompiledType{QName: types.QName{Namespace: types.XSDNamespace, Local: "anySimpleType"}}) {
		t.Fatalf("expected anySimpleType match")
	}

	if !isWhitespaceOnlyBytes([]byte(" \t\r\n")) || isWhitespaceOnlyBytes([]byte("x")) {
		t.Fatalf("unexpected whitespace-only bytes result")
	}
	if !isWhitespaceOnly([]byte(" \t\n")) || isWhitespaceOnly([]byte("x")) {
		t.Fatalf("unexpected whitespace-only result")
	}
}

func TestStreamIdentityHelpers(t *testing.T) {
	xmlStr := `<root xmlns:ex="urn:ex" ex:attr="v"></root>`
	dec, err := xmlstream.NewStringReader(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("NewStreamDecoder error = %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}

	run := &streamRun{dec: dec}
	got, err := run.normalizeQNameValue("ex:val", ev.ScopeDepth)
	if err != nil {
		t.Fatalf("normalizeQNameValue error = %v", err)
	}
	if got != "{urn:ex}val" {
		t.Fatalf("unexpected normalized QName: %s", got)
	}
	if attr, ok := findAttrByLocal(ev.Attrs, "attr"); !ok || attr.Value() != "v" {
		t.Fatalf("expected attr lookup to find value")
	}

	if !isAnySimpleOrAnyType(types.GetBuiltin(types.TypeNameAnySimpleType)) {
		t.Fatalf("expected anySimpleType to match")
	}
	if isAnySimpleOrAnyType(types.GetBuiltin(types.TypeNameString)) {
		t.Fatalf("expected string to not match anySimpleType")
	}
}

func TestLengthFacetHelpers(t *testing.T) {
	length := &types.Length{Value: 1}
	if !isLengthFacet(length) {
		t.Fatalf("expected length facet to be detected")
	}

	ct := &grammar.CompiledType{
		IsQNameOrNotationType: true,
		PrimitiveType: &grammar.CompiledType{
			QName: types.QName{Namespace: types.XSDNamespace, Local: "QName"},
		},
	}
	if !shouldSkipLengthFacet(ct, length) {
		t.Fatalf("expected length facet to be skipped for QName")
	}

	if !facetsAllowSimpleValue([]types.Facet{length}) {
		t.Fatalf("length facets should allow simple values")
	}
	minInclusive, err := types.NewMinInclusive("1", types.GetBuiltin(types.TypeNameDecimal))
	if err != nil {
		t.Fatalf("NewMinInclusive error: %v", err)
	}
	if facetsAllowSimpleValue([]types.Facet{minInclusive}) {
		t.Fatalf("expected range facets to require typed values")
	}
}

func TestCheckComplexTypeFacets(t *testing.T) {
	run := &validationRun{}
	stringType := types.GetBuiltin(types.TypeName("string"))
	ct := &grammar.CompiledType{
		SimpleContentType: &grammar.CompiledType{Original: stringType},
		Facets:            []types.Facet{&types.MinLength{Value: 2}},
	}
	if violations := run.checkComplexTypeFacets("a", ct); len(violations) == 0 {
		t.Fatalf("expected minLength facet violation")
	}

	decimalType := types.GetBuiltin(types.TypeName("decimal"))
	minInclusive, err := types.NewMinInclusive("10", decimalType)
	if err != nil {
		t.Fatalf("NewMinInclusive error: %v", err)
	}
	ct = &grammar.CompiledType{
		SimpleContentType: &grammar.CompiledType{Original: decimalType},
		Facets:            []types.Facet{minInclusive},
	}
	if violations := run.checkComplexTypeFacets("12", ct); len(violations) != 0 {
		t.Fatalf("expected minInclusive to pass")
	}
}
