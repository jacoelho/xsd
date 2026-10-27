package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type elementStartFixture struct {
	name, schema, document string
	rejected               []elementStartOpponent
}
type elementStartOpponent struct {
	document string
	scenario string
}

func BenchmarkSessionIntrinsicXSIDemand(b *testing.B) {
	for _, shape := range []string{"none", "dormant", "ancestor", "idref", "skipped"} {
		b.Run(shape, func(b *testing.B) {
			particle := `<xs:element name="item" type="T" nillable="true" minOccurs="0" maxOccurs="unbounded"/>`
			item := `<item xsi:type="T" xsi:nil="true" xsi:noNamespaceSchemaLocation="fixture.xsd"/>`
			constraint, rootAttrs := "", ` xsi:nil="false"`
			switch shape {
			case "dormant":
				constraint = `<xs:unique name="u"><xs:selector xpath="absent"/><xs:field xpath="@xsi:nil"/></xs:unique>`
			case "ancestor":
				constraint = `<xs:unique name="u"><xs:selector xpath="."/><xs:field xpath="@xsi:nil"/></xs:unique>`
			case "idref":
				rootAttrs += ` id="target" ref="target"`
			case "skipped":
				particle = `<xs:any namespace="urn:skip" processContents="skip" minOccurs="0" maxOccurs="unbounded"/>`
				item = `<s:item xsi:type="T" xsi:nil="true" xsi:noNamespaceSchemaLocation="fixture.xsd"/>`
			}
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><xs:simpleType name="T"><xs:restriction base="xs:string"/></xs:simpleType><xs:element name="root" nillable="true"><xs:complexType><xs:sequence>` + particle + `</xs:sequence><xs:attribute name="id" type="xs:ID"/><xs:attribute name="ref" type="xs:IDREF"/></xs:complexType>` + constraint + `</xs:element></xs:schema>`
			doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:s="urn:skip"` + rootAttrs + `>` + strings.Repeat(item, 1000) + `</root>`
			benchmarkSessionDocument(b, schema, doc)
		})
	}
}

func elementStartUniqueFixture(tb testing.TB) elementStartFixture {
	tb.Helper()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="child" type="xs:string" minOccurs="0" maxOccurs="unbounded"/></xs:sequence></xs:complexType><xs:unique name="u0"><xs:selector xpath="child"/><xs:field xpath="."/></xs:unique><xs:unique name="u1"><xs:selector xpath="child"/><xs:field xpath="."/></xs:unique></xs:element></xs:schema>`
	var doc strings.Builder
	doc.WriteString("<r>")
	for i := range 1000 {
		fmt.Fprintf(&doc, "<child>v%d</child>", i)
	}
	doc.WriteString("</r>")
	text := doc.String()
	return elementStartFixture{name: "unique-simple", schema: schema, document: text, rejected: []elementStartOpponent{{elementStartMutation(tb, text, "<child>v1</child>", "<child>v0</child>"), "duplicate simple text under two ancestor unique constraints"}}}
}

func elementStartAllFixture(tb testing.TB) elementStartFixture {
	tb.Helper()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="row" minOccurs="0" maxOccurs="unbounded"><xs:complexType><xs:all><xs:element name="a" type="xs:string"/><xs:element name="b" type="xs:string"/><xs:element name="c" type="xs:string"/></xs:all></xs:complexType></xs:element></xs:sequence></xs:complexType></xs:element></xs:schema>`
	orders := [3][3]string{{"a", "b", "c"}, {"c", "a", "b"}, {"b", "c", "a"}}
	var doc strings.Builder
	doc.WriteString("<r>")
	for i := range 1000 {
		doc.WriteString("<row>")
		for _, name := range orders[i%len(orders)] {
			fmt.Fprintf(&doc, "<%s>v%d</%s>", name, i, name)
		}
		doc.WriteString("</row>")
	}
	doc.WriteString("</r>")
	text := doc.String()
	return elementStartFixture{name: "required-all", schema: schema, document: text, rejected: []elementStartOpponent{
		{elementStartMutation(tb, text, "<c>v0</c>", ""), "missing required all child"},
		{elementStartMutation(tb, text, "</row>", "<a>duplicate</a></row>"), "duplicate all child"},
	}}
}

func elementStartMutation(tb testing.TB, original, old, replacement string) string {
	tb.Helper()
	if !strings.Contains(original, old) {
		tb.Fatalf("semantic mutation anchor absent: %q", old)
	}
	changed := strings.Replace(original, old, replacement, 1)
	if changed == original {
		tb.Fatal("semantic mutation did not change document")
	}
	return changed
}

func checkElementStartFixture(tb testing.TB, f elementStartFixture) (*xsd.Engine, *xsd.Session) {
	tb.Helper()
	engine, err := xsd.Compile(xsd.Bytes(f.name+".xsd", []byte(f.schema)))
	if err != nil {
		tb.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	if err := session.Validate(strings.NewReader(f.document)); err != nil {
		tb.Fatal(err)
	}
	for _, bad := range f.rejected {
		err := session.Validate(strings.NewReader(bad.document))
		if bad.scenario == "" || bad.document == f.document || !xsderrors.IsConclusiveValidation(err) {
			tb.Fatalf("%s semantic control (%s) = %v, want conclusive rejection", f.name, bad.scenario, err)
		}
		tb.Logf("semantic control %s/%s observed: %v", f.name, bad.scenario, err)
		if err := session.Validate(strings.NewReader(f.document)); err != nil {
			tb.Fatalf("reuse after %s: %v", f.name, err)
		}
	}
	return engine, session
}

func BenchmarkSessionElementStartUniqueSimple(b *testing.B) {
	fixture := elementStartUniqueFixture(b)
	_, session := checkElementStartFixture(b, fixture)
	reader := strings.NewReader(fixture.document)
	if err := session.Validate(reader); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(fixture.document)))
	b.ReportAllocs()
	for b.Loop() {
		reader.Reset(fixture.document)
		if err := session.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionElementStartAll(b *testing.B) {
	fixture := elementStartAllFixture(b)
	_, session := checkElementStartFixture(b, fixture)
	reader := strings.NewReader(fixture.document)
	if err := session.Validate(reader); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(fixture.document)))
	b.ReportAllocs()
	for b.Loop() {
		reader.Reset(fixture.document)
		if err := session.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}

// One operation is the complete rejected-document/valid-document reuse pair.
func BenchmarkSessionElementStartRecoveryPair(b *testing.B) {
	fixture := elementStartAllFixture(b)
	engine, _ := checkElementStartFixture(b, fixture)
	session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: 1})
	if err != nil {
		b.Fatal(err)
	}
	bad := elementStartMutation(b, fixture.document, "<row>", `<row unexpected="x">`)
	reader := strings.NewReader(fixture.document)
	if err := session.Validate(reader); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(bad) + len(fixture.document)))
	b.ReportAllocs()
	for b.Loop() {
		reader.Reset(bad)
		if err := session.Validate(reader); !xsderrors.IsConclusiveValidation(err) {
			b.Fatalf("recovery pair semantic reject: %v", err)
		}
		reader.Reset(fixture.document)
		if err := session.Validate(reader); err != nil {
			b.Fatalf("recovery pair valid reuse: %v", err)
		}
	}
}
