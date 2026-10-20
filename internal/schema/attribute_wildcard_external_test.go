package schema_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestOrdinaryAttributeWildcardRestrictionProcessing(t *testing.T) {
	for _, content := range []string{"complexContent", "simpleContent"} {
		for _, tc := range []struct {
			base, derived string
			valid         bool
		}{
			{base: "lax", derived: "lax", valid: true},
			{base: "lax", derived: "strict", valid: true},
			{base: "lax", derived: "skip"},
			{base: "strict", derived: "lax"},
		} {
			t.Run(content+"/"+tc.base+"-to-"+tc.derived, func(t *testing.T) {
				base := fmt.Sprintf(`<xs:anyAttribute processContents="%s"/>`, tc.base)
				if content == "simpleContent" {
					base = `<xs:simpleContent><xs:extension base="xs:string">` + base + `</xs:extension></xs:simpleContent>`
				}
				schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">%s</xs:complexType>
  <xs:complexType name="R"><xs:%s><xs:restriction base="Base">
    <xs:anyAttribute namespace="urn:u" processContents="%s"/>
  </xs:restriction></xs:%s></xs:complexType>
  <xs:element name="r" type="R"/>
</xs:schema>`, base, content, tc.derived, content)
				engine, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("ordinary-wildcard.xsd", []byte(schema))})
				if tc.valid {
					if err != nil {
						t.Fatalf("Compile() error = %v", err)
					}
					mustValidate(t, engine, `<r/>`)
					return
				}
				expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
				if engine != nil {
					t.Fatal("Compile() returned an engine for weakened wildcard processing")
				}
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if !ok || diagnostic.Path() != "ordinary-wildcard.xsd" || diagnostic.Line() == 0 {
					t.Fatalf("Compile() error = %v; want source path and line", err)
				}
			})
		}
	}
}

func TestAnyTypeAttributeWildcardPreservesProcessing(t *testing.T) {
	for _, process := range []string{"skip", "lax", "strict"} {
		t.Run(process, func(t *testing.T) {
			engine := mustCompile(t, fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:u" targetNamespace="urn:u">
  <xs:attribute name="known" type="xs:int"/>
  <xs:complexType name="R"><xs:complexContent><xs:restriction base="xs:anyType">
    <xs:anyAttribute namespace="urn:u" processContents="%s"/>
  </xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="r" type="t:R"/>
</xs:schema>`, process))
			mustValidate(t, engine, `<t:r xmlns:t="urn:u" t:known="1"/>`)
			invalidValue := `<t:r xmlns:t="urn:u" t:known="invalid"/>`
			if process == "skip" {
				mustValidate(t, engine, invalidValue)
			} else if err := validateWithRuntime(engine, invalidValue); err == nil {
				t.Fatal("Validate() accepted an invalid value under " + process)
			}
			mustNotValidate(t, engine, `<t:r xmlns:t="urn:u" xmlns:o="urn:outside" o:known="1"/>`, xsderrors.CodeValidationAttribute)
		})
	}
}

func TestPublishSchemaRejectsWeakenedOrdinaryAttributeWildcardAndAllowsRetry(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:u" targetNamespace="urn:u">
  <xs:attribute name="known" type="xs:int"/>
  <xs:complexType name="Base" mixed="true"><xs:complexContent mixed="true"><xs:extension base="xs:anyType"/></xs:complexContent></xs:complexType>
  <xs:complexType name="R"><xs:complexContent><xs:restriction base="t:Base">
    <xs:anyAttribute namespace="urn:u" processContents="strict"/>
  </xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="r" type="t:R"/>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(build); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	expected := mutableSchemaBuild(t, schema)
	var changed []xsdSchema.WildcardID
	for _, current := range []*xsdSchema.SchemaBuild{build, expected} {
		q, ok := current.Names.LookupQName("urn:u", "R")
		if !ok {
			t.Fatal("missing name R")
		}
		id, ok := current.GlobalTypes[q].Complex()
		if !ok {
			t.Fatal("missing complex type R")
		}
		uses := current.AttributeUseSets[current.ComplexTypes[id].Attrs]
		wildcards := []xsdSchema.WildcardID{uses.Wildcard, uses.WildcardDeclared}
		if current == build {
			changed = wildcards
		}
		for _, wildcard := range wildcards {
			current.Wildcards[wildcard].Process = xsdSchema.ProcessSkip
		}
	}
	published, err := publishSchema(build)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if published != nil {
		t.Fatal("PublishSchema() accepted weakened processing for an ordinary base")
	}
	if !reflect.DeepEqual(*build, *expected) {
		t.Fatal("PublishSchema() changed build after attribute wildcard audit failure")
	}
	for _, wildcard := range changed {
		build.Wildcards[wildcard].Process = xsdSchema.ProcessStrict
	}
	published, err = publishSchema(build)
	if err != nil {
		t.Fatalf("PublishSchema() retry error = %v", err)
	}
	mustValidate(t, published, `<t:r xmlns:t="urn:u" t:known="1"/>`)
	if err = validateWithRuntime(published, `<t:r xmlns:t="urn:u" t:known="invalid"/>`); err == nil {
		t.Fatal("Validate() accepted an invalid value after strict-processing repair")
	}
}
