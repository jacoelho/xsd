package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type identityProgramFixture struct {
	name     string
	schema   string
	valid    []string
	invalid  []string
	measured string
}

func mixedIdentityProgramSchema(count int) string {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:identity" targetNamespace="urn:identity" elementFormDefault="unqualified" attributeFormDefault="unqualified"><xs:attribute name="slot" type="xs:string"/><xs:element name="rows"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded">`)
	for index := range count {
		fmt.Fprintf(&schema, `<xs:element name="e_%d" type="xs:string"/><xs:element name="a_%d"><xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType></xs:element><xs:element name="w_%d"><xs:complexType><xs:attribute ref="p:slot"/><xs:attribute name="id" type="xs:string"/></xs:complexType></xs:element>`, index, index, index)
	}
	schema.WriteString(`</xs:choice></xs:complexType></xs:element></xs:sequence></xs:complexType><xs:key name="tuple"><xs:selector xpath="row"/>`)
	var alternatives []string
	for _, branch := range []string{"e_%d", "a_%d/@id", "w_%d/@*", "w_%d/@p:*"} {
		for index := range count {
			alternatives = append(alternatives, fmt.Sprintf(branch, index))
		}
	}
	xpath := strings.Join(alternatives, " | ")
	for range count {
		fmt.Fprintf(&schema, `<xs:field xpath="%s"/>`, xpath)
	}
	schema.WriteString(`</xs:key></xs:element></xs:schema>`)
	return schema.String()
}

func mixedIdentityProgramFixture(count int) identityProgramFixture {
	fixture := identityProgramFixture{name: fmt.Sprintf("Mixed%d", count), schema: mixedIdentityProgramSchema(count)}
	for _, index := range []int{0, count - 1} {
		fixture.valid = append(fixture.valid,
			fmt.Sprintf(`<p:rows xmlns:p="urn:identity"><row><e_%d>v0</e_%d></row><row><e_%d>v1</e_%d></row></p:rows>`, index, index, index, index),
			fmt.Sprintf(`<p:rows xmlns:p="urn:identity"><row><a_%d id="v0"/></row><row><a_%d id="v1"/></row></p:rows>`, index, index),
			fmt.Sprintf(`<p:rows xmlns:p="urn:identity"><row><w_%d id="v0"/></row><row><w_%d id="v1"/></row></p:rows>`, index, index),
			fmt.Sprintf(`<p:rows xmlns:p="urn:identity"><row><w_%d p:slot="v0"/></row><row><w_%d p:slot="v1"/></row></p:rows>`, index, index),
		)
	}
	fixture.measured = fixture.valid[7]
	fixture.invalid = []string{
		`<p:rows xmlns:p="urn:identity"><row><e_0>v0</e_0></row><row><e_0>v0</e_0></row></p:rows>`,
		`<p:rows xmlns:p="urn:identity"><row/></p:rows>`,
		fmt.Sprintf(`<p:rows xmlns:p="urn:identity"><row><e_0>v0</e_0><e_%d>v1</e_%d></row></p:rows>`, count-1, count-1),
	}
	return fixture
}

func fanoutIdentityProgramFixture() identityProgramFixture {
	//nolint:dupword // Repeated field expressions exercise tuple identity across alternative attributes.
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="rows"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType><xs:attribute name="a" type="xs:string"/><xs:attribute name="b" type="xs:string"/><xs:attribute name="c" type="xs:string"/><xs:attribute name="d" type="xs:string"/><xs:attribute name="e" type="xs:string"/></xs:complexType></xs:element></xs:sequence></xs:complexType><xs:key name="tuple"><xs:selector xpath="row"/><xs:field xpath="@a|@b|@c|@d|@e"/><xs:field xpath="@a|@b|@c|@d|@e"/><xs:field xpath="@a|@b|@c|@d|@e"/><xs:field xpath="@a|@b|@c|@d|@e"/><xs:field xpath="@a|@b|@c|@d|@e"/></xs:key></xs:element></xs:schema>`
	const measured = `<rows><row a="v0"/><row b="v1"/><row c="v2"/><row d="v3"/><row e="v4"/></rows>`
	return identityProgramFixture{
		name: "Fanout5x5", schema: schema, measured: measured,
		valid: []string{
			`<rows><row a="v0"/></rows>`, `<rows><row b="v1"/></rows>`,
			`<rows><row c="v2"/></rows>`, `<rows><row d="v3"/></rows>`,
			`<rows><row e="v4"/></rows>`, measured,
		},
		invalid: []string{
			`<rows><row a="same"/><row e="same"/></rows>`,
			`<rows><row/></rows>`, `<rows><row a="v0" b="v1"/></rows>`,
		},
	}
}

