package schema_test

import (
	"fmt"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeGroupUnionPreservesSharedMembers(t *testing.T) {
	for _, groups := range []string{
		`<xs:attributeGroup ref="left"/><xs:attributeGroup ref="right"/><xs:attributeGroup ref="left"/>`,
		`<xs:attributeGroup ref="right"/><xs:attributeGroup ref="left"/><xs:attributeGroup ref="right"/>`,
	} {
		t.Run(groups, func(t *testing.T) {
			// The intersection is one use component, not two equal declarations.
			engine := mustCompile(t, fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="shared"><xs:attribute name="value" type="xs:int" use="required"/></xs:attributeGroup>
  <xs:attributeGroup name="left"><xs:attributeGroup ref="shared"/><xs:attribute name="x" type="xs:string" use="required"/></xs:attributeGroup>
  <xs:attributeGroup name="right"><xs:attributeGroup ref="shared"/><xs:attribute name="y" type="xs:string" use="required" fixed="yes"/></xs:attributeGroup>
  <xs:element name="r"><xs:complexType>%s</xs:complexType></xs:element>
</xs:schema>`, groups))
			mustValidate(t, engine, `<r value="1" x="text" y="yes"/>`)
			for _, doc := range []string{
				`<r x="text" y="yes"/>`,
				`<r value="1" y="yes"/>`,
				`<r value="1" x="text"/>`,
				`<r value="1" x="text" y="no"/>`,
			} {
				mustNotValidate(t, engine, doc, xsderrors.CodeValidationAttribute)
			}
			if err := validateWithRuntime(engine, `<r value="invalid" x="text" y="yes"/>`); err == nil {
				t.Fatal("Validate() accepted an invalid shared attribute value")
			}
		})
	}
}

func TestRepeatedAttributeGroupMembersConsumeDependencyBudget(t *testing.T) {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:attributeGroup name="shared">`)
	for i := range 32 {
		fmt.Fprintf(&schema, `<xs:attribute name="a%d" type="xs:int" use="required"/>`, i)
	}
	schema.WriteString(`</xs:attributeGroup><xs:element name="r"><xs:complexType>`)
	for range 32 {
		schema.WriteString(`<xs:attributeGroup ref="shared"/>`)
	}
	schema.WriteString(`</xs:complexType></xs:element></xs:schema>`)
	inputs := []source.Source{source.Bytes("schema.xsd", []byte(schema.String()))}
	engine, err := xsdSchema.Compile(xsdSchema.Options{MaxSchemaDependencySteps: 4096}, inputs)
	if err != nil {
		t.Fatalf("Compile(adequate budget) error = %v", err)
	}
	var attributes strings.Builder
	for i := range 32 {
		fmt.Fprintf(&attributes, ` a%d="1"`, i)
	}
	mustValidate(t, engine, `<r`+attributes.String()+`/>`)
	mustNotValidate(t, engine, `<r/>`, xsderrors.CodeValidationAttribute)
	if err = validateWithRuntime(engine, `<r`+strings.Replace(attributes.String(), `a0="1"`, `a0="invalid"`, 1)+`/>`); err == nil {
		t.Fatal("Validate() accepted an invalid shared attribute value")
	}
	engine, err = xsdSchema.Compile(xsdSchema.Options{MaxSchemaDependencySteps: 512}, inputs)
	expectCode(t, err, xsderrors.CodeSchemaLimit)
	if engine != nil {
		t.Fatal("Compile(insufficient budget) returned an engine")
	}
}

func TestAttributeGroupUnionRejectsDistinctUsesOfSameDeclaration(t *testing.T) {
	_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="value" type="xs:int"/>
  <xs:attributeGroup name="left"><xs:attribute ref="value"/></xs:attributeGroup>
  <xs:attributeGroup name="right"><xs:attribute ref="value"/></xs:attributeGroup>
  <xs:element name="r"><xs:complexType><xs:attributeGroup ref="left"/><xs:attributeGroup ref="right"/></xs:complexType></xs:element>
</xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaDuplicate)
}

func TestAttributeGroupIdentitySurvivesDerivationAndReuse(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="shared"><xs:attribute name="value" type="xs:string" fixed="yes"/></xs:attributeGroup>
  <xs:complexType name="base"><xs:attributeGroup ref="shared"/></xs:complexType>
  <xs:complexType name="extended"><xs:complexContent><xs:extension base="base"><xs:attributeGroup ref="shared"/></xs:extension></xs:complexContent></xs:complexType>
  <xs:complexType name="unchanged"><xs:complexContent><xs:restriction base="base"><xs:attributeGroup ref="shared"/></xs:restriction></xs:complexContent></xs:complexType>
  <xs:complexType name="required"><xs:complexContent><xs:restriction base="base"><xs:attribute name="value" type="xs:string" use="required" fixed="yes"/></xs:restriction></xs:complexContent></xs:complexType>
  <xs:complexType name="prohibited"><xs:complexContent><xs:restriction base="base"><xs:attribute name="value" type="xs:string" use="prohibited"/></xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="extended" type="extended"/>
  <xs:element name="unchanged" type="unchanged"/>
  <xs:element name="required" type="required"/>
  <xs:element name="prohibited" type="prohibited"/>
  <xs:element name="independent"><xs:complexType><xs:attributeGroup ref="shared"/></xs:complexType></xs:element>
</xs:schema>`)
	for _, name := range []string{"extended", "unchanged", "independent"} {
		mustValidate(t, engine, fmt.Sprintf(`<%s/>`, name))
		mustValidate(t, engine, fmt.Sprintf(`<%s value="yes"/>`, name))
		mustNotValidate(t, engine, fmt.Sprintf(`<%s value="no"/>`, name), xsderrors.CodeValidationAttribute)
	}
	mustValidate(t, engine, `<required value="yes"/>`)
	mustNotValidate(t, engine, `<required/>`, xsderrors.CodeValidationAttribute)
	mustNotValidate(t, engine, `<required value="no"/>`, xsderrors.CodeValidationAttribute)
	mustValidate(t, engine, `<prohibited/>`)
	mustNotValidate(t, engine, `<prohibited value="yes"/>`, xsderrors.CodeValidationAttribute)
}
