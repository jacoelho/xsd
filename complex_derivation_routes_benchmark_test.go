package xsd_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func complexDerivationRoutesSource(memberSimpleType string) []byte {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="C0"/><xs:complexType name="S0"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>`)
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&schema, `<xs:complexType name="C%d"><xs:complexContent><xs:extension base="C%d"/></xs:complexContent></xs:complexType><xs:complexType name="S%d"><xs:simpleContent><xs:extension base="S%d"/></xs:simpleContent></xs:complexType>`, i, i-1, i, i-1)
	}
	fmt.Fprintf(&schema, `<xs:element name="headC" type="C0" abstract="true"/><xs:element name="memberC" type="C8" substitutionGroup="headC"/><xs:element name="headS" type="xs:string" abstract="true"/><xs:element name="memberS" type="%s" substitutionGroup="headS"/><xs:element name="headA" type="xs:anyType" abstract="true"/><xs:element name="memberA" type="S8" substitutionGroup="headA"/><xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="headC"/><xs:element ref="headS"/><xs:element ref="headA"/></xs:sequence></xs:complexType></xs:element></xs:schema>`, memberSimpleType)
	return []byte(schema.String())
}

func checkComplexDerivationRoutes(tb testing.TB, engine *xsd.Engine) {
	tb.Helper()
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	valid := []byte(`<root><memberC/><memberS>x</memberS><memberA>y</memberA></root>`)
	reader := bytes.NewReader(valid)
	if err := session.Validate(reader); err != nil {
		tb.Fatalf("valid substitution routes: %v", err)
	}
	for _, invalid := range []string{
		`<root><memberS>x</memberS><memberC/><memberA>y</memberA></root>`,
		`<unknown/>`,
	} {
		reader.Reset([]byte(invalid))
		if err := session.Validate(reader); err == nil {
			tb.Fatalf("invalid substitution document accepted: %s", invalid)
		}
		reader.Reset(valid)
		if err := session.Validate(reader); err != nil {
			tb.Fatalf("same-session recovery: %v", err)
		}
	}
	failed, err := xsd.Compile(xsd.Bytes("incompatible-substitution.xsd", complexDerivationRoutesSource("C8")))
	if failed != nil || err == nil {
		tb.Fatalf("unrelated complex member accepted for string head: engine=%v err=%v", failed, err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != "substitution group member memberS type C8 is not derived from head headS type {http://www.w3.org/2001/XMLSchema}string" {
		tb.Fatalf("incompatible substitution diagnostic: %v", err)
	}
}

func TestCompileComplexDerivationRoutesSemantics(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("complex-derivation.xsd", complexDerivationRoutesSource("S8")))
	if err != nil {
		t.Fatal(err)
	}
	checkComplexDerivationRoutes(t, engine)
}

func BenchmarkCompileComplexDerivationRoutes(b *testing.B) {
	source := xsd.Bytes("complex-derivation.xsd", complexDerivationRoutesSource("S8"))
	engine, err := xsd.Compile(source)
	if err != nil {
		b.Fatal(err)
	}
	checkComplexDerivationRoutes(b, engine)
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(source)
		if err != nil {
			b.Fatal(err)
		}
	}
	checkComplexDerivationRoutes(b, engine)
}
