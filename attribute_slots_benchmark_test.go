package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const slotSchemaHeader = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:slots" targetNamespace="urn:slots" elementFormDefault="qualified" attributeFormDefault="unqualified">`
const slotRootOpen = `<t:r xmlns:t="urn:slots"`

type slotOpponent struct{ name, document string }
type slotFixture struct {
	name, schema, document string
	accepted               []string
	opponents              []slotOpponent
}

type slotUseMode uint8

const (
	slotOptional slotUseMode = iota
	slotRequired
)

func slotAttributes(useMode slotUseMode) string {
	var out strings.Builder
	for i := range 128 {
		mode := ""
		if useMode == slotRequired {
			mode = ` use="required"`
		}
		fmt.Fprintf(&out, `<xs:attribute name="a%d" type="xs:string"%s/>`, i, mode)
	}
	return out.String()
}

func slotDocumentAttributes(omit int) string {
	var out strings.Builder
	for i := range 128 {
		if i != omit {
			fmt.Fprintf(&out, ` a%d="v%d"`, i, i)
		}
	}
	return out.String()
}

func attributeSlotFixtures() []slotFixture {
	wrap := func(content string) string { return slotSchemaHeader + content + `</xs:schema>` }
	element := func(attributes string) string {
		return wrap(`<xs:element name="r"><xs:complexType>` + attributes + `</xs:complexType></xs:element>`)
	}
	all := slotDocumentAttributes(-1)
	fixtures := []slotFixture{
		{name: "None", schema: element(""), document: slotRootOpen + `/>`, opponents: []slotOpponent{{"unknown", slotRootOpen + ` extra="x"/>`}}},
		{name: "OneRequired", schema: element(`<xs:attribute name="a0" type="xs:string" use="required"/>`), document: slotRootOpen + ` a0="v0"/>`, opponents: []slotOpponent{{"missing", slotRootOpen + `/>`}}},
		{name: "ManyOptional128", schema: element(slotAttributes(slotOptional)), document: slotRootOpen + all + `/>`, accepted: []string{slotRootOpen + `/>`}, opponents: []slotOpponent{{"unknown128", slotRootOpen + all + ` a128="extra"/>`}}},
		{name: "Required128", schema: element(slotAttributes(slotRequired)), document: slotRootOpen + all + `/>`, opponents: []slotOpponent{{"missing64", slotRootOpen + slotDocumentAttributes(64) + `/>`}, {"missing127", slotRootOpen + slotDocumentAttributes(127) + `/>`}}},
	}
	var constrained strings.Builder
	for i := range 128 {
		facet, literal := "default", fmt.Sprintf("d%d", i)
		if i%2 != 0 {
			facet, literal = "fixed", fmt.Sprintf("f%d", i)
		}
		if i == 0 {
			literal = ""
		}
		fmt.Fprintf(&constrained, `<xs:attribute name="a%d" type="xs:string" %s="%s"/>`, i, facet, literal)
	}
	constraintSchema := wrap(`<xs:element name="r"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="2"><xs:complexType>` + constrained.String() + `</xs:complexType></xs:element></xs:sequence></xs:complexType><xs:key name="defaultsAreValues"><xs:selector xpath="t:row"/><xs:field xpath="@a0"/></xs:key></xs:element>`)
	fixtures = append(fixtures, slotFixture{name: "Constraints128", schema: constraintSchema, document: slotRootOpen + `><t:row/></t:r>`, accepted: []string{slotRootOpen + `><t:row/><t:row a0="other"/></t:r>`}, opponents: []slotOpponent{{"duplicateDefault", slotRootOpen + `><t:row/><t:row/></t:r>`}, {"fixedMismatch", slotRootOpen + `><t:row a1="wrong"/></t:r>`}}})
	nested := wrap(`<xs:attributeGroup name="G0"><xs:attribute name="a0" type="xs:string" use="required"/><xs:attribute name="a1" type="xs:string" default="d1"/></xs:attributeGroup><xs:attributeGroup name="G1"><xs:attributeGroup ref="t:G0"/><xs:attribute name="a2" type="xs:string" use="required"/></xs:attributeGroup><xs:attributeGroup name="G2"><xs:attributeGroup ref="t:G1"/><xs:attribute name="a3" type="xs:string"/></xs:attributeGroup><xs:complexType name="B"><xs:attributeGroup ref="t:G2"/><xs:attribute name="gone" type="xs:string"/></xs:complexType><xs:complexType name="R"><xs:complexContent><xs:restriction base="t:B"><xs:attributeGroup ref="t:G2"/><xs:attribute name="gone" use="prohibited"/></xs:restriction></xs:complexContent></xs:complexType><xs:element name="r" type="t:R"/>`)
	fixtures = append(fixtures, slotFixture{name: "NestedRestriction", schema: nested, document: slotRootOpen + ` a0="v0" a2="v2"/>`, opponents: []slotOpponent{{"missing0", slotRootOpen + ` a2="v2"/>`}, {"missing2", slotRootOpen + ` a0="v0"/>`}, {"prohibited", slotRootOpen + ` a0="v0" a2="v2" gone="x"/>`}}})
	var reuse strings.Builder
	reuse.WriteString(`<xs:attributeGroup name="G">` + slotAttributes(slotRequired) + `</xs:attributeGroup>`)
	for i := range 32 {
		fmt.Fprintf(&reuse, `<xs:element name="e%d"><xs:complexType><xs:attributeGroup ref="t:G"/></xs:complexType></xs:element>`, i)
	}
	reuse.WriteString(`<xs:element name="r"><xs:complexType><xs:choice maxOccurs="unbounded">`)
	for i := range 32 {
		fmt.Fprintf(&reuse, `<xs:element ref="t:e%d"/>`, i)
	}
	reuse.WriteString(`</xs:choice></xs:complexType></xs:element>`)
	fixtures = append(fixtures, slotFixture{name: "SoleGroupReuse128", schema: wrap(reuse.String()), document: slotRootOpen + `><t:e0` + all + `/><t:e31` + all + `/></t:r>`, opponents: []slotOpponent{{"missing127", slotRootOpen + `><t:e31` + slotDocumentAttributes(127) + `/></t:r>`}}})
	return fixtures
}