func identityProgramFixtures() []identityProgramFixture {
	exact := identityFieldsBenchmarkDoc(1, 100)
	return []identityProgramFixture{
		{name: "NoIdentity", schema: benchmarkSchema, valid: []string{benchmarkDoc()}, measured: benchmarkDoc()},
		{name: "Exact1", schema: identityFieldsBenchmarkSchema(1), valid: []string{exact}, measured: exact,
			invalid: []string{`<rows><row k0="same"/><row k0="same"/></rows>`}},
		mixedIdentityProgramFixture(3), mixedIdentityProgramFixture(9), fanoutIdentityProgramFixture(),
	}
}

func checkIdentityProgramSession(tb testing.TB, session *xsd.Session, fixture identityProgramFixture) {
	tb.Helper()
	for index, document := range fixture.valid {
		if err := session.Validate(strings.NewReader(document)); err != nil {
			tb.Fatalf("%s valid document %d: %v", fixture.name, index, err)
		}
	}
	for index, document := range fixture.invalid {
		err := session.Validate(strings.NewReader(document))
		diagnostics := xsderrors.Flatten(err)
		if len(diagnostics) == 0 {
			tb.Fatalf("%s invalid document %d lacked an identity diagnostic: %v", fixture.name, index, err)
		}
		for _, diagnostic := range diagnostics {
			structured, ok := errors.AsType[*xsderrors.Error](diagnostic)
			if !ok || structured.Category() != xsderrors.CategoryValidation || structured.Code() != xsderrors.CodeValidationIdentity {
				tb.Fatalf("%s invalid document %d returned a nonidentity diagnostic: %v", fixture.name, index, diagnostic)
			}
		}
		if err := session.Validate(strings.NewReader(fixture.measured)); err != nil {
			tb.Fatalf("%s recovery after invalid document %d: %v", fixture.name, index, err)
		}
	}
}

func checkIdentityProgramEngine(tb testing.TB, engine *xsd.Engine, fixture identityProgramFixture) {
	tb.Helper()
	if engine == nil {
		tb.Fatalf("%s compilation returned a nil engine", fixture.name)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	checkIdentityProgramSession(tb, session, fixture)
}

func TestIdentityProgramFixtures(t *testing.T) {
	for _, fixture := range identityProgramFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("identity-programs.xsd", []byte(fixture.schema)))
			if err != nil {
				t.Fatal(err)
			}
			checkIdentityProgramEngine(t, engine, fixture)
		})
	}
}

func BenchmarkCompileIdentityPrograms(b *testing.B) {
	for _, fixture := range identityProgramFixtures()[1:] {
		b.Run(fixture.name, func(b *testing.B) {
			source := xsd.Bytes("identity-programs.xsd", []byte(fixture.schema))
			engine, err := xsd.Compile(source)
			if err != nil {
				b.Fatal(err)
			}
			checkIdentityProgramEngine(b, engine, fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(source)
				if err != nil || engine == nil {
					b.Fatalf("%s compile: engine nil %t, error %v", fixture.name, engine == nil, err)
				}
			}
			checkIdentityProgramEngine(b, engine, fixture)
		})
	}
}

func BenchmarkSessionIdentityPrograms(b *testing.B) {
	for _, fixture := range []identityProgramFixture{mixedIdentityProgramFixture(9), fanoutIdentityProgramFixture()} {
		b.Run(fixture.name, func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("identity-programs.xsd", []byte(fixture.schema)))
			if err != nil || engine == nil {
				b.Fatalf("compile: engine nil %t, error %v", engine == nil, err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			checkIdentityProgramSession(b, session, fixture)
			reader := strings.NewReader(fixture.measured)
			if err := session.Validate(reader); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(fixture.measured)))
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(fixture.measured)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
			checkIdentityProgramSession(b, session, fixture)
		})
	}
}
