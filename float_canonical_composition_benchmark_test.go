package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const floatCompositionSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Enumeration"><xs:restriction base="FLOAT_KIND"><xs:enumeration value="1"/><xs:enumeration value="1.5"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Bounded"><xs:restriction base="FLOAT_KIND"><xs:minInclusive value="0"/><xs:maxExclusive value="10"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Earlier"><xs:restriction base="xs:decimal"><xs:minInclusive value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="FloatList"><xs:list itemType="FLOAT_KIND"/></xs:simpleType>
  <xs:simpleType name="FloatUnion"><xs:union memberTypes="Earlier FLOAT_KIND"/></xs:simpleType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="enum" type="Enumeration"/>
    <xs:element name="bound" type="Bounded"/>
    <xs:element name="fixed" type="FLOAT_KIND" fixed="1"/>
    <xs:element name="list" type="FloatList"/>
    <xs:element name="union" type="FloatUnion"/>
    <xs:element name="row"><xs:complexType><xs:attribute name="value" type="FLOAT_KIND" use="required"/></xs:complexType></xs:element>
    <xs:element name="ref"><xs:complexType><xs:attribute name="value" type="FLOAT_KIND" use="required"/></xs:complexType></xs:element>
  </xs:sequence></xs:complexType>
    <xs:key name="ids"><xs:selector xpath="row"/><xs:field xpath="@value"/></xs:key>
    <xs:keyref name="refs" refer="ids"><xs:selector xpath="ref"/><xs:field xpath="@value"/></xs:keyref>
  </xs:element>
</xs:schema>`

const floatCompositionDocument = `<root><enum>1.5</enum><bound>2</bound><fixed>1.0</fixed><list>1 1.5</list><union>1.5</union><row value="1.00"/><ref value="1E0"/></root>`

type floatCompositionOpponent struct {
	document string
	code     xsderrors.Code
}

type floatCompositionFixture struct {
	name, schema string
	rejected     []floatCompositionOpponent
}

func floatCompositionFixtures(tb testing.TB) [2]floatCompositionFixture {
	tb.Helper()
	var fixtures [2]floatCompositionFixture
	for i, entry := range []struct{ name, kind string }{{"Float32", "xs:float"}, {"Float64", "xs:double"}} {
		fixture := floatCompositionFixture{name: entry.name, schema: strings.ReplaceAll(floatCompositionSchema, "FLOAT_KIND", entry.kind)}
		if len(fixture.schema) > 8<<10 || len(floatCompositionDocument) > 1<<10 {
			tb.Fatal("floating fixture exceeds source/document bounds")
		}
		for _, opponent := range []struct {
			old, new string
			code     xsderrors.Code
		}{
			{"<enum>1.5</enum>", "<enum>2</enum>", xsderrors.CodeValidationFacet},
			{"<bound>2</bound>", "<bound>10</bound>", xsderrors.CodeValidationFacet},
			{"<fixed>1.0</fixed>", "<fixed>2</fixed>", xsderrors.CodeValidationElement},
			{"<list>1 1.5</list>", "<list>1 INX</list>", xsderrors.CodeValidationFacet},
			{"<union>1.5</union>", "<union>INX</union>", xsderrors.CodeValidationFacet},
			{`<ref value="1E0"/>`, `<ref value="2"/>`, xsderrors.CodeValidationIdentity},
		} {
			if strings.Count(floatCompositionDocument, opponent.old) != 1 {
				tb.Fatal("opponent anchor is not unique")
			}
			fixture.rejected = append(fixture.rejected, floatCompositionOpponent{strings.Replace(floatCompositionDocument, opponent.old, opponent.new, 1), opponent.code})
		}
		fixtures[i] = fixture
	}
	return fixtures
}

func checkFloatCompositionRejection(tb testing.TB, err error, code xsderrors.Code) {
	tb.Helper()
	diagnostics := xsderrors.Flatten(err)
	if len(diagnostics) == 0 {
		tb.Fatal("invalid floating composition accepted")
	}
	for _, diagnostic := range diagnostics {
		item, ok := errors.AsType[*xsderrors.Error](diagnostic)
		if !ok || item.Category() != xsderrors.CategoryValidation || item.Code() != code {
			tb.Fatalf("floating opponent error=%v; provisional expected validation/%s", diagnostic, code)
		}
	}
}

func checkFloatCompositionFixture(tb testing.TB, session *xsd.Session, fixture floatCompositionFixture) {
	tb.Helper()
	if err := session.Validate(strings.NewReader(floatCompositionDocument)); err != nil {
		tb.Fatalf("valid %s composition: %v", fixture.name, err)
	}
	for _, opponent := range fixture.rejected {
		err := session.Validate(strings.NewReader(opponent.document))
		checkFloatCompositionRejection(tb, err, opponent.code)
		if err := session.Validate(strings.NewReader(floatCompositionDocument)); err != nil {
			tb.Fatalf("same-session recovery: %v", err)
		}
	}
}

func newFloatCompositionSession(tb testing.TB, engine *xsd.Engine) *xsd.Session {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	return session
}

func TestFloatCanonicalCompositionFixtures(t *testing.T) {
	t.Parallel()
	for _, fixture := range floatCompositionFixtures(t) {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.Compile(xsd.Bytes("float-composition.xsd", []byte(fixture.schema)))
			if err != nil || engine == nil {
				t.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			checkFloatCompositionFixture(t, newFloatCompositionSession(t, engine), fixture)
		})
	}
}

func BenchmarkFloatCanonicalComposition(b *testing.B) {
	for _, fixture := range floatCompositionFixtures(b) {
		b.Run("Compile"+fixture.name+"EnumBounds", func(b *testing.B) {
			source := xsd.Bytes("float-composition.xsd", []byte(fixture.schema))
			engine, err := xsd.Compile(source)
			if err != nil || engine == nil {
				b.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			checkFloatCompositionFixture(b, newFloatCompositionSession(b, engine), fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(source)
				if err != nil || engine == nil {
					b.Fatalf("Compile: engine=%v error=%v", engine, err)
				}
			}
			checkFloatCompositionFixture(b, newFloatCompositionSession(b, engine), fixture)
		})
		b.Run("Session"+fixture.name+"Composition", func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("float-composition.xsd", []byte(fixture.schema)))
			if err != nil || engine == nil {
				b.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			session := newFloatCompositionSession(b, engine)
			checkFloatCompositionFixture(b, session, fixture)
			reader := strings.NewReader(floatCompositionDocument)
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(floatCompositionDocument)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
			checkFloatCompositionFixture(b, session, fixture)
		})
	}
}
