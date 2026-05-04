package xsd

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type mapResolver map[string]string

func (r mapResolver) ResolveSchema(_ string, location string) (SchemaSource, error) {
	data, ok := r[location]
	if !ok {
		return SchemaSource{}, ErrSchemaNotFound
	}
	return Reader(location, strings.NewReader(data)), nil
}

func TestReaderWithResolverResolvesNestedIncludes(t *testing.T) {
	resolver := mapResolver{
		"types.xsd": `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="base.xsd"/>
  <xs:complexType name="Included">
    <xs:sequence>
      <xs:element name="v" type="Value"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`,
		"base.xsd": `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Value">
    <xs:restriction base="xs:int"/>
  </xs:simpleType>
</xs:schema>`,
	}
	engine, err := Compile(Reader("main.xsd", strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="Included"/>
</xs:schema>`)).WithResolver(resolver))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root><v>7</v></root>`)
	mustNotValidate(t, engine, `<root><v>x</v></root>`, ErrValidationFacet)
}

func TestResolverNotFoundPreservesUnresolvedSchemaLocation(t *testing.T) {
	engine, err := Compile(Reader("main.xsd", strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)).WithResolver(ResolverFunc(func(string, string) (SchemaSource, error) {
		return SchemaSource{}, ErrSchemaNotFound
	})))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root>7</root>`)
}

func TestResolverErrorReturnsSchemaRead(t *testing.T) {
	_, err := Compile(Reader("main.xsd", strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="broken.xsd"/>
</xs:schema>`)).WithResolver(ResolverFunc(func(string, string) (SchemaSource, error) {
		return SchemaSource{}, errors.New("resolver failed")
	})))
	expectCode(t, err, ErrSchemaRead)
}

func TestExplicitIncludeResolvesProvidedSource(t *testing.T) {
	engine, err := Compile(
		sourceBytes("schemas/main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="tns:Included"/>
</xs:schema>`)),
		sourceBytes("schemas/types.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:complexType name="Included">
    <xs:sequence>
      <xs:element name="v" type="Code"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>X</v></root>`)
}

func TestFileResolvesLocalIncludeAndImport(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, filepath.Join(dir, "main.xsd"), `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           xmlns:o="urn:other"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="types.xsd"/>
  <xs:import namespace="urn:other" schemaLocation="other.xsd"/>
  <xs:element name="root" type="tns:Included"/>
  <xs:element name="other" type="o:Other"/>
</xs:schema>`)
	writeSchemaFile(t, filepath.Join(dir, "types.xsd"), `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:complexType name="Included">
    <xs:sequence>
      <xs:element name="v" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)
	writeSchemaFile(t, filepath.Join(dir, "other.xsd"), `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other">
  <xs:simpleType name="Other">
    <xs:restriction base="xs:string">
      <xs:enumeration value="ok"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	engine, err := Compile(File(filepath.Join(dir, "main.xsd")))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
	mustValidate(t, engine, `<other xmlns="urn:test">ok</other>`)
	mustNotValidate(t, engine, `<other xmlns="urn:test">bad</other>`, ErrValidationFacet)
}

func TestReaderDoesNotResolveSchemaLocationFromName(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, filepath.Join(dir, "types.xsd"), `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)
	engine, err := Compile(sourceBytes(filepath.Join(dir, "main.xsd"), []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	err = engine.Validate(strings.NewReader(`<root>7</root>`))
	expectCode(t, err, ErrValidationRoot)
}

func TestIncludeAndImportNamespaceMismatchAreSchemaErrors(t *testing.T) {
	_, err := Compile(
		sourceBytes("main.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:include schemaLocation="other.xsd"/></xs:schema>`)),
		sourceBytes("other.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`)),
	)
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(
		sourceBytes("main.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="urn:a" schemaLocation="other.xsd"/></xs:schema>`)),
		sourceBytes("other.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`)),
	)
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(
		sourceBytes("main.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import schemaLocation="other.xsd"/></xs:schema>`)),
		sourceBytes("other.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)),
	)
	expectCode(t, err, ErrSchemaReference)
}

func TestSchemaLocationHintsCanBeUnresolved(t *testing.T) {
	_, err := Compile(sourceBytes("main.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="http://example.invalid/missing.xsd"/><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	_, err = Compile(sourceBytes("main.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include/></xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("main.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:sequence/></xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}
