package xsd

import (
	"slices"
	"testing"
)

var xmlWhitespaceSink string

func TestXMLWhitespaceHelpers(t *testing.T) {
	if got := trimXMLWhitespace("\t\r a \n"); got != "a" {
		t.Fatalf("trimXMLWhitespace() = %q, want %q", got, "a")
	}
	if got := trimXMLWhitespace("\u00a0a\u00a0"); got != "\u00a0a\u00a0" {
		t.Fatalf("trimXMLWhitespace() = %q, want NBSPs preserved", got)
	}
	if got := string(trimXMLWhitespaceBytes([]byte("\t\r a \n"))); got != "a" {
		t.Fatalf("trimXMLWhitespaceBytes() = %q, want %q", got, "a")
	}
	byteTests := []struct {
		name string
		in   []byte
		want bool
	}{
		{name: "empty", in: nil, want: true},
		{name: "xml whitespace", in: []byte(" \t\r\n"), want: true},
		{name: "text", in: []byte(" a "), want: false},
		{name: "unicode space", in: []byte("\u00a0"), want: false},
	}
	for _, tt := range byteTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isXMLWhitespaceBytes(tt.in); got != tt.want {
				t.Fatalf("isXMLWhitespaceBytes() = %v, want %v", got, tt.want)
			}
		})
	}
	fields := slices.Collect(xmlFieldsSeq(" \ta\nb\rc "))
	if !slices.Equal(fields, []string{"a", "b", "c"}) {
		t.Fatalf("xmlFieldsSeq() = %#v, want %#v", fields, []string{"a", "b", "c"})
	}
	fields = slices.Collect(xmlFieldsSeq("a\u00a0b c"))
	if !slices.Equal(fields, []string{"a\u00a0b", "c"}) {
		t.Fatalf("xmlFieldsSeq() = %#v, want NBSP inside first field", fields)
	}
}

func BenchmarkXMLWhitespaceNormalizeReplaceASCII(b *testing.B) {
	in := "alpha\tbeta\ngamma\rdelta alpha\tbeta\ngamma\rdelta"
	for b.Loop() {
		xmlWhitespaceSink = normalizeWhitespace(in, whitespaceReplace)
	}
}

func BenchmarkXMLWhitespaceNormalizeCollapseASCII(b *testing.B) {
	in := " \talpha  beta\ngamma\r delta \t alpha  beta\ngamma\r delta "
	for b.Loop() {
		xmlWhitespaceSink = normalizeWhitespace(in, whitespaceCollapse)
	}
}

func BenchmarkXMLWhitespaceNormalizeReplaceNoop(b *testing.B) {
	in := "alpha beta gamma delta"
	for b.Loop() {
		xmlWhitespaceSink = normalizeWhitespace(in, whitespaceReplace)
	}
}

func BenchmarkXMLWhitespaceNormalizeCollapseNoop(b *testing.B) {
	in := "alpha beta gamma delta"
	for b.Loop() {
		xmlWhitespaceSink = normalizeWhitespace(in, whitespaceCollapse)
	}
}

func BenchmarkXMLWhitespaceNormalizeReplaceUnicode(b *testing.B) {
	in := "alpha\tβeta\ngamma\rδelta alpha\tβeta\ngamma\rδelta"
	for b.Loop() {
		xmlWhitespaceSink = normalizeWhitespace(in, whitespaceReplace)
	}
}

func BenchmarkXMLWhitespaceAttributeUnicode(b *testing.B) {
	in := "alpha\tβeta\ngamma\rδelta alpha\tβeta\ngamma\rδelta"
	for b.Loop() {
		xmlWhitespaceSink = replaceXMLWhitespace(in)
	}
}

func TestSchemaLexicalWhitespaceIsXMLWhitespace(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" abstract="&#x9;true&#xA;"/>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() XML whitespace boolean error = %v", err)
	}

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" abstract="&#xA0;true&#xA0;"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	engine, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="&#x9;xs:int&#xA;"/>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() XML whitespace QName error = %v", err)
	}
	mustValidate(t, engine, `<root>7</root>`)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="&#xA0;xs:int&#xA0;"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestOccurrenceWhitespaceIsXMLWhitespace(t *testing.T) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element name="v" type="xs:int" maxOccurs="&#x9;unbounded&#xA;"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() XML whitespace maxOccurs error = %v", err)
	}
	mustValidate(t, engine, `<root><v>1</v><v>2</v></root>`)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element name="v" type="xs:int" maxOccurs="&#xA0;unbounded&#xA0;"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaOccurrence)
}

func TestIdentityXPathWhitespaceIsXMLWhitespace(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k"><xs:selector xpath="&#xA0;row&#xA0;"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestSchemaLocationWhitespaceIsXMLWhitespace(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	mustValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:t&#x9;hinted.xsd"/>`)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:t&#xA0;hinted.xsd"/>`, ErrValidationAttribute)
}

func TestNotationTextWhitespaceIsXMLWhitespace(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="n" public="p">&#xA0;</xs:notation>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestXMLDeclarationWhitespaceIsXMLWhitespace(t *testing.T) {
	if err := validateXMLDeclContent([]byte("version=\"1.0\"\t")); err != nil {
		t.Fatalf("validateXMLDeclContent() XML whitespace error = %v", err)
	}
	if err := validateXMLDeclContent([]byte("version=\"1.0\"\u00a0")); err == nil {
		t.Fatal("validateXMLDeclContent() error = nil for NBSP")
	}
}
