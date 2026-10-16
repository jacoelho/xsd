package xsd_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestChangedTypeQNameConstraintUsesSchemaNamespaceContext(t *testing.T) {
	for _, constraint := range []string{"default", "fixed"} {
		t.Run(constraint, func(t *testing.T) {
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:simpleType name="Declared"><xs:union memberTypes="xs:string xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"><xs:enumeration value="p:x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Declared" ` + constraint + `="p:x"/>
</xs:schema>`
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			for _, test := range []struct {
				name string
				doc  string
			}{
				{
					name: "instance prefix is absent",
					doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`,
				},
				{
					name: "instance prefix is rebound",
					doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual"/>`,
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					if err := engine.Validate(strings.NewReader(test.doc)); err != nil {
						t.Fatalf("Validate() error = %v, want schema constraint {urn:p}x to remain valid", err)
					}
				})
			}
		})
	}
}

func TestProspectiveQNameConstraintKeepsUnresolvedSchemaPrefix(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Declared"><xs:union memberTypes="xs:string xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"/></xs:simpleType>
  <xs:element name="root" type="Declared" default="p:x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("prospective-qname.xsd", []byte(source)))
	if err != nil {
		t.Fatalf("legal string default with unresolved prefix: %v", err)
	}
	for _, document := range []string{
		`<root/>`,
		`<root xmlns:p="urn:instance" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual">p:x</root>`,
	} {
		if validationErr := engine.Validate(strings.NewReader(document)); validationErr != nil {
			t.Fatalf("Validate(%s): %v", document, validationErr)
		}
	}
	err = engine.Validate(strings.NewReader(`<root xmlns:p="urn:instance" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
}

func TestChangedTypeQNameConstraintUsesSchemaDefaultNamespace(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="urn:p" xmlns="urn:p" elementFormDefault="qualified">
  <xs:simpleType name="Declared"><xs:union memberTypes="xs:string xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"><xs:enumeration value="x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Declared" fixed="x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// The root and xsi:type use an explicit schema namespace prefix so that the
	// instance default namespace can intentionally differ from the schema's
	// default namespace used to compile the unprefixed fixed QName.
	doc := `<p:root xmlns:p="urn:p" xmlns="urn:other" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:Actual"/>`
	if err := engine.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Validate() error = %v, want schema default namespace {urn:p}x", err)
	}
}

func TestChangedTypeQNameConstraintUsesSchemaNoDefaultNamespace(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="urn:p" xmlns:p="urn:p" elementFormDefault="qualified">
  <xs:simpleType name="Declared"><xs:union memberTypes="xs:string xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"><xs:enumeration value="x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="p:Declared" fixed="x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// The schema has no default namespace, so the unprefixed constraint QName
	// is in no namespace. The instance default namespace must not change that
	// prospective QName while the prefixed root and xsi:type remain qualified.
	doc := `<p:root xmlns:p="urn:p" xmlns="urn:other" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:Actual"/>`
	if err := engine.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Validate() error = %v, want schema no-namespace x", err)
	}
}

func TestQNameConstraintIdentityUsesSchemaExpandedName(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"/></xs:simpleType>
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="source" type="xs:QName" default="p:x"/>
      <xs:element name="ref"><xs:complexType>
        <xs:attribute name="value" type="xs:QName" use="required"/>
      </xs:complexType></xs:element>
    </xs:sequence></xs:complexType>
    <xs:key name="sourceName"><xs:selector xpath="source"/><xs:field xpath="."/></xs:key>
    <xs:keyref name="sourceRef" refer="sourceName"><xs:selector xpath="ref"/><xs:field xpath="@value"/></xs:keyref>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	document := func(sourceAttrs, refValue string) string {
		return `<root xmlns:p="urn:other" xmlns:q="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
			`<source` + sourceAttrs + `/><ref value="` + refValue + `"/></root>`
	}
	tests := []struct {
		name       string
		sourceAttr string
		refValue   string
		valid      bool
	}{
		{name: "same declared type matches schema expanded name", refValue: "q:x", valid: true},
		{name: "same declared type rejects rebound name", refValue: "p:x"},
		{name: "changed actual type matches schema expanded name", sourceAttr: ` xsi:type="Actual"`, refValue: "q:x", valid: true},
		{name: "changed actual type rejects rebound name", sourceAttr: ` xsi:type="Actual"`, refValue: "p:x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(document(test.sourceAttr, test.refValue)))
			if test.valid {
				if err != nil {
					t.Fatalf("Validate() error = %v, want keyref to resolve to {urn:p}x", err)
				}
				return
			}
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)
			if !errorTreeContains(err, "keyref does not resolve") {
				t.Fatalf("Validate() error = %v, want keyref mismatch for {urn:other}x", err)
			}
		})
	}
}

func TestImplicitAnyTypeQNameConstraintUsesSchemaNamespaceContext(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"><xs:enumeration value="p:x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" default="p:x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	for _, doc := range []string{
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`,
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual"/>`,
	} {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			t.Fatalf("Validate(%s) error = %v, want implicit anyType constraint {urn:p}x", doc, err)
		}
	}
}

func TestChangedTypeQNameListConstraintUsesSchemaNamespaceContext(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:simpleType name="Declared"><xs:list itemType="xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="Declared"><xs:enumeration value="p:x p:x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Declared" default="p:x p:x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	for _, doc := range []string{
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`,
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual"/>`,
	} {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			t.Fatalf("Validate(%s) error = %v, want schema list {urn:p}x {urn:p}x", doc, err)
		}
	}
}

func TestChangedTypeNotationConstraintPreservesDeclarationChecks(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="urn:p" xmlns:p="urn:p" elementFormDefault="qualified">
  <xs:notation name="n" public="notation-n"/>
  <xs:simpleType name="Declared"><xs:restriction base="xs:NOTATION"><xs:enumeration value="p:n"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="p:Declared"/></xs:simpleType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="defaulted" type="p:Declared" default="p:n"/>
    <xs:element name="value" type="p:Actual"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	valid := `<e:root xmlns:e="urn:p" xmlns:p="urn:other" xmlns:t="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><e:defaulted xsi:type="t:Actual"/><e:value>t:n</e:value></e:root>`
	err = engine.Validate(strings.NewReader(valid))
	if err != nil {
		t.Fatalf("Validate(valid NOTATION values) error = %v", err)
	}
	invalid := `<e:root xmlns:e="urn:p" xmlns:t="urn:p"><e:defaulted/><e:value>t:missing</e:value></e:root>`
	err = engine.Validate(strings.NewReader(invalid))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
}

func TestQNameConstraintSessionReuseAndConcurrentEngineValidation(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:simpleType name="Declared"><xs:union memberTypes="xs:string xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"><xs:enumeration value="p:x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Declared" fixed="p:x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	valid := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual"/>`
	invalid := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual">p:x</root>`
	for _, test := range []struct {
		name string
		doc  string
		want bool
	}{
		{name: "success", doc: valid, want: true},
		{name: "failure", doc: invalid},
		{name: "reuse after failure", doc: valid, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := session.Validate(strings.NewReader(test.doc))
			if (err == nil) != test.want {
				t.Fatalf("Validate() error = %v, want valid %v", err, test.want)
			}
		})
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 25 {
				if err := engine.Validate(strings.NewReader(valid)); err != nil {
					t.Errorf("concurrent valid Validate() error = %v", err)
				}
				if err := engine.Validate(strings.NewReader(invalid)); err == nil {
					t.Error("concurrent invalid Validate() succeeded")
				}
			}
		})
	}
	wg.Wait()
}
