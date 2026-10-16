package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type canonicalChildSourceOpponent struct {
	document string
	code     xsderrors.Code
}

type canonicalChildSourceFixture struct {
	name, schema string
	accepted     []string
	rejected     []canonicalChildSourceOpponent
}

func canonicalChildSourceFixtures(tb testing.TB) []canonicalChildSourceFixture {
	tb.Helper()
	var attributes strings.Builder
	for index := range 128 {
		fmt.Fprintf(&attributes, `<xs:attribute name="a%d" type="xs:string"/>`, index)
	}
	wide := simpleContentProjectionSchema
	for _, anchor := range []string{
		`<xs:extension base="xs:decimal"><xs:attribute name="tag" type="xs:string" use="required"/></xs:extension>`,
		`<xs:restriction base="Amount"><xs:minInclusive value="0"/><xs:maxInclusive value="10"/><xs:attribute name="tag" type="xs:string" use="required"/></xs:restriction>`,
	} {
		if strings.Count(wide, anchor) != 1 {
			tb.Fatal("simple-content fixture anchor is not unique")
		}
		end := strings.LastIndex(anchor, "</")
		wide = strings.Replace(wide, anchor, anchor[:end]+attributes.String()+anchor[end:], 1)
	}
	const inline = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Mixed" mixed="true"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:simpleContent>
        <xs:restriction base="Mixed">
          <xs:simpleType>
            <xs:restriction base="xs:string"/>
          </xs:simpleType>
        </xs:restriction>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	const valid = `<r xmlns:q="urn:p"><amount tag="x">1.5</amount><limited tag="x">2</limited><q>q:Thing</q></r>`
	fixtures := []canonicalChildSourceFixture{
		{name: "FacetsWide", schema: wide, accepted: []string{valid}},
		{name: "NoFacetsInline", schema: inline, accepted: []string{`<root>value</root>`, `<root/>`}},
	}
	fixtures[0].rejected = append(fixtures[0].rejected,
		canonicalChildSourceOpponent{strings.Replace(valid, `>2</limited>`, `>11</limited>`, 1), xsderrors.CodeValidationFacet},
		canonicalChildSourceOpponent{strings.Replace(valid, ` tag="x"`, ``, 1), xsderrors.CodeValidationAttribute},
		canonicalChildSourceOpponent{strings.Replace(valid, `q:Thing`, `q:Other`, 1), xsderrors.CodeValidationElement},
	)
	fixtures[1].rejected = append(fixtures[1].rejected,
		canonicalChildSourceOpponent{`<other>value</other>`, xsderrors.CodeValidationRoot},
		canonicalChildSourceOpponent{`<root x="1">value</root>`, xsderrors.CodeValidationAttribute},
	)
	for _, fixture := range fixtures {
		if len(fixture.schema) > 96<<10 {
			tb.Fatal("canonical child source fixture exceeds byte bound")
		}
	}
	return fixtures
}

func checkCanonicalChildSourceFixture(tb testing.TB, engine *xsd.Engine, fixture canonicalChildSourceFixture) {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	for _, document := range fixture.accepted {
		if err := session.Validate(strings.NewReader(document)); err != nil {
			tb.Fatalf("%s accepted document: %v", fixture.name, err)
		}
	}
	for _, opponent := range fixture.rejected {
		err := session.Validate(strings.NewReader(opponent.document))
		diagnostics := xsderrors.Flatten(err)
		if len(diagnostics) == 0 {
			tb.Fatalf("%s opponent has no diagnostic: %v", fixture.name, err)
		}
		for _, diagnostic := range diagnostics {
			item, ok := errors.AsType[*xsderrors.Error](diagnostic)
			if !ok || item.Category() != xsderrors.CategoryValidation || item.Code() != opponent.code {
				tb.Fatalf("%s opponent diagnostic = %v, want validation/%s", fixture.name, diagnostic, opponent.code)
			}
		}
		for _, document := range fixture.accepted {
			if err := session.Validate(strings.NewReader(document)); err != nil {
				tb.Fatalf("%s same-session recovery: %v", fixture.name, err)
			}
		}
	}
}

func TestCompileCanonicalChildSourceFixtures(t *testing.T) {
	for _, fixture := range canonicalChildSourceFixtures(t) {
		t.Run(fixture.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("canonical-children.xsd", []byte(fixture.schema)))
			if err != nil || engine == nil {
				t.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			checkCanonicalChildSourceFixture(t, engine, fixture)
		})
	}
}

func BenchmarkCompileCanonicalChildSources(b *testing.B) {
	for _, fixture := range canonicalChildSourceFixtures(b) {
		b.Run(fixture.name, func(b *testing.B) {
			source := xsd.Bytes("canonical-children.xsd", []byte(fixture.schema))
			engine, err := xsd.Compile(source)
			if err != nil || engine == nil {
				b.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			checkCanonicalChildSourceFixture(b, engine, fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(source)
				if err != nil || engine == nil {
					b.Fatalf("Compile: engine=%v error=%v", engine, err)
				}
			}
			checkCanonicalChildSourceFixture(b, engine, fixture)
		})
	}
}
