package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="v" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

func TestFormatXMLData(t *testing.T) {
	resp := formatXMLData(`<root><v>1</v></root>`)
	if resp.Error != "" {
		t.Fatalf("formatXMLData() error = %s", resp.Error)
	}
	if resp.XML != "<root>\n  <v>1</v>\n</root>" {
		t.Fatalf("formatted XML = %q", resp.XML)
	}
}

func TestValidateXMLDataValidAndInvalid(t *testing.T) {
	valid := validateXMLData("<root>\n  <v>1</v>\n</root>", testSchema)
	if !valid.Valid || len(valid.Errors) != 0 || valid.Error != "" {
		t.Fatalf("valid response = %+v", valid)
	}

	invalid := validateXMLData("<root>\n  <v>x</v>\n</root>", testSchema)
	if invalid.Valid {
		t.Fatal("invalid document validated")
	}
	if len(invalid.Errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1: %+v", len(invalid.Errors), invalid)
	}
	if invalid.Errors[0].Line != 2 {
		t.Fatalf("error line = %d, want 2", invalid.Errors[0].Line)
	}
	if invalid.Errors[0].Code != "validation.facet" {
		t.Fatalf("error code = %q, want validation.facet", invalid.Errors[0].Code)
	}
	if invalid.Errors[0].Source != "xml" {
		t.Fatalf("error source = %q, want xml", invalid.Errors[0].Source)
	}
}

func TestValidateXMLDataRejectsOversizeXML(t *testing.T) {
	resp := validateXMLData(string(make([]byte, int(maxXMLBytes)+1)), testSchema)
	if resp.Error == "" {
		t.Fatal("validateXMLData() accepted oversize XML")
	}
}

func TestValidateXMLDataRejectsOversizeXSD(t *testing.T) {
	resp := validateXMLData(`<root/>`, string(make([]byte, int(maxXSDBytes)+1)))
	if resp.Error == "" {
		t.Fatal("validateXMLData() accepted oversize XSD")
	}
}

func TestValidateXMLDataReportsWhitespaceOnlySchemaAsSchemaError(t *testing.T) {
	resp := validateXMLData(`<root/>`, " \t\r\n")
	if resp.Valid {
		t.Fatal("validateXMLData() accepted whitespace-only schema")
	}
	if resp.Error != "" {
		t.Fatalf("error = %q, want schema error list", resp.Error)
	}
	if len(resp.Errors) == 0 {
		t.Fatalf("len(errors) = 0, want schema error: %+v", resp)
	}
	if resp.Errors[0].Source != "xsd" {
		t.Fatalf("error source = %q, want xsd", resp.Errors[0].Source)
	}
}

