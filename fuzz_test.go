package xsd

import (
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzXMLStreamParser(f *testing.F) {
	for _, seed := range []string{
		`<root/>`,
		`<?xml version="1.0"?><root a="&amp;">text</root>`,
		`<root><![CDATA[x<y]]><!--c--><?pi v?></root>`,
		`<root><a/><b attr="value">text</b></root>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}
		names := newByteStringCache()
		values := newByteStringCache()
		parser := new(xmlStreamParser)
		parser.reset(strings.NewReader(input), &names, &values)
		for tokens := 0; ; tokens++ {
			if tokens > 4096 {
				t.Skip()
			}
			_, err := parser.next()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
		}
	})
}

func FuzzSchemaParserLimits(f *testing.F) {
	for _, seed := range []string{
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`,
		`<!DOCTYPE xs:schema><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence></xs:complexType></xs:element></xs:schema>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, schema string) {
		if len(schema) > 8192 {
			t.Skip()
		}
		if _, err := CompileWithOptions(
			CompileOptions{
				MaxSchemaDepth:        32,
				MaxSchemaAttributes:   32,
				MaxSchemaTokenBytes:   4096,
				MaxSchemaSourceBytes:  8192,
				MaxSchemaNames:        256,
				MaxFiniteOccurs:       256,
				MaxContentModelStates: 256,
			},
			sourceBytes("fuzz.xsd", []byte(schema)),
		); err != nil {
			return
		}
	})
}

func FuzzValidateNeverPanics(f *testing.F) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="id" type="xs:ID" minOccurs="0" maxOccurs="unbounded"/>
        <xs:element name="ref" type="xs:IDREF" minOccurs="0" maxOccurs="unbounded"/>
      </xs:sequence>
      <xs:attribute name="name" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{
		`<root/>`,
		`<root><id>a</id><ref>a</ref></root>`,
		`<root><id>a</id><id>a</id></root>`,
		`<root><ref>missing</ref></root>`,
		`<other/>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, doc string) {
		if len(doc) > 4096 {
			t.Skip()
		}
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			return
		}
	})
}

func FuzzXSDRegexSyntax(f *testing.F) {
	for _, seed := range []string{
		`[A-Z]{2}\d{4}`,
		`\p{Lu}+`,
		`a|b`,
		`[abc-]`,
		`([a-z]+)?`,
		`\p{}0`,
		`\p{Is}`,
		`\C0`,
		`0{0002}`,
		`0{1001}`,
		`0{1001,}`,
		`0{0,1001}`,
		`0{1001,1000}`,
		`0{0001001}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 256 {
			t.Skip()
		}
		if !utf8.ValidString(source) {
			return
		}
		if err := validateXSDRegexSyntaxWithCompiler(source, nil); err != nil {
			return
		}
		goSource := "^(?:" + translateXSDRegexToGo(source) + ")$"
		if _, err := regexp.Compile(goSource); err != nil {
			t.Fatalf("validated regex does not compile: %q -> %q: %v", source, goSource, err)
		}
	})
}
