package validate

import (
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
)

func TestIdentityEvaluationUsesCompiledDispatchForSelectorAndFields(t *testing.T) {
	t.Parallel()

	fixture := compiledIdentityRuntimeForTest(t)
	evaluation := newIdentityEvaluation(fixture.rt, identityLimits{}, 0)
	if err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 1, 1),
		Name:    xsdSchema.RuntimeName{Known: true, Name: fixture.elemName},
		Element: fixture.elemID,
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatalf("startElement() error = %v", err)
	}

	elementMatches := evaluation.dispatchElementFieldMatches()
	if len(elementMatches) != 1 || elementMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("dispatchElementFieldMatches() = %+v, want selection 0 field 0", elementMatches)
	}

	attrMatches, err := evaluation.dispatchAttributeFieldMatches(xsdSchema.RuntimeName{Name: fixture.attrName, Known: true})
	if err != nil {
		t.Fatalf("dispatchAttributeFieldMatches() error = %v", err)
	}
	if len(attrMatches) != 1 || attrMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("dispatchAttributeFieldMatches() = %+v, want one deduplicated field match", attrMatches)
	}
	unknownMatches, err := evaluation.dispatchAttributeFieldMatches(xsdSchema.RuntimeName{NS: "urn:a", Local: "unknown"})
	if err != nil {
		t.Fatalf("dispatchAttributeFieldMatches(unknown) error = %v", err)
	}
	if len(unknownMatches) != 1 || unknownMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("dispatchAttributeFieldMatches(unknown) = %+v, want namespace-wildcard match", unknownMatches)
	}
	wrongNamespace, err := evaluation.dispatchAttributeFieldMatches(xsdSchema.RuntimeName{NS: "urn:other", Local: "unknown"})
	if err != nil {
		t.Fatalf("dispatchAttributeFieldMatches(wrong namespace) error = %v", err)
	}
	if len(wrongNamespace) != 0 {
		t.Fatalf("dispatchAttributeFieldMatches(wrong namespace) = %+v, want no match", wrongNamespace)
	}
}

func TestCompiledIdentityFieldPathsMatchElementAndAttributeBranches(t *testing.T) {
	t.Parallel()

	fixture := compiledIdentityRuntimeForTest(t)
	otherAttr := xsdSchema.QName{Namespace: 999, Local: fixture.attrName.Local}
	namePath := []xsdSchema.RuntimeName{{Known: true, Name: fixture.elemName}}

	constraint, ok := fixture.rt.IdentityConstraint(fixture.constraintID)
	if !ok {
		t.Fatal("IdentityConstraint() rejected runtime metadata")
	}
	elementFields := constraint.ElementFields()
	elementField, ok := elementFields.At(0)
	if !ok {
		t.Fatal("IdentityConstraint().ElementFields() returned no field")
	}
	elementPath, ok := elementField.Path(0)
	if !ok {
		t.Fatal("IdentityConstraint().ElementFields() returned no path")
	}
	elementProgram := identityFieldPathProgram(elementPath)
	if !elementProgram.Matches(fixture.rt, namePath, 1, 1) {
		t.Fatal("element field path did not match")
	}

	attributeFields := constraint.AttributeFields(fixture.attrName)
	exactAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityConstraint().AttributeFields() returned no exact field")
	}
	exactPath, ok := exactAttributeField.Path(0)
	if !ok {
		t.Fatal("IdentityConstraint().AttributeFields() returned no path")
	}
	exactProgram := identityFieldPathProgram(exactPath)
	if !exactProgram.Matches(fixture.rt, namePath, 1, 1) || !exactProgram.AttributeMatches(fixture.rt, xsdSchema.RuntimeName{Name: fixture.attrName, Known: true}) {
		t.Fatal("exact attribute field path did not match")
	}
	nestedNamePath := []xsdSchema.RuntimeName{
		{Known: true, Name: fixture.elemName},
		{Known: true, Name: fixture.elemName},
	}
	if exactProgram.Matches(fixture.rt, nestedNamePath, 1, 2) {
		t.Fatal("direct attribute field path matched below selected depth")
	}
	attributeFields = constraint.AttributeWildcardFields()
	wildcardAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityConstraint().AttributeWildcardFields() returned no wildcard field")
	}
	wildcardPath, ok := wildcardAttributeField.Path(0)
	if !ok {
		t.Fatal("IdentityConstraint().AttributeWildcardFields() returned no path")
	}
	wildcardProgram := identityFieldPathProgram(wildcardPath)
	if !wildcardProgram.Matches(fixture.rt, namePath, 1, 1) || !wildcardProgram.AttributeMatches(fixture.rt, xsdSchema.RuntimeName{Name: fixture.attrName, Known: true}) {
		t.Fatal("attribute namespace wildcard did not match")
	}
	if wildcardProgram.AttributeMatches(fixture.rt, xsdSchema.RuntimeName{Name: otherAttr, Known: true}) {
		t.Fatal("attribute namespace wildcard matched wrong namespace")
	}
}

