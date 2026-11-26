package schema_test

import (
	"errors"
	"fmt"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

const xsd11TestSchemaStart = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:f="urn:foreign">`

type xsd11VocabularyUse struct {
	name    string
	xml     string
	element string
	attr    string
	normal  string
}

func (u xsd11VocabularyUse) message() string {
	if u.attr == "" {
		return "XSD 1.1 feature " + u.element + " is not supported"
	}
	return "XSD 1.1 attribute " + u.attr + " on xs:" + u.element + " is not supported"
}

// xsd11VocabularyUses lists one use of every vocabulary pair that the XSD 1.1
// schema for schemas adds over XSD 1.0, each with a context where XSD 1.1
// permits it.
var xsd11VocabularyUses = []xsd11VocabularyUse{
	{name: "assert", xml: `<xs:assert test="true()"/>`, element: "assert", normal: `<xs:complexType name="T">%s</xs:complexType>`},
	{name: "assertion", xml: `<xs:assertion test="true()"/>`, element: "assertion", normal: `<xs:simpleType name="T"><xs:restriction base="xs:int">%s</xs:restriction></xs:simpleType>`},
	{name: "explicitTimezone", xml: `<xs:explicitTimezone value="required"/>`, element: "explicitTimezone", normal: `<xs:simpleType name="T"><xs:restriction base="xs:date">%s</xs:restriction></xs:simpleType>`},
	{name: "alternative", xml: `<xs:alternative type="xs:int"/>`, element: "alternative", normal: `<xs:element name="e">%s</xs:element>`},
	{name: "override", xml: `<xs:override schemaLocation="absent.xsd"/>`, element: "override", normal: `%s`},
	{name: "defaultOpenContent", xml: `<xs:defaultOpenContent><xs:any/></xs:defaultOpenContent>`, element: "defaultOpenContent", normal: `%s`},
	{name: "openContent", xml: `<xs:openContent><xs:any/></xs:openContent>`, element: "openContent", normal: `<xs:complexType name="T">%s</xs:complexType>`},
	{name: "schema defaultAttributes", xml: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" defaultAttributes="G"/>`, element: "schema", attr: "defaultAttributes"},
	{name: "schema xpathDefaultNamespace", xml: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xpathDefaultNamespace="##local"/>`, element: "schema", attr: "xpathDefaultNamespace"},
	{name: "complexType defaultAttributesApply", xml: `<xs:complexType defaultAttributesApply="false"/>`, element: "complexType", attr: "defaultAttributesApply", normal: `<xs:element name="e">%s</xs:element>`},
	{name: "element targetNamespace", xml: `<xs:element name="c" targetNamespace="urn:x"/>`, element: "element", attr: "targetNamespace", normal: `<xs:complexType name="T"><xs:sequence>%s</xs:sequence></xs:complexType>`},
	{name: "attribute targetNamespace", xml: `<xs:attribute name="a" targetNamespace="urn:x"/>`, element: "attribute", attr: "targetNamespace", normal: `<xs:complexType name="T">%s</xs:complexType>`},
	{name: "attribute inheritable", xml: `<xs:attribute name="a" inheritable="true"/>`, element: "attribute", attr: "inheritable", normal: `%s`},
	{name: "any notNamespace", xml: `<xs:any notNamespace="urn:x"/>`, element: "any", attr: "notNamespace", normal: `<xs:complexType name="T"><xs:sequence>%s</xs:sequence></xs:complexType>`},
	{name: "any notQName", xml: `<xs:any notQName="a"/>`, element: "any", attr: "notQName", normal: `<xs:complexType name="T"><xs:sequence>%s</xs:sequence></xs:complexType>`},
	{name: "anyAttribute notNamespace", xml: `<xs:anyAttribute notNamespace="urn:x"/>`, element: "anyAttribute", attr: "notNamespace", normal: `<xs:complexType name="T">%s</xs:complexType>`},
	{name: "anyAttribute notQName", xml: `<xs:anyAttribute notQName="a"/>`, element: "anyAttribute", attr: "notQName", normal: `<xs:complexType name="T">%s</xs:complexType>`},
	{name: "selector xpathDefaultNamespace", xml: `<xs:selector xpath="." xpathDefaultNamespace="##local"/>`, element: "selector", attr: "xpathDefaultNamespace", normal: `<xs:element name="e"><xs:unique name="u">%s<xs:field xpath="."/></xs:unique></xs:element>`},
	{name: "field xpathDefaultNamespace", xml: `<xs:field xpath="." xpathDefaultNamespace="##local"/>`, element: "field", attr: "xpathDefaultNamespace", normal: `<xs:element name="e"><xs:unique name="u"><xs:selector xpath="."/>%s</xs:unique></xs:element>`},
	{name: "unique ref", xml: `<xs:unique ref="k"/>`, element: "unique", attr: "ref", normal: `<xs:element name="e">%s</xs:element>`},
	{name: "key ref", xml: `<xs:key ref="k"/>`, element: "key", attr: "ref", normal: `<xs:element name="e">%s</xs:element>`},
	{name: "keyref ref", xml: `<xs:keyref ref="k"/>`, element: "keyref", attr: "ref", normal: `<xs:element name="e">%s</xs:element>`},
}

// xsd11Contexts are positions where XSD 1.0 permits none of
// xsd11VocabularyUses and XSD 1.1 permits few or none of them.
var xsd11Contexts = map[string]string{
	"top level":              `%s`,
	"annotation":             `<xs:annotation>%s</xs:annotation>`,
	"facet":                  `<xs:simpleType name="S"><xs:restriction base="xs:string"><xs:minLength value="1">%s</xs:minLength></xs:restriction></xs:simpleType>`,
	"element reference":      `<xs:element name="r"/><xs:complexType name="C"><xs:sequence><xs:element ref="r">%s</xs:element></xs:sequence></xs:complexType>`,
	"attribute reference":    `<xs:attribute name="a"/><xs:complexType name="C"><xs:attribute ref="a">%s</xs:attribute></xs:complexType>`,
	"after text in sequence": `<xs:complexType name="C"><xs:sequence>text%s</xs:sequence></xs:complexType>`,
}

func compileXSD11TestSchema(schema string) error {
	compiled, err := compileSources(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err == nil && compiled == nil {
		return errors.New("Compile returned neither a schema nor an error")
	}
	return err
}

type xsd11Diagnostic struct {
	code         xsderrors.Code
	message      string
	line, column int
}

func expectXSD11Diagnostic(t *testing.T, schema string, err error, want xsd11Diagnostic) {
	t.Helper()
	code, message, line, column := want.code, want.message, want.line, want.column
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("Compile(%s) error = %T %[2]v, want structured diagnostic %s", schema, err, code)
	}
	if diagnostic.Code() != code || (message != "" && diagnostic.Message() != message) {
		t.Fatalf("Compile(%s) diagnostic = (%s, %q), want (%s, %q)", schema, diagnostic.Code(), diagnostic.Message(), code, message)
	}
	if line != 0 && (diagnostic.Line() != line || diagnostic.Column() != column) {
		t.Fatalf("Compile(%s) location = %d:%d, want %d:%d", schema, diagnostic.Line(), diagnostic.Column(), line, column)
	}
}

func TestXSD11VocabularyIsUnsupportedWhereXSD11PermitsIt(t *testing.T) {
	t.Parallel()
	for _, use := range xsd11VocabularyUses {
		t.Run(use.name, func(t *testing.T) {
			t.Parallel()
			schema := xsd11TestSchemaStart + fmt.Sprintf(use.normal, use.xml) + `</xs:schema>`
			if use.element == "schema" {
				schema = use.xml
			}
			expectXSD11Diagnostic(t, schema, compileXSD11TestSchema(schema), xsd11Diagnostic{code: xsderrors.CodeUnsupportedXSD11, message: use.message()})
		})
	}
}

func TestXSD11VocabularyIsUnsupportedInEveryContext(t *testing.T) {
	t.Parallel()
	for contextName, context := range xsd11Contexts {
		for _, use := range xsd11VocabularyUses {
			t.Run(contextName+"/"+use.name, func(t *testing.T) {
				t.Parallel()
				schema := xsd11TestSchemaStart + fmt.Sprintf(context, use.xml) + `</xs:schema>`
				expectXSD11Diagnostic(t, schema, compileXSD11TestSchema(schema), xsd11Diagnostic{code: xsderrors.CodeUnsupportedXSD11, message: use.message()})
			})
		}
	}
}

func TestXSD11VocabularyIsUnsupportedWhateverItsValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, message string
	}{
		{name: "inheritable not a boolean", body: `<xs:attribute name="a" inheritable="maybe"/>`, message: "XSD 1.1 attribute inheritable on xs:attribute is not supported"},
		{name: "empty notNamespace", body: `<xs:complexType name="T"><xs:anyAttribute notNamespace=""/></xs:complexType>`, message: "XSD 1.1 attribute notNamespace on xs:anyAttribute is not supported"},
		{name: "notNamespace with namespace", body: `<xs:complexType name="T"><xs:anyAttribute namespace="##any" notNamespace="urn:x"/></xs:complexType>`, message: "XSD 1.1 attribute notNamespace on xs:anyAttribute is not supported"},
		{name: "notQName not a QName", body: `<xs:complexType name="T"><xs:anyAttribute notQName="1a"/></xs:complexType>`, message: "XSD 1.1 attribute notQName on xs:anyAttribute is not supported"},
		{name: "key ref not a QName", body: `<xs:element name="e"><xs:key ref="::"/></xs:element>`, message: "XSD 1.1 attribute ref on xs:key is not supported"},
		{name: "key ref with name", body: `<xs:element name="e"><xs:key name="k2" ref="k"/></xs:element>`, message: "XSD 1.1 attribute ref on xs:key is not supported"},
		{name: "keyref ref with refer", body: `<xs:element name="e"><xs:keyref ref="k" refer="k"/></xs:element>`, message: "XSD 1.1 attribute ref on xs:keyref is not supported"},
		{name: "unique ref with selector", body: `<xs:element name="e"><xs:unique ref="k"><xs:selector xpath="."/></xs:unique></xs:element>`, message: "XSD 1.1 attribute ref on xs:unique is not supported"},
		{name: "unique ref with text", body: `<xs:element name="e"><xs:unique ref="k">text</xs:unique></xs:element>`, message: "XSD 1.1 attribute ref on xs:unique is not supported"},
		{name: "defaultAttributesApply not a boolean", body: `<xs:complexType name="T" defaultAttributesApply="yes"/>`, message: "XSD 1.1 attribute defaultAttributesApply on xs:complexType is not supported"},
		{name: "unnamed top-level complexType", body: `<xs:complexType defaultAttributesApply="true"/>`, message: "XSD 1.1 attribute defaultAttributesApply on xs:complexType is not supported"},
		{name: "top-level element targetNamespace", body: `<xs:element name="e" targetNamespace="urn:x"/>`, message: "XSD 1.1 attribute targetNamespace on xs:element is not supported"},
		{name: "element reference targetNamespace", body: `<xs:element name="r"/><xs:complexType name="C"><xs:sequence><xs:element ref="r" targetNamespace="urn:x"/></xs:sequence></xs:complexType>`, message: "XSD 1.1 attribute targetNamespace on xs:element is not supported"},
		{name: "attribute with default and fixed", body: `<xs:attribute name="a" default="1" fixed="1" inheritable="true"/>`, message: "XSD 1.1 attribute inheritable on xs:attribute is not supported"},
		{name: "assert with child content", body: `<xs:complexType name="T"><xs:assert test="true()">text<xs:bogus/></xs:assert></xs:complexType>`, message: "XSD 1.1 feature assert is not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema := xsd11TestSchemaStart + tc.body + `</xs:schema>`
			expectXSD11Diagnostic(t, schema, compileXSD11TestSchema(schema), xsd11Diagnostic{code: xsderrors.CodeUnsupportedXSD11, message: tc.message})
		})
	}
}

func TestXSD11VocabularyPrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		schema  string
		code    xsderrors.Code
		message string
		line    int
		column  int
	}{
		{
			name:   "earlier sibling XSD 1.0 error wins",
			schema: xsd11TestSchemaStart + "\n<xs:element name=\"e\" foo=\"\"/>\n<xs:assert/>\n</xs:schema>",
			code:   xsderrors.CodeSchemaInvalidAttribute, message: `"element" cannot have attribute "foo"`, line: 2, column: 1,
		},
		{
			name:   "ancestor start XSD 1.0 error wins",
			schema: xsd11TestSchemaStart + "\n<xs:complexType name=\"T\" foo=\"\">\n<xs:assert/>\n</xs:complexType>\n</xs:schema>",
			code:   xsderrors.CodeSchemaInvalidAttribute, message: `"complexType" cannot have attribute "foo"`, line: 2, column: 1,
		},
		{
			name:   "vocabulary wins over an earlier XSD 1.0 attribute error on the same start tag",
			schema: xsd11TestSchemaStart + "\n<xs:attribute foo=\"\" name=\"a\" inheritable=\"true\"/>\n</xs:schema>",
			code:   xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 attribute inheritable on xs:attribute is not supported", line: 2, column: 1,
		},
		{
			name:   "vocabulary wins over a schema-namespace attribute and an invalid name",
			schema: xsd11TestSchemaStart + "\n<xs:complexType name=\"T\"><xs:assert xs:foo=\"\" name=\"\"/></xs:complexType>\n</xs:schema>",
			code:   xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 feature assert is not supported", line: 2, column: 26,
		},
		{
			name:   "vocabulary wins over earlier text in an annotation-only parent",
			schema: xsd11TestSchemaStart + "\n<xs:element name=\"r\"/><xs:group name=\"G\"><xs:sequence><xs:element ref=\"r\">text<xs:assert/></xs:element></xs:sequence></xs:group>\n</xs:schema>",
			code:   xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 feature assert is not supported", line: 2, column: 79,
		},
		{
			name:   "first vocabulary attribute in start-tag order wins",
			schema: xsd11TestSchemaStart + "\n<xs:attribute name=\"a\" inheritable=\"true\" targetNamespace=\"urn:x\"/>\n</xs:schema>",
			code:   xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 attribute inheritable on xs:attribute is not supported", line: 2, column: 1,
		},
		{
			name:   "first vocabulary element in document order wins",
			schema: xsd11TestSchemaStart + "\n<xs:override schemaLocation=\"absent.xsd\"/>\n<xs:attribute name=\"a\" inheritable=\"true\"/>\n</xs:schema>",
			code:   xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 feature override is not supported", line: 2, column: 1,
		},
		{
			name:   "root check wins over an XSD 1.1 root element",
			schema: `<xs:override xmlns:xs="http://www.w3.org/2001/XMLSchema" schemaLocation="absent.xsd"/>`,
			code:   xsderrors.CodeSchemaRoot, message: "root element must be xs:schema", line: 1, column: 1,
		},
		{
			name:   "root check wins over an XSD 1.1 attribute of a non-schema root",
			schema: `<xs:element xmlns:xs="http://www.w3.org/2001/XMLSchema" name="e" targetNamespace="urn:x"/>`,
			code:   xsderrors.CodeSchemaRoot, message: "root element must be xs:schema", line: 1, column: 1,
		},
		{
			name:   "XSD 1.1 attribute of the schema root is vocabulary",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" foo="" defaultAttributes="G"/>`,
			code:   xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 attribute defaultAttributes on xs:schema is not supported", line: 1, column: 1,
		},
		{
			name:   "misplaced appinfo content is not vocabulary",
			schema: xsd11TestSchemaStart + "\n<xs:complexType name=\"T\"><xs:appinfo><xs:assert/></xs:appinfo></xs:complexType>\n</xs:schema>",
			code:   xsderrors.CodeSchemaContentModel, message: `xs:"appinfo" can appear only in xs:annotation`, line: 2, column: 26,
		},
		{
			name:   "misplaced documentation content is not vocabulary",
			schema: xsd11TestSchemaStart + "\n<xs:documentation><xs:override/></xs:documentation>\n</xs:schema>",
			code:   xsderrors.CodeSchemaContentModel, message: `xs:"documentation" can appear only in xs:annotation`, line: 2, column: 1,
		},
		{
			name:   "nested schema attribute is vocabulary",
			schema: xsd11TestSchemaStart + "\n<xs:element name=\"e\"><xs:schema defaultAttributes=\"G\"/></xs:element>\n</xs:schema>",
			code:   xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 attribute defaultAttributes on xs:schema is not supported", line: 2, column: 22,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expectXSD11Diagnostic(t, tc.schema, compileXSD11TestSchema(tc.schema), xsd11Diagnostic{code: tc.code, message: tc.message, line: tc.line, column: tc.column})
		})
	}
}

