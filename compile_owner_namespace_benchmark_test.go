package xsd_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const compileOwnerNamespaceSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:r="urn:root" xmlns:a="urn:a" targetNamespace="urn:root" elementFormDefault="qualified">
<xs:import namespace="urn:a"/>
<xs:element name="r"><xs:complexType><xs:sequence><xs:any namespace="urn:a urn:b ##local" processContents="lax" minOccurs="0" maxOccurs="unbounded"/></xs:sequence><xs:anyAttribute namespace="urn:attr ##targetNamespace" processContents="lax"/></xs:complexType>
<xs:unique name="ids"><xs:selector xpath="a:item"/><xs:field xpath="@a:*"/></xs:unique></xs:element>
</xs:schema>`

const compileOwnerNamespaceItemSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a" elementFormDefault="qualified" attributeFormDefault="qualified"><xs:element name="item"><xs:complexType><xs:attribute name="id" type="xs:string" use="required"/></xs:complexType></xs:element></xs:schema>`

func checkCompileOwnerNamespace(tb testing.TB, engine *xsd.Engine) {
	tb.Helper()
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	valid := []byte(`<r xmlns="urn:root" xmlns:a="urn:a" xmlns:q="urn:attr" q:code="x"><a:item a:id="1"/></r>`)
	reader := bytes.NewReader(valid)
	if err := session.Validate(reader); err != nil {
		tb.Fatalf("valid namespaces: %v", err)
	}
	reader.Reset(bytes.Replace(valid, []byte(`xmlns:a="urn:a"`), []byte(`xmlns:a="urn:outside"`), 1))
	if err := session.Validate(reader); err == nil {
		tb.Fatal("child outside admitted wildcard namespaces accepted")
	}
	reader.Reset(valid)
	if err := session.Validate(reader); err != nil {
		tb.Fatalf("same-session recovery: %v", err)
	}
	unbound := strings.Replace(compileOwnerNamespaceSchema, `xpath="@a:*"`, `xpath="@missing:*"`, 1)
	failed, err := xsd.Compile(xsd.Bytes("unbound-identity.xsd", []byte(unbound)), xsd.Bytes("item.xsd", []byte(compileOwnerNamespaceItemSchema)))
	if failed != nil || err == nil {
		tb.Fatalf("unbound identity wildcard: engine=%v err=%v", failed, err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != "unbound QName prefix missing" {
		tb.Fatalf("unbound identity wildcard diagnostic: %v", err)
	}
}

func TestCompileOwnerNamespaceSemantics(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("namespace.xsd", []byte(compileOwnerNamespaceSchema)), xsd.Bytes("item.xsd", []byte(compileOwnerNamespaceItemSchema)))
	if err != nil {
		t.Fatal(err)
	}
	checkCompileOwnerNamespace(t, engine)
}

func BenchmarkCompileOwnerNamespaceMetadata(b *testing.B) {
	source := []byte(compileOwnerNamespaceSchema)
	itemSource := []byte(compileOwnerNamespaceItemSchema)
	engine, err := xsd.Compile(xsd.Bytes("namespace.xsd", source), xsd.Bytes("item.xsd", itemSource))
	if err != nil {
		b.Fatal(err)
	}
	checkCompileOwnerNamespace(b, engine)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("namespace.xsd", source), xsd.Bytes("item.xsd", itemSource)); err != nil {
			b.Fatal(err)
		}
	}
}