type compiledIdentityFixture struct {
	rt           *xsdSchema.Schema
	elemID       xsdSchema.ElementID
	constraintID xsdSchema.IdentityConstraintID
	elemName     xsdSchema.QName
	attrName     xsdSchema.QName
}

func compiledIdentityRuntimeForTest(tb testing.TB) compiledIdentityFixture {
	tb.Helper()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:a" targetNamespace="urn:a" elementFormDefault="qualified">
	<xs:attribute name="id" type="xs:string"/>
  <xs:element name="root">
	<xs:complexType mixed="true"><xs:attribute ref="t:id"/></xs:complexType>
	<xs:key name="k">
	  <xs:selector xpath="."/>
	  <xs:field xpath=". | @t:id | @t:*"/>
	</xs:key>
  </xs:element>
</xs:schema>`
	rt, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("identity.xsd", []byte(schema))})
	if err != nil {
		tb.Fatalf("Compile() error = %v", err)
	}
	elemName, ok := rt.LookupQName("urn:a", "root")
	if !ok {
		tb.Fatal("LookupQName(root) failed")
	}
	attrName, ok := rt.LookupQName("urn:a", "id")
	if !ok {
		tb.Fatal("LookupQName(id) failed")
	}
	elemID, _, ok := rt.RootElement(xsdSchema.RuntimeName{Known: true, Name: elemName})
	if !ok {
		tb.Fatal("RootElement(root) failed")
	}
	constraints, ok := rt.ElementIdentityConstraints(elemID)
	if !ok {
		tb.Fatal("ElementIdentityConstraints(root) failed")
	}
	constraintID, ok := constraints.At(0)
	if !ok {
		tb.Fatal("root has no identity constraint")
	}
	return compiledIdentityFixture{
		rt:           rt,
		elemID:       elemID,
		constraintID: constraintID,
		elemName:     elemName,
		attrName:     attrName,
	}
}

func BenchmarkIdentityDirectFieldPath(b *testing.B) {
	fixture := compiledIdentityRuntimeForTest(b)
	constraint, ok := fixture.rt.IdentityConstraint(fixture.constraintID)
	if !ok {
		b.Fatal("IdentityConstraint() rejected runtime metadata")
	}
	fields := constraint.AttributeFields(fixture.attrName)
	field, ok := fields.At(0)
	if !ok {
		b.Fatal("IdentityConstraint().AttributeFields() returned no exact field")
	}
	path, ok := field.Path(0)
	if !ok {
		b.Fatal("CompiledIdentityFieldRead.Path() returned no exact path")
	}
	program := identityFieldPathProgram(path)
	namePath := make([]xsdSchema.RuntimeName, 256)
	for i := range namePath {
		namePath[i] = xsdSchema.RuntimeName{Known: true, Name: fixture.elemName}
	}

	tests := []struct {
		name          string
		selectedDepth int
		currentDepth  int
		want          bool
	}{
		{name: "match", selectedDepth: 256, currentDepth: 256, want: true},
		{name: "miss", selectedDepth: 1, currentDepth: 256, want: false},
	}
	for _, test := range tests {
		b.Run("compiled_"+test.name, func(b *testing.B) {
			for b.Loop() {
				if got := program.Matches(fixture.rt, namePath, test.selectedDepth, test.currentDepth); got != test.want {
					b.Fatalf("identityPathProgram.Matches() = %t, want %t", got, test.want)
				}
			}
		})
	}
}
