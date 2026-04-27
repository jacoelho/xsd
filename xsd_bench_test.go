package xsd_test

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
)

func BenchmarkValidateSimpleNoIdentity(b *testing.B) {
	schema := compileBenchSchema(b, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	benchValidate(b, schema, `<root xmlns="urn:bench">ok</root>`)
}

func BenchmarkValidateAttrHeavy(b *testing.B) {
	schemaXML, docXML := attrHeavyBenchFixture(32)
	schema := compileBenchSchema(b, schemaXML)
	benchValidate(b, schema, docXML)
}

func BenchmarkValidateNamespaceHeavy(b *testing.B) {
	schemaXML, docXML := namespaceHeavyBenchFixture(24)
	schema := compileBenchSchema(b, schemaXML)
	benchValidate(b, schema, docXML)
}

func BenchmarkValidateIdentityConstraints(b *testing.B) {
	schema := compileBenchSchema(b, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
            <xs:attribute name="code" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="itemIDs">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:unique>
  </xs:element>
</xs:schema>`)
	var doc strings.Builder
	doc.WriteString(`<root xmlns="urn:bench">`)
	for i := range 64 {
		doc.WriteString(`<item id="i`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteString(`" code="x"/>`)
	}
	doc.WriteString(`</root>`)
	benchValidate(b, schema, doc.String())
}

func BenchmarkValidateUnionListFacet(b *testing.B) {
	schema := compileBenchSchema(b, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:simpleType name="NumberList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="Value">
    <xs:union memberTypes="tns:NumberList xs:string"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:Value"/>
</xs:schema>`)
	benchValidate(b, schema, `<root xmlns="urn:bench">1 2 3 4 5 6 7 8 9 10</root>`)
}

func BenchmarkValidateScalarWhitespace(b *testing.B) {
	schema := compileBenchSchema(b, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:simpleType name="CollapsedString">
    <xs:restriction base="xs:string">
      <xs:whiteSpace value="collapse"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:CollapsedString"/>
</xs:schema>`)
	benchValidate(b, schema, `<root xmlns="urn:bench">  alpha   beta
 gamma  </root>`)
}

func BenchmarkCompileSmallSchema(b *testing.B) {
	schemaXML := []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)

	b.ReportAllocs()
	for b.Loop() {
		fsys := fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: schemaXML},
		}
		if _, err := xsd.CompileFS(fsys, "schema.xsd", xsd.CompileConfig{}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestValidatorValidateConcurrent(t *testing.T) {
	schema := compileTestSchema(t, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:int" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	validator, err := schema.NewValidator(xsd.ValidateConfig{})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}
	doc := []byte(`<root xmlns="urn:bench"><item>1</item><item>2</item><item>3</item></root>`)

	const workers = 8
	const iterations = 50
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			reader := bytes.NewReader(nil)
			for range iterations {
				reader.Reset(doc)
				if err := validator.Validate(reader); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("Validate() concurrent error = %v", err)
	}
}

func TestValidationErrorsDeterministicOrder(t *testing.T) {
	schema := compileTestSchema(t, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:int"/>
        <xs:element name="b" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	doc := []byte(`<root xmlns="urn:bench"><a>bad</a><b>worse</b></root>`)

	var first string
	for i := range 20 {
		err := schema.Validate(bytes.NewReader(doc))
		if err == nil {
			t.Fatal("Validate() err = nil, want validation errors")
		}
		got := err.Error()
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("validation error order changed on run %d:\nfirst: %s\n got: %s", i, first, got)
		}
	}
}

func compileBenchSchema(b *testing.B, schemaXML string) *xsd.Schema {
	b.Helper()
	schema := compileSchema(b, schemaXML)
	return schema
}

func compileTestSchema(t *testing.T, schemaXML string) *xsd.Schema {
	t.Helper()
	return compileSchema(t, schemaXML)
}

func compileSchema(tb testing.TB, schemaXML string) *xsd.Schema {
	tb.Helper()
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}
	schema, err := xsd.CompileFS(fsys, "schema.xsd", xsd.CompileConfig{})
	if err != nil {
		tb.Fatalf("Compile() error = %v", err)
	}
	return schema
}

func benchValidate(b *testing.B, schema *xsd.Schema, docXML string) {
	b.Helper()
	docBytes := []byte(docXML)
	reader := bytes.NewReader(nil)

	b.ReportAllocs()
	b.SetBytes(int64(len(docBytes)))
	for b.Loop() {
		reader.Reset(docBytes)
		if err := schema.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func attrHeavyBenchFixture(count int) (string, string) {
	var schema strings.Builder
	schema.WriteString(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>`)
	for i := range count {
		schema.WriteString(`<xs:attribute name="a`)
		schema.WriteString(strconv.Itoa(i))
		schema.WriteString(`" type="xs:string"/>`)
	}
	schema.WriteString(`</xs:complexType>
  </xs:element>
</xs:schema>`)

	var doc strings.Builder
	doc.WriteString(`<root xmlns="urn:bench"`)
	for i := range count {
		doc.WriteByte(' ')
		doc.WriteString("a")
		doc.WriteString(strconv.Itoa(i))
		doc.WriteString(`="v"`)
	}
	doc.WriteString(`/>`)
	return schema.String(), doc.String()
}

func namespaceHeavyBenchFixture(count int) (string, string) {
	var schema strings.Builder
	schema.WriteString(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>`)
	for i := range count {
		schema.WriteString(`<xs:element name="e`)
		schema.WriteString(strconv.Itoa(i))
		schema.WriteString(`" type="xs:string"/>`)
	}
	schema.WriteString(`</xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	var doc strings.Builder
	doc.WriteString(`<root xmlns="urn:bench"`)
	for i := range count {
		doc.WriteString(` xmlns:p`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteString(`="urn:extra:`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteByte('"')
	}
	doc.WriteByte('>')
	for i := range count {
		doc.WriteString(`<e`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteString(`>v</e`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteByte('>')
	}
	doc.WriteString(`</root>`)
	return schema.String(), doc.String()
}
