package main

import (
	"encoding/json"
	"errors"
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
	if resp.Status != statusOK || resp.Error != "" {
		t.Fatalf("formatXMLData() error = %s", resp.Error)
	}
	if resp.XML != "<root>\n  <v>1</v>\n</root>" {
		t.Fatalf("formatted XML = %q", resp.XML)
	}
}

func TestValidateXMLDataValidAndInvalid(t *testing.T) {
	valid := (&validationAdapter{}).validateXMLData("<root>\n  <v>1</v>\n</root>", testSchema)
	if valid.Status != statusValid || len(valid.Errors) != 0 || valid.Error != "" {
		t.Fatalf("valid response = %+v", valid)
	}

	invalid := (&validationAdapter{}).validateXMLData("<root>\n  <v>x</v>\n</root>", testSchema)
	if invalid.Status != statusInvalid {
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

func TestResponsesMarshalAsDiscriminatedStates(t *testing.T) {
	tests := []struct {
		name string
		got  any
		want string
	}{
		{name: "format success", got: formatResponse{Status: statusOK, XML: "<root/>"}, want: `{"status":"ok","xml":"\u003croot/\u003e"}`},
		{name: "format error", got: formatFailure("bad XML", 2, 3), want: `{"status":"error","error":"bad XML","line":2,"column":3}`},
		{name: "valid", got: validateResponse{Status: statusValid}, want: `{"status":"valid"}`},
		{
			name: "invalid",
			got:  validationInvalid([]errorOutput{{Message: "bad value"}}),
			want: `{"status":"invalid","errors":[{"message":"bad value"}]}`,
		},
		{
			name: "error with diagnostics",
			got:  validationError(errors.New("assessment unavailable"), []errorOutput{{Source: "xsd", Message: "bad schema"}}),
			want: `{"status":"error","error":"assessment unavailable","errors":[{"source":"xsd","message":"bad schema"}]}`,
		},
		{name: "error", got: validationFailure("failed"), want: `{"status":"error","error":"failed"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.got)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(data) != tt.want {
				t.Fatalf("Marshal() = %s, want %s", data, tt.want)
			}
		})
	}
}

func TestValidationAdapterReusesAndEvictsOnlyItsSuccessfulEngine(t *testing.T) {
	adapter := &validationAdapter{}
	if response := adapter.validateXMLData(`<root><v>1</v></root>`, testSchema); response.Status != statusValid {
		t.Fatalf("first validation = %+v", response)
	}
	engine := adapter.engine
	if engine == nil {
		t.Fatal("first validation did not publish compiled engine")
	}
	if response := adapter.validateXMLData(`<root><v>x</v></root>`, testSchema); response.Status != statusInvalid {
		t.Fatalf("invalid instance = %+v", response)
	}
	if adapter.engine != engine {
		t.Fatal("instance failure evicted cached engine")
	}
	if response := adapter.validateXMLData(`<root><v>1</v></root>`, testSchema+"\n"); response.Status != statusValid {
		t.Fatalf("changed schema validation = %+v", response)
	}
	if adapter.engine == engine {
		t.Fatal("changed schema reused prior engine")
	}
	if response := adapter.validateXMLData(`<root/>`, `<!DOCTYPE xs:schema><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`); response.Status != statusError {
		t.Fatalf("failed compilation = %+v, want error", response)
	}
	if adapter.engine != nil || adapter.xsdText != "" {
		t.Fatal("failed compilation published a cache entry")
	}
}

func TestValidationAdapterAdmissionFailuresLeaveCacheUnchanged(t *testing.T) {
	adapter := &validationAdapter{}
	if response := adapter.validateXMLData(`<root><v>1</v></root>`, testSchema); response.Status != statusValid {
		t.Fatalf("warm validation = %+v", response)
	}
	engine := adapter.engine
	key := adapter.xsdText
	if response := adapter.validateXMLData(string(make([]byte, int(maxXMLBytes)+1)), testSchema); response.Status != statusError {
		t.Fatalf("oversized XML = %+v", response)
	}
	if adapter.engine != engine || adapter.xsdText != key {
		t.Fatal("oversized XML changed the cache")
	}
	if response := adapter.validateXMLData(`<root/>`, string(make([]byte, int(maxXSDBytes)+1))); response.Status != statusError {
		t.Fatalf("oversized XSD = %+v", response)
	}
	if adapter.engine != engine || adapter.xsdText != key {
		t.Fatal("oversized XSD changed the cache")
	}
}

func TestValidationAdapterFatalAssessmentFailuresRetainCache(t *testing.T) {
	adapter := &validationAdapter{}
	if response := adapter.validateXMLData(`<root><v>1</v></root>`, testSchema); response.Status != statusValid {
		t.Fatalf("warm validation = %+v", response)
	}
	engine := adapter.engine
	key := adapter.xsdText
	if engine == nil || key != testSchema {
		t.Fatalf("warm cache = (%p, %q), want compiled engine and exact schema key", engine, key)
	}

	for _, test := range []struct {
		name string
		xml  string
		code string
	}{
		{name: "malformed XML", xml: `<root><v>1</root>`, code: "validation.xml"},
		{name: "unsupported XML 1.1", xml: `<?xml version="1.1"?><root><v>1</v></root>`, code: "unsupported.xml_1_1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := adapter.validateXMLData(test.xml, testSchema)
			if response.Status != statusError || response.Error == "" {
				t.Fatalf("response = %+v, want error with summary", response)
			}
			if len(response.Errors) != 1 || response.Errors[0].Source != "xml" || response.Errors[0].Code != test.code {
				t.Fatalf("diagnostics = %+v, want one xml %s diagnostic", response.Errors, test.code)
			}
			if adapter.engine != engine || adapter.xsdText != key {
				t.Fatalf("fatal assessment changed cache: engine=%p key=%q, want engine=%p key=%q", adapter.engine, adapter.xsdText, engine, key)
			}
		})
	}

	response := adapter.validateXMLData(`<root><v>1</v></root>`, testSchema)
	if response.Status != statusValid || response.Error != "" || len(response.Errors) != 0 {
		t.Fatalf("validation after fatal assessments = %+v, want valid response", response)
	}
	if adapter.engine != engine || adapter.xsdText != key {
		t.Fatalf("successful recovery changed cache: engine=%p key=%q, want engine=%p key=%q", adapter.engine, adapter.xsdText, engine, key)
	}
}

func TestValidationAdapterClassifiesAssessmentFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		xml    string
		status responseStatus
		code   string
	}{
		{name: "undeclared root", xml: `<other/>`, status: statusInvalid, code: "validation.root"},
		{name: "no root", xml: "   ", status: statusError, code: "validation.xml"},
		{name: "text outside root", xml: `tail<root><v>1</v></root>`, status: statusError, code: "validation.xml"},
		{name: "unsupported XML version", xml: `<?xml version="1.1"?><root><v>1</v></root>`, status: statusError, code: "unsupported.xml_1_1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := (&validationAdapter{}).validateXMLData(test.xml, testSchema)
			if response.Status != test.status || response.Error == "" && test.status == statusError {
				t.Fatalf("response = %+v, want status %s", response, test.status)
			}
			if len(response.Errors) != 1 || response.Errors[0].Code != test.code {
				t.Fatalf("diagnostics = %+v, want %s", response.Errors, test.code)
			}
		})
	}
}

func TestValidationAdapterClassifiesConclusiveAssessmentCodes(t *testing.T) {
	for _, test := range []struct {
		name   string
		schema string
		xml    string
		code   string
	}{
		{
			name: "required attribute",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="id" type="xs:string" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xml:  `<root/>`,
			code: "validation.attribute",
		},
		{
			name: "incompatible xsi type",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`,
			xml:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:int">1</root>`,
			code: "validation.type",
		},
		{
			name: "nilled non-nillable element",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`,
			xml:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`,
			code: "validation.nil",
		},
		{
			name: "duplicate key value",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
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
</xs:schema>`,
			xml:  `<root><item id="a"/><item id="a"/></root>`,
			code: "validation.identity",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := (&validationAdapter{}).validateXMLData(test.xml, test.schema)
			if response.Status != statusInvalid || response.Error != "" {
				t.Fatalf("response = %+v, want invalid response without summary", response)
			}
			if len(response.Errors) != 1 || response.Errors[0].Source != "xml" || response.Errors[0].Code != test.code {
				t.Fatalf("diagnostics = %+v, want one xml %s diagnostic", response.Errors, test.code)
			}
		})
	}
}

func TestInvalidResponseRequiresDiagnostics(t *testing.T) {
	resp := validationInvalid(nil)
	if resp.Status != statusError || resp.Error == "" || len(resp.Errors) != 0 {
		t.Fatalf("validationInvalid(nil) = %+v, want infrastructure error", resp)
	}
}

func TestValidateXMLDataReportsMalformedXMLBeforeSchemaErrorsWithoutDiscardingEither(t *testing.T) {
	resp := (&validationAdapter{}).validateXMLData(`<root><v>1</root>`, `<!DOCTYPE xs:schema><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	if resp.Status != statusError {
		t.Fatal("validateXMLData() did not report an operational error")
	}
	if resp.Error == "" {
		t.Fatal("error summary is empty")
	}
	if len(resp.Errors) != 2 {
		t.Fatalf("len(errors) = %d, want 2: %+v", len(resp.Errors), resp)
	}
	if resp.Errors[0].Source != "xml" {
		t.Fatalf("error source = %q, want xml", resp.Errors[0].Source)
	}
	if resp.Errors[0].Code != "validation.xml" {
		t.Fatalf("error code = %q, want validation.xml", resp.Errors[0].Code)
	}
	if resp.Errors[1].Source != "xsd" || resp.Errors[1].Code != "unsupported.dtd" {
		t.Fatalf("second error = %+v, want xsd unsupported.dtd", resp.Errors[1])
	}
}

func TestValidateXMLDataReportsMalformedXMLAfterSchemaCompiles(t *testing.T) {
	resp := (&validationAdapter{}).validateXMLData(`<root><v>1</root>`, testSchema)
	if resp.Status != statusError {
		t.Fatal("validateXMLData() did not report an operational error")
	}
	if resp.Error == "" {
		t.Fatal("error summary is empty")
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1: %+v", len(resp.Errors), resp)
	}
	if resp.Errors[0].Source != "xml" {
		t.Fatalf("error source = %q, want xml", resp.Errors[0].Source)
	}
	if resp.Errors[0].Code != "validation.xml" {
		t.Fatalf("error code = %q, want validation.xml", resp.Errors[0].Code)
	}
}

func TestValidateXMLDataUsesParserFailurePosition(t *testing.T) {
	for _, test := range []struct {
		name      string
		xml       string
		line, col int
	}{
		{name: "end tag", xml: "<root>\n</root x>", line: 2, col: 8},
		{name: "buffered character data", xml: "<root>\n<v>1</v>\nabcdefgh\x01</root>", line: 3, col: 9},
	} {
		t.Run(test.name, func(t *testing.T) {
			resp := (&validationAdapter{}).validateXMLData(test.xml, testSchema)
			if resp.Status != statusError || len(resp.Errors) != 1 || resp.Error == "" {
				t.Fatalf("validateXMLData() = %+v, want one operational diagnostic", resp)
			}
			got := resp.Errors[0]
			if got.Code != "validation.xml" || got.Line != test.line || got.Column != test.col {
				t.Fatalf("diagnostic = %s at %d:%d, want validation.xml at %d:%d", got.Code, got.Line, got.Column, test.line, test.col)
			}
		})
	}
}

func TestValidateXMLDataReportsMalformedXMLBeyondValidationErrorLimit(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="v" type="xs:int" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	var input strings.Builder
	input.WriteString(`<root>`)
	for range maxValidationErrors {
		input.WriteString(`<v>x</v>`)
	}
	input.WriteString(`<v>1</root>`)

	resp := (&validationAdapter{}).validateXMLData(input.String(), schema)
	if resp.Status != statusError {
		t.Fatal("validateXMLData() did not report an operational error")
	}
	if resp.Error == "" {
		t.Fatal("error summary is empty")
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1: %+v", len(resp.Errors), resp)
	}
	if resp.Errors[0].Source != "xml" || resp.Errors[0].Code != "validation.xml" {
		t.Fatalf("error = %+v, want XML syntax error", resp.Errors[0])
	}
}

func TestValidateXMLDataAcceptsCompactXMLWithoutFormatting(t *testing.T) {
	const xml = `<root><v>1</v></root>`
	formatted := formatXMLData(xml)
	if formatted.Error != "" {
		t.Fatalf("formatXMLData() error = %s", formatted.Error)
	}
	if formatted.XML == xml {
		t.Fatalf("formatXMLData() did not change compact XML")
	}

	resp := (&validationAdapter{}).validateXMLData(xml, testSchema)
	if resp.Status != statusValid || resp.Error != "" || len(resp.Errors) != 0 {
		t.Fatalf("validationAdapter.validateXMLData() = %+v", resp)
	}
}

func TestValidateXMLDataRejectsOversizeXML(t *testing.T) {
	resp := (&validationAdapter{}).validateXMLData(string(make([]byte, int(maxXMLBytes)+1)), testSchema)
	if resp.Status != statusError || resp.Error == "" {
		t.Fatal("validationAdapter.validateXMLData() accepted oversize XML")
	}
}

func TestValidateXMLDataRejectsOversizeXSD(t *testing.T) {
	resp := (&validationAdapter{}).validateXMLData(`<root/>`, string(make([]byte, int(maxXSDBytes)+1)))
	if resp.Status != statusError || resp.Error == "" {
		t.Fatal("validationAdapter.validateXMLData() accepted oversize XSD")
	}
}

func TestValidateXMLDataRejectsOversizeXSDBeforeParsingXML(t *testing.T) {
	resp := (&validationAdapter{}).validateXMLData(`<root>`, string(make([]byte, int(maxXSDBytes)+1)))
	if !strings.Contains(resp.Error, "XSD exceeds") {
		t.Fatalf("validationAdapter.validateXMLData() = %+v, want XSD size error", resp)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("validationAdapter.validateXMLData() errors = %+v, want size error only", resp.Errors)
	}
}

func TestValidateXMLDataReportsWhitespaceOnlySchemaAsSchemaError(t *testing.T) {
	resp := (&validationAdapter{}).validateXMLData(`<root/>`, " \t\r\n")
	if resp.Status != statusError {
		t.Fatal("validateXMLData() did not report a schema error")
	}
	if resp.Error == "" {
		t.Fatal("error summary is empty")
	}
	if len(resp.Errors) == 0 {
		t.Fatalf("len(errors) = 0, want schema error: %+v", resp)
	}
	if resp.Errors[0].Source != "xsd" {
		t.Fatalf("error source = %q, want xsd", resp.Errors[0].Source)
	}
}

func TestValidateXMLDataMarksSchemaErrors(t *testing.T) {
	resp := (&validationAdapter{}).validateXMLData(`<root/>`, `<!DOCTYPE xs:schema><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	if resp.Status != statusError {
		t.Fatal("validateXMLData() did not report a schema error")
	}
	if resp.Error == "" {
		t.Fatal("error summary is empty")
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
	resp := (&validationAdapter{}).validateXMLData(formatted.XML, schema)
	if resp.Status != statusValid {
		t.Fatalf("validationAdapter.validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
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
	resp := (&validationAdapter{}).validateXMLData(formatted.XML, schema)
	if resp.Status != statusValid {
		t.Fatalf("validationAdapter.validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
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
	if resp.Status != statusError || resp.Error == "" {
		t.Fatal("formatXMLData() accepted oversized formatted output")
	}
	if !strings.Contains(resp.Error, "XML formatted output byte limit exceeded") {
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
	resp := (&validationAdapter{}).validateXMLData(formatted.XML, booksSchema)
	if resp.Status != statusValid {
		t.Fatalf("validationAdapter.validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
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
	resp := (&validationAdapter{}).validateXMLData(formatted.XML, booksSchema)
	if resp.Status != statusInvalid {
		t.Fatal("validationAdapter.validateXMLData() accepted missing pub_date")
	}
	if len(resp.Errors) == 0 || resp.Errors[0].Code != "validation.element" {
		t.Fatalf("validationAdapter.validateXMLData() = %+v, formatted XML = %q", resp, formatted.XML)
	}
}

func BenchmarkValidateXMLDataCold(b *testing.B) {
	b.ReportAllocs()
	inputs := [...]string{
		`<root><v>1</v></root>`,
		`<root><v>2</v></root>`,
	}
	schemas := [...]string{testSchema, testSchema + "\n"}
	b.ResetTimer()
	for i := range b.N {
		response := (&validationAdapter{}).validateXMLData(inputs[i%len(inputs)], schemas[i%len(schemas)])
		if response.Status != statusValid {
			b.Fatalf("cold validation = %+v", response)
		}
	}
}

func BenchmarkValidateXMLDataWarm(b *testing.B) {
	b.ReportAllocs()
	inputs := [...]string{
		`<root><v>1</v></root>`,
		`<root><v>2</v></root>`,
	}
	adapter := &validationAdapter{}
	if response := adapter.validateXMLData(inputs[0], testSchema); response.Status != statusValid {
		b.Fatalf("warmup validation = %+v", response)
	}
	b.ResetTimer()
	for i := range b.N {
		response := adapter.validateXMLData(inputs[i%len(inputs)], testSchema)
		if response.Status != statusValid {
			b.Fatalf("warm validation = %+v", response)
		}
	}
}

func BenchmarkValidateXMLDataChangedSchema(b *testing.B) {
	b.ReportAllocs()
	inputs := [...]string{
		`<root><v>1</v></root>`,
		`<root><v>2</v></root>`,
	}
	schemas := [...]string{testSchema, testSchema + "\n"}
	adapter := &validationAdapter{}
	if response := adapter.validateXMLData(inputs[0], schemas[1]); response.Status != statusValid {
		b.Fatalf("warmup validation = %+v", response)
	}
	b.ResetTimer()
	for i := range b.N {
		response := adapter.validateXMLData(inputs[i%len(inputs)], schemas[i%len(schemas)])
		if response.Status != statusValid {
			b.Fatalf("changed-schema validation = %+v", response)
		}
	}
}