func checkAttributeSlotFixture(tb testing.TB, fixture slotFixture) *xsd.Session {
	tb.Helper()
	if len(fixture.schema) > 96<<10 || len(fixture.document) > 32<<10 {
		tb.Fatal("fixture byte bound")
	}
	engine, err := xsd.Compile(xsd.Bytes("attribute-slots.xsd", []byte(fixture.schema)))
	if err != nil || engine == nil {
		tb.Fatalf("%s compile engine=%v error=%v", fixture.name, engine, err)
	}
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	valid := append([]string{fixture.document}, fixture.accepted...)
	for _, document := range valid {
		if len(document) > 32<<10 {
			tb.Fatal("document byte bound")
		}
		if validationErr := session.Validate(strings.NewReader(document)); validationErr != nil {
			tb.Fatalf("%s accepted document: %v", fixture.name, validationErr)
		}
	}
	for _, opponent := range fixture.opponents {
		if len(opponent.document) > 32<<10 {
			tb.Fatal("opponent byte bound")
		}
		validationErr := session.Validate(strings.NewReader(opponent.document))
		if validationErr == nil || !xsderrors.IsConclusiveValidation(validationErr) {
			tb.Fatalf("%s/%s requires rejection: %v", fixture.name, opponent.name, validationErr)
		}
		failures := xsderrors.Flatten(validationErr)
		if len(failures) == 0 {
			tb.Fatal("missing structured diagnostics")
		}
		for _, failure := range failures {
			diagnostic, ok := errors.AsType[*xsderrors.Error](failure)
			if !ok || diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() == "" {
				tb.Fatalf("%s/%s diagnostic=%v", fixture.name, opponent.name, failure)
			}
			wantCode := xsderrors.CodeValidationAttribute
			if fixture.name == "Constraints128" && opponent.name == "duplicateDefault" {
				wantCode = xsderrors.CodeValidationIdentity
			}
			if diagnostic.Code() != wantCode {
				tb.Fatalf("%s/%s diagnostic code = %s, want %s", fixture.name, opponent.name, diagnostic.Code(), wantCode)
			}
		}
		if recoveryErr := session.Validate(strings.NewReader(fixture.document)); recoveryErr != nil {
			tb.Fatalf("same-session %s recovery: %v", fixture.name, recoveryErr)
		}
	}
	return session
}

func TestAttributeSlotPublicationSemantics(t *testing.T) {
	for _, fixture := range attributeSlotFixtures() {
		t.Run(fixture.name, func(t *testing.T) { checkAttributeSlotFixture(t, fixture) })
	}
}

func BenchmarkCompileAttributeSlots(b *testing.B) {
	for _, fixture := range attributeSlotFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			checkAttributeSlotFixture(b, fixture)
			source := []byte(fixture.schema)
			b.ReportAllocs()
			for b.Loop() {
				engine, err := xsd.Compile(xsd.Bytes("attribute-slots.xsd", source))
				if err != nil || engine == nil {
					b.Fatalf("compile engine=%v error=%v", engine, err)
				}
			}
		})
	}
}

func BenchmarkSessionAttributeSlots(b *testing.B) {
	for _, fixture := range attributeSlotFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			session := checkAttributeSlotFixture(b, fixture)
			reader := strings.NewReader(fixture.document)
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(fixture.document)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
