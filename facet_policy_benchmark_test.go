package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type facetPolicyFixture struct {
	name     string
	schema   string
	accepted []string
	rejected []string
}

func facetPolicyFixtures() []facetPolicyFixture {
	const header = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`
	const root = `<xs:element name="e" type="V"/></xs:schema>`
	return []facetPolicyFixture{
		{
			name: "Decimal",
			schema: header + `<xs:simpleType name="V"><xs:restriction base="xs:decimal">
<xs:minInclusive value="-10"/><xs:maxInclusive value="10"/>
<xs:totalDigits value="3"/><xs:fractionDigits value="1"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>2.5</e>`},
			rejected: []string{`<e>12.5</e>`, `<e>1.25</e>`},
		},
		{
			name: "String",
			schema: header + `<xs:simpleType name="V"><xs:restriction base="xs:string">
<xs:length value="3"/><xs:pattern value="[a-z]+"/>
<xs:enumeration value="abc"/><xs:whiteSpace value="collapse"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>abc</e>`, `<e>  abc  </e>`},
			rejected: []string{`<e>abcd</e>`, `<e>xyz</e>`},
		},
		{
			name: "List",
			schema: header + `<xs:simpleType name="Items"><xs:list itemType="xs:int"/></xs:simpleType>
<xs:simpleType name="V"><xs:restriction base="Items">
<xs:length value="2"/><xs:pattern value=".*"/><xs:enumeration value="1 2"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>1 2</e>`},
			rejected: []string{`<e>1 3</e>`, `<e>1</e>`},
		},
		{
			name: "Union",
			schema: header + `<xs:simpleType name="Mix"><xs:union memberTypes="xs:int xs:boolean"/></xs:simpleType>
<xs:simpleType name="V"><xs:restriction base="Mix">
<xs:pattern value="[12]|true"/><xs:enumeration value="1"/><xs:enumeration value="true"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>1</e>`, `<e>true</e>`},
			rejected: []string{`<e>2</e>`, `<e>3</e>`},
		},
	}
}

func checkFacetPolicyFixture(tb testing.TB, engine *xsd.Engine, fixture facetPolicyFixture) {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	for _, document := range fixture.accepted {
		if validationErr := session.Validate(strings.NewReader(document)); validationErr != nil {
			tb.Fatalf("%s accepted %q: %v", fixture.name, document, validationErr)
		}
	}
	for _, document := range fixture.rejected {
		validationErr := session.Validate(strings.NewReader(document))
		if validationErr == nil || !xsderrors.IsConclusiveValidation(validationErr) {
			tb.Fatalf("%s rejected %q: %v", fixture.name, document, validationErr)
		}
		diagnostics := xsderrors.Flatten(validationErr)
		if len(diagnostics) != 1 {
			tb.Fatalf("%s rejected %q diagnostic count = %d, want 1: %v", fixture.name, document, len(diagnostics), validationErr)
		}
		diagnostic, ok := errors.AsType[*xsderrors.Error](diagnostics[0])
		if !ok || diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() != xsderrors.CodeValidationFacet {
			tb.Fatalf("%s rejected %q diagnostic = %v, want validation facet", fixture.name, document, validationErr)
		}
		for _, accepted := range fixture.accepted {
			if recoveryErr := session.Validate(strings.NewReader(accepted)); recoveryErr != nil {
				tb.Fatalf("%s same-session recovery after %q: %v", fixture.name, document, recoveryErr)
			}
		}
	}
}

func BenchmarkCompileFacetPolicy(b *testing.B) {
	for _, fixture := range facetPolicyFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			source := xsd.Bytes("facet-policy.xsd", []byte(fixture.schema))
			engine, err := xsd.Compile(source)
			if err != nil || engine == nil {
				b.Fatalf("%s Compile: engine=%v err=%v", fixture.name, engine, err)
			}
			checkFacetPolicyFixture(b, engine, fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(source)
				if err != nil || engine == nil {
					b.Fatalf("%s Compile: engine=%v err=%v", fixture.name, engine, err)
				}
			}
			checkFacetPolicyFixture(b, engine, fixture)
		})
	}
}
