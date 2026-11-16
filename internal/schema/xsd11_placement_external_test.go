package schema_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

// TestXSD11VocabularyPlacement places every XSD 1.1 element and every XSD
// 1.0 element carrying an XSD 1.1 attribute under each XSD parent context.
// The permitted contexts come from the st11.html XML Representation
// Summaries; elsewhere the use is invalid under XSD 1.1 too, so it must fail
// with an XSD 1.0 diagnostic. No placement may compile.
func TestXSD11VocabularyPlacement(t *testing.T) {
	t.Parallel()
	contexts := map[string]string{
		"schema":                     `%s<xs:element name="r"/>`,
		"element":                    `<xs:element name="r">%s</xs:element>`,
		"element ref":                `<xs:element name="r"/><xs:group name="G"><xs:sequence><xs:element ref="r">%s</xs:element></xs:sequence></xs:group>`,
		"complexType":                `<xs:complexType name="T">%s</xs:complexType>`,
		"complexContent restriction": `<xs:complexType name="T"><xs:complexContent><xs:restriction base="xs:anyType">%s</xs:restriction></xs:complexContent></xs:complexType>`,
		"complexContent extension":   `<xs:complexType name="T"><xs:complexContent><xs:extension base="xs:anyType">%s</xs:extension></xs:complexContent></xs:complexType>`,
		"simpleContent restriction":  `<xs:complexType name="S"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType><xs:complexType name="T"><xs:simpleContent><xs:restriction base="S">%s</xs:restriction></xs:simpleContent></xs:complexType>`,
		"simpleContent extension":    `<xs:complexType name="T"><xs:simpleContent><xs:extension base="xs:string">%s</xs:extension></xs:simpleContent></xs:complexType>`,
		"simpleType restriction":     `<xs:simpleType name="T"><xs:restriction base="xs:string">%s</xs:restriction></xs:simpleType>`,
		"list":                       `<xs:simpleType name="T"><xs:list>%s</xs:list></xs:simpleType>`,
		"sequence":                   `<xs:complexType name="T"><xs:sequence>%s</xs:sequence></xs:complexType>`,
		"choice":                     `<xs:complexType name="T"><xs:choice>%s</xs:choice></xs:complexType>`,
		"all":                        `<xs:complexType name="T"><xs:all>%s</xs:all></xs:complexType>`,
		"group":                      `<xs:group name="G">%s</xs:group>`,
		"attributeGroup":             `<xs:attributeGroup name="G">%s</xs:attributeGroup>`,
		"unique":                     `<xs:element name="r"><xs:unique name="u">%s</xs:unique></xs:element>`,
		"selector":                   `<xs:element name="r"><xs:unique name="u"><xs:selector xpath=".">%s</xs:selector><xs:field xpath="."/></xs:unique></xs:element>`,
		"misplaced XSD 1.1 ancestor": `<xs:complexType name="T"><xs:sequence><xs:alternative><xs:complexType><xs:sequence><xs:choice>%s</xs:choice></xs:sequence></xs:complexType></xs:alternative></xs:sequence></xs:complexType>`,
		"attributeGroup reference":   `<xs:attributeGroup name="G"/><xs:complexType name="T"><xs:attributeGroup ref="G">%s</xs:attributeGroup></xs:complexType>`,
		"key outside element":        `<xs:complexType name="T"><xs:key name="k">%s</xs:key></xs:complexType>`,
		"nested schema":              `<xs:element name="r"><xs:schema>%s</xs:schema></xs:element>`,
	}
	complexDerivations := []string{"complexType", "complexContent restriction", "complexContent extension", "simpleContent restriction", "simpleContent extension"}
	attributeUses := append(slices.Clone(complexDerivations), "attributeGroup")
	particles := []string{"sequence", "choice", "all"}
	uses := []struct {
		name      string
		xml       string
		permitted []string
	}{
		{name: "assert", xml: `<xs:assert test="true()"/>`, permitted: complexDerivations},
		{name: "assertion", xml: `<xs:assertion test="true()"/>`, permitted: []string{"simpleType restriction", "simpleContent restriction"}},
		{name: "explicitTimezone", xml: `<xs:explicitTimezone value="optional"/>`, permitted: []string{"simpleType restriction", "simpleContent restriction"}},
		{name: "alternative", xml: `<xs:alternative type="xs:string"/>`, permitted: []string{"element"}},
		{name: "override", xml: `<xs:override schemaLocation="absent.xsd"/>`, permitted: []string{"schema"}},
		{name: "defaultOpenContent", xml: `<xs:defaultOpenContent><xs:any/></xs:defaultOpenContent>`, permitted: []string{"schema"}},
		{name: "openContent", xml: `<xs:openContent><xs:any/></xs:openContent>`, permitted: []string{"complexType", "complexContent restriction", "complexContent extension"}},
		{name: "any notNamespace", xml: `<xs:any notNamespace="urn:x"/>`, permitted: particles},
		{name: "any namespace and notNamespace", xml: `<xs:any namespace="urn:y" notNamespace="urn:x"/>`},
		{name: "anyAttribute notQName", xml: `<xs:anyAttribute notQName="a"/>`, permitted: attributeUses},
		{name: "local complexType defaultAttributesApply", xml: `<xs:complexType defaultAttributesApply="false"/>`, permitted: []string{"element"}},
		{name: "named complexType defaultAttributesApply", xml: `<xs:complexType name="C" defaultAttributesApply="false"/>`, permitted: []string{"schema"}},
		{name: "attribute inheritable", xml: `<xs:attribute name="a" inheritable="true"/>`, permitted: append([]string{"schema"}, attributeUses...)},
		{name: "attribute targetNamespace", xml: `<xs:attribute name="a" targetNamespace="urn:x"/>`, permitted: attributeUses},
		{name: "attribute ref targetNamespace", xml: `<xs:attribute ref="a" targetNamespace="urn:x"/>`},
		{name: "attribute form targetNamespace", xml: `<xs:attribute name="a" form="qualified" targetNamespace="urn:x"/>`},
		{name: "element targetNamespace", xml: `<xs:element name="e" targetNamespace="urn:x"/>`, permitted: particles},
		{name: "element ref targetNamespace", xml: `<xs:element ref="e" targetNamespace="urn:x"/>`},
		{name: "unique ref", xml: `<xs:unique ref="k"/>`, permitted: []string{"element"}},
		{name: "key ref", xml: `<xs:key ref="k"/>`, permitted: []string{"element"}},
		{name: "keyref ref", xml: `<xs:keyref ref="k"/>`, permitted: []string{"element"}},
		{name: "keyref ref and refer", xml: `<xs:keyref ref="k" refer="k"/>`},
		{name: "unique ref and name", xml: `<xs:unique name="u2" ref="k"/>`},
		{name: "unique ref with selector", xml: `<xs:unique ref="k"><xs:selector xpath="."/><xs:field xpath="."/></xs:unique>`},
		{name: "unique ref with annotation", xml: `<xs:unique ref="k"><xs:annotation/></xs:unique>`, permitted: []string{"element"}},
		{name: "unique ref with two annotations", xml: `<xs:unique ref="k"><xs:annotation/><xs:annotation/></xs:unique>`},
		{name: "unique ref with text", xml: `<xs:unique ref="k">text</xs:unique>`},
		{name: "attribute inheritable with name and ref", xml: `<xs:attribute name="b" ref="a" inheritable="true"/>`},
		{name: "attribute inheritable without name or ref", xml: `<xs:attribute inheritable="true"/>`},
		{name: "attribute ref inheritable with type", xml: `<xs:attribute ref="a" type="xs:int" inheritable="true"/>`},
		{name: "attribute ref inheritable", xml: `<xs:attribute ref="a" inheritable="true"/>`, permitted: attributeUses},
		{name: "attribute inheritable with default and fixed", xml: `<xs:attribute name="a" default="1" fixed="1" inheritable="true"/>`},
		{name: "attribute targetNamespace with default and required use", xml: `<xs:attribute name="a" default="1" use="required" targetNamespace="urn:x"/>`},
		{name: "attribute targetNamespace with fixed and prohibited use", xml: `<xs:attribute name="a" fixed="1" use="prohibited" targetNamespace="urn:x"/>`},
		{name: "element targetNamespace with default and fixed", xml: `<xs:element name="e" default="1" fixed="1" targetNamespace="urn:x"/>`},
		{name: "selector xpathDefaultNamespace", xml: `<xs:selector xpath="." xpathDefaultNamespace="##local"/>`, permitted: []string{"unique"}},
		{name: "field xpathDefaultNamespace", xml: `<xs:field xpath="." xpathDefaultNamespace="##local"/>`, permitted: []string{"unique"}},
		{name: "schema defaultAttributes", xml: `<xs:schema defaultAttributes="G"/>`},
	}
	for contextName, context := range contexts {
		for _, use := range uses {
			t.Run(contextName+"/"+use.name, func(t *testing.T) {
				t.Parallel()
				schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + fmt.Sprintf(context, use.xml) + `</xs:schema>`
				compiled, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
				if compiled != nil || err == nil {
					t.Fatalf("Compile(%s) succeeded, want an error", schema)
				}
				wantUnsupported := slices.Contains(use.permitted, contextName)
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if !ok {
					t.Fatalf("Compile(%s) error = %T %[2]v, want structured diagnostic", schema, err)
				}
				if gotUnsupported := diagnostic.Code() == xsderrors.CodeUnsupportedXSD11; gotUnsupported != wantUnsupported {
					t.Fatalf("Compile(%s) error = %v, want unsupported.xsd_1_1 = %t", schema, err, wantUnsupported)
				}
			})
		}
	}
}
