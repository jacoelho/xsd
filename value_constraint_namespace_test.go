package xsd_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestQNameConstraintResolvesInInstanceScope(t *testing.T) {
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
				name  string
				doc   string
				valid bool
			}{
				{
					name:  "instance prefix is bound to the admitted namespace",
					doc:   `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:p" xsi:type="Actual"/>`,
					valid: true,
				},
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
					err := engine.Validate(strings.NewReader(test.doc))
					if test.valid {
						if err != nil {
							t.Fatalf("Validate() error = %v", err)
						}
						return
					}
					expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
				})
			}
		})
	}
}

func TestQNameConstraintSpellingNeedNotResolveInSchema(t *testing.T) {
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
		`<root xmlns:p="urn:instance" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`,
	} {
		if validationErr := engine.Validate(strings.NewReader(document)); validationErr != nil {
			t.Fatalf("Validate(%s): %v", document, validationErr)
		}
	}
	err = engine.Validate(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
}

func TestUnprefixedQNameConstraintUsesInstanceDefaultNamespace(t *testing.T) {
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
	const sameDefault = `<p:root xmlns:p="urn:p" xmlns="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:Actual"/>`
	if err := engine.Validate(strings.NewReader(sameDefault)); err != nil {
		t.Fatalf("Validate(instance default namespace urn:p) error = %v", err)
	}
	for _, doc := range []string{
		`<p:root xmlns:p="urn:p" xmlns="urn:other" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:Actual"/>`,
		`<p:root xmlns:p="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:Actual"/>`,
	} {
		expectCategoryCode(t, engine.Validate(strings.NewReader(doc)), xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
	}
}

func TestQNameConstraintIdentityUsesInstanceExpandedName(t *testing.T) {
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
		{name: "declared type matches instance expanded name", refValue: "p:x", valid: true},
		{name: "declared type rejects schema expanded name", refValue: "q:x"},
		{name: "changed actual type matches instance expanded name", sourceAttr: ` xsi:type="Actual"`, refValue: "p:x", valid: true},
		{name: "changed actual type rejects schema expanded name", sourceAttr: ` xsi:type="Actual"`, refValue: "q:x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(document(test.sourceAttr, test.refValue)))
			if test.valid {
				if err != nil {
					t.Fatalf("Validate() error = %v, want keyref to resolve to {urn:other}x", err)
				}
				return
			}
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)
			if !errorTreeContains(err, `keyref value does not match any value of referenced identity constraint "sourceName"`) {
				t.Fatalf("Validate() error = %v, want keyref mismatch for {urn:p}x", err)
			}
		})
	}
}

func TestImplicitAnyTypeQNameConstraintResolvesInInstanceScope(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"><xs:enumeration value="p:x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" default="p:x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	const bound = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:p" xsi:type="Actual"/>`
	if err := engine.Validate(strings.NewReader(bound)); err != nil {
		t.Fatalf("Validate(%s) error = %v", bound, err)
	}
	for _, doc := range []string{
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`,
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual"/>`,
	} {
		expectCategoryCode(t, engine.Validate(strings.NewReader(doc)), xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
	}
}

func TestQNameListConstraintResolvesInInstanceScope(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:simpleType name="Declared"><xs:list itemType="xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="Declared"><xs:enumeration value="p:x p:x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Declared" default="p:x p:x"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	const bound = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:p" xsi:type="Actual"/>`
	if err := engine.Validate(strings.NewReader(bound)); err != nil {
		t.Fatalf("Validate(%s) error = %v", bound, err)
	}
	for _, doc := range []string{
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`,
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual"/>`,
	} {
		expectCategoryCode(t, engine.Validate(strings.NewReader(doc)), xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
	}
}

func TestNotationConstraintResolvesInInstanceScopeAndKeepsDeclarationChecks(t *testing.T) {
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

	valid := `<e:root xmlns:e="urn:p" xmlns:p="urn:p" xmlns:t="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><e:defaulted xsi:type="t:Actual"/><e:value>t:n</e:value></e:root>`
	err = engine.Validate(strings.NewReader(valid))
	if err != nil {
		t.Fatalf("Validate(valid NOTATION values) error = %v", err)
	}
	for _, invalid := range []string{
		`<e:root xmlns:e="urn:p" xmlns:p="urn:p" xmlns:t="urn:p"><e:defaulted/><e:value>t:missing</e:value></e:root>`,
		`<e:root xmlns:e="urn:p" xmlns:p="urn:other" xmlns:t="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><e:defaulted xsi:type="t:Actual"/><e:value>t:n</e:value></e:root>`,
	} {
		expectCategoryCode(t, engine.Validate(strings.NewReader(invalid)), xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
	}
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
	valid := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:p" xsi:type="Actual"/>`
	invalid := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:other" xsi:type="Actual"/>`
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