func TestValidateXMLDataMarksSchemaErrors(t *testing.T) {
	resp := validateXMLData(`<root/>`, `<!DOCTYPE xs:schema><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	if resp.Valid {
		t.Fatal("validateXMLData() accepted invalid schema")
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1: %+v", len(resp.Errors), resp)
	}
	if resp.Errors[0].Source != "xsd" {
		t.Fatalf("error source = %q, want xsd", resp.Errors[0].Source)
	}
	if resp.Errors[0].Code != "unsupported.dtd" {
		t.Fatalf("error code = %q, want unsupported.dtd", resp.Errors[0].Code)
	}
}

func TestValidateXMLDataUsesFormattedWhitespaceText(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="v">
          <xs:simpleType>
            <xs:restriction base="xs:string">
              <xs:minLength value="1"/>
            </xs:restriction>
          </xs:simpleType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	formatted := formatXMLData(`<root><v> </v></root>`)
	if formatted.Error != "" {
		t.Fatalf("formatXMLData() error = %s", formatted.Error)
	}
	resp := validateXMLData(formatted.XML, schema)
	if !resp.Valid {
		t.Fatalf("validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
	}
}

func TestValidateXMLDataUsesCommentOnlySimpleContent(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="v">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:length value="0"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`

	formatted := formatXMLData(`<v><!--c--></v>`)
	if formatted.Error != "" {
		t.Fatalf("formatXMLData() error = %s", formatted.Error)
	}
	if formatted.XML != `<v><!--c--></v>` {
		t.Fatalf("formatted XML = %q", formatted.XML)
	}
	resp := validateXMLData(formatted.XML, schema)
	if !resp.Valid {
		t.Fatalf("validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
	}
}

func TestFormatXMLDataCapsFormattedOutput(t *testing.T) {
	const depth = 1200
	var input strings.Builder
	for range depth {
		input.WriteString("<a>")
	}
	for range depth {
		input.WriteString("</a>")
	}

	resp := formatXMLData(input.String())
	if resp.Error == "" {
		t.Fatal("formatXMLData() accepted oversized formatted output")
	}
	if !strings.Contains(resp.Error, "formatted XML exceeds") {
		t.Fatalf("formatXMLData() error = %q", resp.Error)
	}
}

const booksSchema = `<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema"
            targetNamespace="urn:books"
            xmlns:bks="urn:books">
  <xsd:element name="books" type="bks:BooksForm"/>
  <xsd:complexType name="BooksForm">
    <xsd:sequence>
      <xsd:element name="book" type="bks:BookForm" minOccurs="0" maxOccurs="unbounded"/>
    </xsd:sequence>
  </xsd:complexType>
  <xsd:complexType name="BookForm">
    <xsd:sequence>
      <xsd:element name="author" type="xsd:string"/>
      <xsd:element name="title" type="xsd:string"/>
      <xsd:element name="genre" type="xsd:string"/>
      <xsd:element name="price" type="xsd:float"/>
      <xsd:element name="pub_date" type="xsd:date"/>
      <xsd:element name="review" type="xsd:string"/>
    </xsd:sequence>
    <xsd:attribute name="id" type="xsd:string"/>
  </xsd:complexType>
</xsd:schema>`

func TestValidateXMLDataAcceptsUnqualifiedLocalElements(t *testing.T) {
	const xml = `<?xml version="1.0"?>
<x:books xmlns:x="urn:books">
  <book id="bk001">
    <author>Writer</author>
    <title>The First Book</title>
    <genre>Fiction</genre>
    <price>44.95</price>
    <pub_date>2000-10-01</pub_date>
    <review>An amazing story of nothing.</review>
  </book>
  <book id="bk002">
    <author>Poet</author>
    <title>The Poet's First Poem</title>
    <genre>Poem</genre>
    <price>24.95</price>
    <pub_date>2001-10-01</pub_date>
    <review>Least poetic poems.</review>
  </book>
</x:books>`

	formatted := formatXMLData(xml)
	if formatted.Error != "" {
		t.Fatalf("formatXMLData() error = %s", formatted.Error)
	}
	resp := validateXMLData(formatted.XML, booksSchema)
	if !resp.Valid {
		t.Fatalf("validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
	}
}

func TestValidateXMLDataRejectsBookMissingPubDate(t *testing.T) {
	const xml = `<?xml version="1.0"?>
<x:books xmlns:x="urn:books">
  <book id="bk002">
    <author>Poet</author>
    <title>The Poet's First Poem</title>
    <genre>Poem</genre>
    <price>24.95</price>
    <review>Least poetic poems.</review>
  </book>
</x:books>`

	formatted := formatXMLData(xml)
	if formatted.Error != "" {
		t.Fatalf("formatXMLData() error = %s", formatted.Error)
	}
	resp := validateXMLData(formatted.XML, booksSchema)
	if resp.Valid {
		t.Fatal("validateXMLData() accepted missing pub_date")
	}
	if len(resp.Errors) == 0 || resp.Errors[0].Code != "validation.element" {
		t.Fatalf("validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
	}
}

func TestWASMTargetBuilds(t *testing.T) {
	wasm := filepath.Join(t.TempDir(), "xsd.wasm")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-ldflags=-s -w", "-o", wasm, ".")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build js/wasm failed: %v\n%s", err, out)
	}
}

func TestHostTargetBuilds(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "wasmxsd")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build host failed: %v\n%s", err, out)
	}
}
