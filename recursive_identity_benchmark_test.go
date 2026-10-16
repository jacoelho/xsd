package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const recursiveIdentityBenchmarkSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element ref="root" minOccurs="0" maxOccurs="unbounded"/></xs:sequence>
      <xs:attribute name="id" type="xs:string" use="required"/>
      <xs:attribute name="ref" type="xs:string"/>
    </xs:complexType>
    <xs:key name="ids"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>
    <xs:keyref name="refs" refer="ids"><xs:selector xpath="."/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>`

func checkRecursiveIdentityBenchmark(tb testing.TB, engine *xsd.Engine) {
	tb.Helper()
	if engine == nil {
		tb.Fatal("Compile returned nil engine")
	}
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	const valid = `<root id="x" ref="x"><root id="x"/><root id="x"/></root>`
	const invalid = `<root id="x" ref="missing"><root id="x"/><root id="x"/></root>`
	if validationErr := session.Validate(strings.NewReader(valid)); validationErr != nil {
		tb.Fatal(validationErr)
	}
	rejection := session.Validate(strings.NewReader(invalid))
	failures := xsderrors.Flatten(rejection)
	if rejection == nil || len(failures) == 0 {
		tb.Fatal("unresolved recursive keyref accepted")
	}
	for _, failure := range failures {
		diagnostic, ok := errors.AsType[*xsderrors.Error](failure)
		if !ok || diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() != xsderrors.CodeValidationIdentity || !strings.Contains(diagnostic.Message(), "keyref does not resolve") {
			tb.Fatalf("recursive keyref diagnostic = %v", failure)
		}
	}
	if recoveryErr := session.Validate(strings.NewReader(valid)); recoveryErr != nil {
		tb.Fatalf("same-session recovery: %v", recoveryErr)
	}
}

func BenchmarkCompileRecursiveIdentity(b *testing.B) {
	source := xsd.Bytes("recursive-identity.xsd", []byte(recursiveIdentityBenchmarkSchema))
	engine, err := xsd.Compile(source)
	if err != nil {
		b.Fatal(err)
	}
	checkRecursiveIdentityBenchmark(b, engine)
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(source)
		if err != nil || engine == nil {
			b.Fatalf("Compile returned engine=%v error=%v", engine, err)
		}
	}
	checkRecursiveIdentityBenchmark(b, engine)
}
