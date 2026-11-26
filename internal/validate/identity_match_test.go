package validate

import (
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
)

func TestIdentityEvaluationUsesCompiledDispatchForSelectorAndFields(t *testing.T) {
	t.Parallel()

	fixture := compiledIdentityRuntimeForTest(t)
	evaluation := newIdentityEvaluation(fixture.rt, identityLimits{})
	if err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 1, 1),
		Name:    fixture.elemName,
		Element: fixture.elemID,
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatalf("startElement() error = %v", err)
	}

	elementMatches := evaluation.dispatchElementFieldMatches()
	if len(elementMatches) != 1 || elementMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("dispatchElementFieldMatches() = %+v, want selection 0 field 0", elementMatches)
	}

	attrMatches, err := evaluation.dispatchAttributeFieldMatches(attributeFieldName{name: fixture.attrName, known: true})
	if err != nil {
		t.Fatalf("dispatchAttributeFieldMatches() error = %v", err)
	}
	if len(attrMatches) != 1 || attrMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("dispatchAttributeFieldMatches() = %+v, want one deduplicated field match", attrMatches)
	}
	unknownMatches, err := evaluation.dispatchAttributeFieldMatches(attributeFieldNameOf(fixture.rt.ResolveName("urn:a", "unknown")))
	if err != nil {
		t.Fatalf("dispatchAttributeFieldMatches(unknown) error = %v", err)
	}
	if len(unknownMatches) != 1 || unknownMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("dispatchAttributeFieldMatches(unknown) = %+v, want namespace-wildcard match", unknownMatches)
	}
	wrongNamespace, err := evaluation.dispatchAttributeFieldMatches(attributeFieldNameOf(fixture.rt.ResolveName("urn:other", "unknown")))
	if err != nil {
		t.Fatalf("dispatchAttributeFieldMatches(wrong namespace) error = %v", err)
	}
	if len(wrongNamespace) != 0 {
		t.Fatalf("dispatchAttributeFieldMatches(wrong namespace) = %+v, want no match", wrongNamespace)
	}
}

func TestCompiledIdentityAttributeFieldPathsMatchExactAndWildcardBranches(t *testing.T) {
	t.Parallel()

	fixture := compiledIdentityRuntimeForTest(t)
	otherAttr := xsdSchema.QName{Namespace: 999, Local: fixture.attrName.Local}
	namePath := []xsdSchema.RuntimeName{fixture.elemName}

	constraint, ok := fixture.rt.IdentityDispatch().Program(fixture.constraintID)
	if !ok {
		t.Fatal("IdentityDispatch().Program() rejected runtime metadata")
	}
	attributeFields := constraint.AttributeFields(fixture.attrName)
	exactAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityDispatch().Program().AttributeFields() returned no exact field")
	}
	exactPath, ok := exactAttributeField.Path(0)
	if !ok {
		t.Fatal("IdentityDispatch().Program().AttributeFields() returned no path")
	}
	exactProgram := exactPath
	if !exactProgram.Matches(fixture.rt, namePath, 1, 1) || !exactProgram.KnownAttributeMatches(fixture.attrName) {
		t.Fatal("exact attribute field path did not match")
	}
	nestedNamePath := []xsdSchema.RuntimeName{
		fixture.elemName,
		fixture.elemName,
	}
	if exactProgram.Matches(fixture.rt, nestedNamePath, 1, 2) {
		t.Fatal("direct attribute field path matched below selected depth")
	}
	attributeFields = constraint.AttributeWildcardFields()
	wildcardAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityDispatch().Program().AttributeWildcardFields() returned no wildcard field")
	}
	wildcardPath, ok := wildcardAttributeField.Path(0)
	if !ok {
		t.Fatal("IdentityDispatch().Program().AttributeWildcardFields() returned no path")
	}
	wildcardProgram := wildcardPath
	if !wildcardProgram.Matches(fixture.rt, namePath, 1, 1) || !wildcardProgram.KnownAttributeMatches(fixture.attrName) {
		t.Fatal("attribute namespace wildcard did not match")
	}
	if wildcardProgram.KnownAttributeMatches(otherAttr) {
		t.Fatal("attribute namespace wildcard matched wrong namespace")
	}
}

type compiledIdentityFixture struct {
	rt           *xsdSchema.Schema
	elemID       xsdSchema.ElementID
	constraintID xsdSchema.IdentityConstraintID
	elemName     xsdSchema.RuntimeName
	attrName     xsdSchema.QName
	attrRuntime  xsdSchema.RuntimeName
}

func compiledIdentityRuntimeForTest(tb testing.TB) compiledIdentityFixture {
	tb.Helper()
	return compiledIdentityRuntimeWithSelectorForTest(tb, ".")
}

// compiledIdentityRuntimeWithSelectorForTest compiles the shared root key with
// selector in place of ".".
func compiledIdentityRuntimeWithSelectorForTest(tb testing.TB, selector string) compiledIdentityFixture {
	tb.Helper()

	const schemaTemplate = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:a" targetNamespace="urn:a" elementFormDefault="qualified">
	<xs:attribute name="id" type="xs:string"/>
  <xs:element name="root">
	<xs:complexType mixed="true"><xs:attribute ref="t:id"/></xs:complexType>
	<xs:key name="k">
	  <xs:selector xpath="SELECTOR"/>
	  <xs:field xpath=". | @t:id | @t:*"/>
	</xs:key>
  </xs:element>
</xs:schema>`
	schema := strings.Replace(schemaTemplate, "SELECTOR", selector, 1)
	rt, err := compileSources([]source.Source{source.Bytes("identity.xsd", []byte(schema))})
	if err != nil {
		tb.Fatalf("Compile() error = %v", err)
	}
	elemName := resolveKnownNameForTest(tb, rt, "urn:a", "root")
	attrRuntime := resolveKnownNameForTest(tb, rt, "urn:a", "id")
	attrName, _ := attrRuntime.QName()
	elemID, _, ok := rt.GlobalElement(elemName)
	if !ok {
		tb.Fatal("GlobalElement(root) failed")
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
		attrRuntime:  attrRuntime,
	}
}

func BenchmarkIdentityDirectFieldPath(b *testing.B) {
	fixture := compiledIdentityRuntimeForTest(b)
	constraint, ok := fixture.rt.IdentityDispatch().Program(fixture.constraintID)
	if !ok {
		b.Fatal("IdentityDispatch().Program() rejected runtime metadata")
	}
	fields := constraint.AttributeFields(fixture.attrName)
	field, ok := fields.At(0)
	if !ok {
		b.Fatal("IdentityDispatch().Program().AttributeFields() returned no exact field")
	}
	path, ok := field.Path(0)
	if !ok {
		b.Fatal("IdentityCompiledFieldProgramRead.Path() returned no exact path")
	}
	program := path
	namePath := make([]xsdSchema.RuntimeName, 256)
	for i := range namePath {
		namePath[i] = fixture.elemName
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
