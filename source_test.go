package xsd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
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

func TestLoadedSchemaDocumentsSortBySourceName(t *testing.T) {
	limits, err := normalizeCompileOptions(CompileOptions{})
	if err != nil {
		t.Fatalf("normalizeCompileOptions() error = %v", err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatalf("newCompiler() error = %v", err)
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	if err := c.load([]SchemaSource{
		sourceBytes("file:///b.xsd", schema),
		sourceBytes("a.xsd", schema),
	}); err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if got, want := []string{c.docs[0].name, c.docs[1].name}, []string{"a.xsd", "file:///b.xsd"}; !slices.Equal(got, want) {
		t.Fatalf("doc names = %v, want %v", got, want)
	}
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

func TestChameleonIncludeAdoptedByMultipleNamespaces(t *testing.T) {
	engine, err := Compile(
		sourceBytes("schemas/a-main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:a"
           targetNamespace="urn:a"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="t:Shared"/>
</xs:schema>`)),
		sourceBytes("schemas/b-main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:b"
           targetNamespace="urn:b"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="t:Shared"/>
</xs:schema>`)),
		sourceBytes("schemas/common.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:complexType name="Shared">
    <xs:sequence>
      <xs:element name="v" type="Value"/>
    </xs:sequence>
  </xs:complexType>
  <xs:simpleType name="Value">
    <xs:restriction base="xs:int"/>
  </xs:simpleType>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:a"><v>7</v></root>`)
	mustValidate(t, engine, `<root xmlns="urn:b"><v>7</v></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:a"><v>x</v></root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root xmlns="urn:b"><v>x</v></root>`, ErrValidationFacet)
}

func TestChameleonIncludeCascadeAdoptedByMultipleNamespaces(t *testing.T) {
	mainSchema := func(ns string) []byte {
		return []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="` + ns + `"
           targetNamespace="` + ns + `"
           elementFormDefault="qualified">
  <xs:include schemaLocation="mid.xsd"/>
  <xs:element name="root" type="t:Mid"/>
</xs:schema>`)
	}
	engine, err := Compile(
		sourceBytes("schemas/a-main.xsd", mainSchema("urn:a")),
		sourceBytes("schemas/b-main.xsd", mainSchema("urn:b")),
		sourceBytes("schemas/mid.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:include schemaLocation="base.xsd"/>
  <xs:complexType name="Mid">
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
	mustValidate(t, engine, `<root xmlns="urn:a"><v>7</v></root>`)
	mustValidate(t, engine, `<root xmlns="urn:b"><v>7</v></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:b"><v>x</v></root>`, ErrValidationFacet)
}

func TestChameleonIncludeSameNamespaceTwiceCompiles(t *testing.T) {
	engine, err := Compile(
		sourceBytes("schemas/main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:a"
           targetNamespace="urn:a"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="t:Shared"/>
</xs:schema>`)),
		sourceBytes("schemas/extra.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:a"
           targetNamespace="urn:a"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="extra" type="t:Shared"/>
</xs:schema>`)),
		sourceBytes("schemas/common.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:complexType name="Shared">
    <xs:sequence>
      <xs:element name="v" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:a"><v>7</v></root>`)
	mustValidate(t, engine, `<extra xmlns="urn:a"><v>7</v></extra>`)
}

func TestChameleonIncludeIdentityConstraintAdoptedByMultipleNamespaces(t *testing.T) {
	mainSchema := func(ns string) []byte {
		return []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="` + ns + `">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`)
	}
	engine, err := Compile(
		sourceBytes("schemas/a-main.xsd", mainSchema("urn:a")),
		sourceBytes("schemas/b-main.xsd", mainSchema("urn:b")),
		sourceBytes("schemas/common.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="list">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<a:list xmlns:a="urn:a"><item id="1"/><item id="2"/></a:list>`)
	mustValidate(t, engine, `<b:list xmlns:b="urn:b"><item id="1"/><item id="2"/></b:list>`)
	mustNotValidate(t, engine, `<a:list xmlns:a="urn:a"><item id="1"/><item id="1"/></a:list>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<b:list xmlns:b="urn:b"><item id="1"/><item id="1"/></b:list>`, ErrValidationIdentity)
}

func TestChameleonIncludeMultipleNamespacesViaResolver(t *testing.T) {
	const commonSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:include schemaLocation="mem:base"/>
  <xs:complexType name="Shared">
    <xs:sequence>
      <xs:element name="v" type="Value"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`
	const baseSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Value">
    <xs:restriction base="xs:int"/>
  </xs:simpleType>
</xs:schema>`
	resolver := ResolverFunc(func(_, location string) (SchemaSource, error) {
		switch location {
		case "mem:common":
			return Reader("resolved/common.xsd", strings.NewReader(commonSchema)), nil
		case "mem:base":
			return Reader("resolved/base.xsd", strings.NewReader(baseSchema)), nil
		default:
			return SchemaSource{}, ErrSchemaNotFound
		}
	})
	mainSchema := func(ns string) string {
		return `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="` + ns + `"
           targetNamespace="` + ns + `"
           elementFormDefault="qualified">
  <xs:include schemaLocation="mem:common"/>
  <xs:element name="root" type="t:Shared"/>
</xs:schema>`
	}
	engine, err := Compile(
		Reader("a-main.xsd", strings.NewReader(mainSchema("urn:a"))).WithResolver(resolver),
		Reader("b-main.xsd", strings.NewReader(mainSchema("urn:b"))).WithResolver(resolver),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:a"><v>7</v></root>`)
	mustValidate(t, engine, `<root xmlns="urn:b"><v>7</v></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:a"><v>x</v></root>`, ErrValidationFacet)
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

func TestFileResolverMissingIncludeIsUnresolved(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, filepath.Join(dir, "main.xsd"), `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)
	engine, err := Compile(File(filepath.Join(dir, "main.xsd")))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root>7</root>`)
}

func TestResolvedMissingSourceReadNotFoundIsUnresolved(t *testing.T) {
	engine, err := Compile(Reader("main.xsd", strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)).WithResolver(ResolverFunc(func(string, string) (SchemaSource, error) {
		return SchemaSource{
			name: "missing.xsd",
			open: func() (io.ReadCloser, error) {
				return nil, os.ErrNotExist
			},
		}, nil
	})))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root>7</root>`)
}

func TestMissingTopLevelFileIsSchemaReadError(t *testing.T) {
	_, err := Compile(File(filepath.Join(t.TempDir(), "missing.xsd")))
	expectCategoryCode(t, err, SchemaParseErrorCategory, ErrSchemaRead)
	if errors.Is(err, ErrSchemaNotFound) {
		t.Fatalf("Compile() error wraps ErrSchemaNotFound")
	}
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

func TestFileURIIncludePropagatesChameleonTarget(t *testing.T) {
	dir := t.TempDir()
	types := filepath.Join(dir, "types.xsd")
	writeSchemaFile(t, types, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
  <xs:complexType name="Included">
    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	main := filepath.Join(dir, "main.xsd")
	writeSchemaFile(t, main, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:t="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="file://`+filepath.ToSlash(types)+`"/>
  <xs:element name="root" type="t:Included"/>
</xs:schema>`)
	engine, err := Compile(File(main))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
}

func TestFileURIImportNamespaceMismatchIsSchemaError(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other.xsd")
	writeSchemaFile(t, other, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`)
	main := filepath.Join(dir, "main.xsd")
	writeSchemaFile(t, main, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import namespace="urn:a" schemaLocation="file://`+filepath.ToSlash(other)+`"/>
</xs:schema>`)
	_, err := Compile(File(main))
	expectCode(t, err, ErrSchemaReference)
}

func TestURISchemaLocationResolvesProvidedSourceName(t *testing.T) {
	engine, err := Compile(
		sourceBytes("main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="urn:types"/>
  <xs:element name="root" type="t:Included"/>
</xs:schema>`)),
		sourceBytes("urn:types", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
  <xs:complexType name="Included">
    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
}

func TestRelativeSchemaLocationResolvesProvidedURLSourceName(t *testing.T) {
	engine, err := Compile(
		sourceBytes("https://example.test/schema/main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="t:Included"/>
</xs:schema>`)),
		sourceBytes("https://example.test/schema/types.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
  <xs:complexType name="Included">
    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
}

func TestFileURISchemaLocationResolvesProvidedFileSource(t *testing.T) {
	dir := t.TempDir()
	types := filepath.Join(dir, "types.xsd")
	writeSchemaFile(t, types, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
  <xs:complexType name="Included">
    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	engine, err := Compile(
		sourceBytes("https://example.test/schema/main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="file://`+filepath.ToSlash(types)+`"/>
  <xs:element name="root" type="t:Included"/>
</xs:schema>`)),
		File(types),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
}

func TestWhitespaceOnlyIncludeSchemaLocationIsMissing(t *testing.T) {
	_, err := Compile(sourceBytes("https://example.test/schema/main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="   "/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestWhitespaceOnlyIncludeSchemaLocationDoesNotCallResolver(t *testing.T) {
	called := false
	_, err := Compile(Reader("https://example.test/schema/main.xsd", strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="   "/>
</xs:schema>`)).WithResolver(ResolverFunc(func(base, location string) (SchemaSource, error) {
		called = true
		return SchemaSource{}, ErrSchemaNotFound
	})))
	expectCode(t, err, ErrSchemaReference)
	if called {
		t.Fatal("resolver called for whitespace-only include schemaLocation")
	}
}

func TestResolverSchemaLocationUsesCollapsedWhitespace(t *testing.T) {
	engine, err := Compile(Reader("main.xsd", strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="  types.xsd  "/>
  <xs:element name="root" type="Included"/>
</xs:schema>`)).WithResolver(mapResolver{
		"types.xsd": `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Included">
    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
  </xs:complexType>
</xs:schema>`,
	}))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root><v>7</v></root>`)
}

func TestFileLikeReaderNamesUseCleanedSourceKeys(t *testing.T) {
	engine, err := Compile(
		sourceBytes("./main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="Included"/>
</xs:schema>`)),
		sourceBytes("./types.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Included">
    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root><v>7</v></root>`)
}

func TestURISchemaLocationImportNamespaceMismatchIsSchemaError(t *testing.T) {
	_, err := Compile(
		sourceBytes("main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import namespace="urn:a" schemaLocation="urn:other"/>
</xs:schema>`)),
		sourceBytes("urn:other", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`)),
	)
	expectCode(t, err, ErrSchemaReference)
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

func TestHTTPResolverImportNamespaceMismatchIsSchemaError(t *testing.T) {
	_, err := Compile(Reader("main.xsd", strings.NewReader(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:import namespace="urn:a" schemaLocation="http://example.invalid/other.xsd"/>
	</xs:schema>`)).WithResolver(mapResolver{
		"http://example.invalid/other.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`,
	}))
	expectCode(t, err, ErrSchemaReference)
}

func TestHTTPResolverChameleonIncludePropagatesTargetNamespace(t *testing.T) {
	engine, err := Compile(Reader("main.xsd", strings.NewReader(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	           targetNamespace="urn:test"
	           xmlns:t="urn:test"
	           elementFormDefault="qualified">
	  <xs:include schemaLocation="http://example.invalid/types.xsd"/>
	  <xs:element name="root" type="t:Included"/>
	</xs:schema>`)).WithResolver(mapResolver{
		"http://example.invalid/types.xsd": `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
	  <xs:complexType name="Included">
	    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
	  </xs:complexType>
	</xs:schema>`,
	}))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
}

func TestResolverReceivesURIBaseUnchanged(t *testing.T) {
	const mainName = "https://example.test/schema/main.xsd"
	var gotBase string
	engine, err := Compile(Reader(mainName, strings.NewReader(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	           targetNamespace="urn:test"
	           xmlns:t="urn:test"
	           elementFormDefault="qualified">
	  <xs:include schemaLocation="types.xsd"/>
	  <xs:element name="root" type="t:Included"/>
	</xs:schema>`)).WithResolver(ResolverFunc(func(base, location string) (SchemaSource, error) {
		gotBase = base
		if location != "types.xsd" {
			return SchemaSource{}, ErrSchemaNotFound
		}
		return Reader("https://example.test/schema/types.xsd", strings.NewReader(`
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
		  <xs:complexType name="Included">
		    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
		  </xs:complexType>
		</xs:schema>`)), nil
	})))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if gotBase != mainName {
		t.Fatalf("resolver base = %q, want %q", gotBase, mainName)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
}

func TestResolverAliasIncludeNamespaceMismatchIsSchemaError(t *testing.T) {
	_, err := Compile(Reader("main.xsd", strings.NewReader(`
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a">
		  <xs:include schemaLocation="mem:types"/>
		</xs:schema>`)).WithResolver(ResolverFunc(func(_, location string) (SchemaSource, error) {
		if location != "mem:types" {
			return SchemaSource{}, ErrSchemaNotFound
		}
		return Reader("resolved/types.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`)), nil
	})))
	expectCode(t, err, ErrSchemaReference)
}

func TestResolverAliasImportNamespaceMismatchIsSchemaError(t *testing.T) {
	_, err := Compile(Reader("main.xsd", strings.NewReader(`
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		  <xs:import namespace="urn:a" schemaLocation="mem:types"/>
		</xs:schema>`)).WithResolver(ResolverFunc(func(_, location string) (SchemaSource, error) {
		if location != "mem:types" {
			return SchemaSource{}, ErrSchemaNotFound
		}
		return Reader("resolved/types.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`)), nil
	})))
	expectCode(t, err, ErrSchemaReference)
}

func TestResolverAliasChameleonIncludePropagatesTargetNamespace(t *testing.T) {
	engine, err := Compile(Reader("main.xsd", strings.NewReader(`
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
		           targetNamespace="urn:test"
		           xmlns:t="urn:test"
		           elementFormDefault="qualified">
		  <xs:include schemaLocation=" mem:types "/>
		  <xs:element name="root" type="t:Included"/>
		</xs:schema>`)).WithResolver(ResolverFunc(func(_, location string) (SchemaSource, error) {
		if location != "mem:types" {
			return SchemaSource{}, ErrSchemaNotFound
		}
		return Reader("resolved/types.xsd", strings.NewReader(`
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
		  <xs:complexType name="Included">
		    <xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence>
		  </xs:complexType>
		</xs:schema>`)), nil
	})))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test"><v>7</v></root>`)
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