func TestXSD11NamesOutsideTheVocabularyAreNotUnsupported(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		body string
		code xsderrors.Code
	}{
		{name: "appinfo payload", body: `<xs:annotation><xs:appinfo><xs:assert/><xs:key ref="k"/></xs:appinfo></xs:annotation>`},
		{name: "documentation payload", body: `<xs:annotation><xs:documentation><xs:override/></xs:documentation></xs:annotation>`},
		{name: "foreign attribute with a vocabulary local name", body: `<xs:attribute name="a" f:inheritable="true"/>`},
		{name: "schema-namespace attribute with a vocabulary local name", body: `<xs:complexType name="T"><xs:sequence><xs:any xs:notQName=""/></xs:sequence></xs:complexType>`, code: xsderrors.CodeSchemaInvalidAttribute},
		{name: "vocabulary attribute on another XSD element", body: `<xs:group name="G" inheritable="true"><xs:sequence/></xs:group>`, code: xsderrors.CodeSchemaInvalidAttribute},
		{name: "abstract facet head", body: `<xs:simpleType name="S"><xs:restriction base="xs:string"><xs:facet value="1"/></xs:restriction></xs:simpleType>`, code: xsderrors.CodeSchemaContentModel},
		{name: "foreign element with a vocabulary local name", body: `<f:assert/>`, code: xsderrors.CodeSchemaContentModel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema := xsd11TestSchemaStart + tc.body + `</xs:schema>`
			err := compileXSD11TestSchema(schema)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("Compile(%s) error = %v, want success", schema, err)
				}
				return
			}
			expectXSD11Diagnostic(t, schema, err, xsd11Diagnostic{code: tc.code})
		})
	}
}
