package xsd

import (
	"errors"
	"io"
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

func TestChameleonIncludeTargetNamespacePropagatesThroughNestedIncludes(t *testing.T) {
	engine, err := Compile(
		sourceBytes("schemas/z-main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="a-mid.xsd"/>
  <xs:element name="root" type="t:Included"/>
</xs:schema>`)),
		sourceBytes("schemas/a-mid.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:include schemaLocation="base.xsd"/>
  <xs:complexType name="Included">
    <xs:sequence>
      <xs:element name="v" type="Value"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)),
		sourceBytes("schemas/base.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Value">
    <xs:restriction base="xs:int"/>
  </xs:simpleType>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><v>x</v></root>`, ErrValidationFacet)
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

func TestCompileOptionsSchemaSourceByteLimitAppliesToFile(t *testing.T) {
	dir := t.TempDir()
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	path := filepath.Join(dir, "schema.xsd")
	writeSchemaFile(t, path, schema)

	_, err := CompileWithOptions(CompileOptions{MaxSchemaSourceBytes: int64(len(schema) - 1)}, File(path))
	expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)
}

func TestCompileOptionsSchemaSourceByteLimitAppliesToResolvedInclude(t *testing.T) {
	included := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>` + strings.Repeat("x", 128) + `</xs:documentation></xs:annotation></xs:schema>`
	main := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="types.xsd"/></xs:schema>`
	_, err := CompileWithOptions(
		CompileOptions{MaxSchemaSourceBytes: int64(len(included) - 1)},
		Reader("main.xsd", strings.NewReader(main)).WithResolver(mapResolver{
			"types.xsd": included,
		}),
	)
	expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)
}

func TestLimitedReaderRejectsOverLimit(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	if _, err := Compile(LimitedReader("schema.xsd", strings.NewReader(schema), int64(len(schema)))); err != nil {
		t.Fatalf("Compile() limited reader boundary error = %v", err)
	}

	_, err := Compile(LimitedReader("schema.xsd", strings.NewReader(schema), int64(len(schema)-1)))
	expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)
}

func TestLimitedReaderRejectsInvalidLimit(t *testing.T) {
	_, err := Compile(LimitedReader("schema.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`), 0))
	expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)
}

func TestResolveLocalSchemaLocationFileURIHost(t *testing.T) {
	if _, ok := resolveLocalSchemaLocation("/tmp/main.xsd", "file://example.com/tmp/types.xsd"); ok {
		t.Fatal("resolveLocalSchemaLocation() accepted non-local file URI host")
	}

	want := filepath.Clean(filepath.FromSlash("/tmp/types.xsd"))
	for _, location := range []string{"file:///tmp/types.xsd", "file://localhost/tmp/types.xsd"} {
		t.Run(location, func(t *testing.T) {
			got, ok := resolveLocalSchemaLocation("/tmp/main.xsd", location)
			if !ok {
				t.Fatalf("resolveLocalSchemaLocation() ok = false")
			}
			if got != want {
				t.Fatalf("resolveLocalSchemaLocation() = %q, want %q", got, want)
			}
		})
	}
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

type countedSchemaRead struct {
	data  string
	reads int
}

func (s *countedSchemaRead) open() (io.ReadCloser, error) {
	s.reads++
	return io.NopCloser(strings.NewReader(s.data)), nil
}

func TestCompileReadsResolvedSchemaSourcesOnce(t *testing.T) {
	main := &countedSchemaRead{data: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="Included"/>
</xs:schema>`}
	types := &countedSchemaRead{data: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Included"><xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence></xs:complexType>
</xs:schema>`}
	resolver := ResolverFunc(func(_, location string) (SchemaSource, error) {
		if location != "types.xsd" {
			return SchemaSource{}, ErrSchemaNotFound
		}
		return SchemaSource{name: "types.xsd", open: types.open}, nil
	})
	engine, err := Compile(SchemaSource{name: "main.xsd", open: main.open, resolver: resolver})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root><v>7</v></root>`)
	if main.reads != 1 {
		t.Fatalf("main reads = %d, want 1", main.reads)
	}
	if types.reads != 1 {
		t.Fatalf("types reads = %d, want 1", types.reads)
	}
}
