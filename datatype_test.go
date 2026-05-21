package xsd

import (
	"strings"
	"testing"
)

func TestInstanceReaderZeroLengthReadIsIgnored(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	if err := engine.Validate(&zeroReadThenStringReader{s: "<root/>"}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDerivedEnumerationNarrowsBaseEnumeration(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string">
      <xs:enumeration value="red"/>
      <xs:enumeration value="green"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:enumeration value="green"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="color" type="Derived"/>
</xs:schema>`)
	mustValidate(t, engine, `<color>green</color>`)
	mustNotValidate(t, engine, `<color>red</color>`, ErrValidationFacet)

	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:enumeration value="red"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Bad"><xs:restriction base="Base"><xs:enumeration value="blue"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestPatternFacetTreatsCaretAndDollarAsLiterals(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:pattern value="^abc$"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>^abc$</root>`)
	mustNotValidate(t, engine, `<root>abc</root>`, ErrValidationFacet)
}

func TestPatternEscapedUnsupportedEscapesAreLiterals(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:pattern value="\\i|\\c|a&amp;&amp;b"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	mustValidate(t, engine, `<root>\i</root>`)
	mustValidate(t, engine, `<root>\c</root>`)
	mustValidate(t, engine, `<root>a&amp;&amp;b</root>`)
	mustNotValidate(t, engine, `<root>i</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>a&amp;b</root>`, ErrValidationFacet)
}

func TestInvalidLengthFacetCombinationsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:string"><xs:length value="2"/><xs:minLength value="3"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:string"><xs:minLength value="3"/><xs:maxLength value="2"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:NMTOKENS"><xs:minLength value="0"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestInvalidDigitFacetCombinationIsSchemaError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="Bad"><xs:restriction base="xs:decimal"><xs:totalDigits value="2"/><xs:fractionDigits value="3"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:decimal"><xs:totalDigits value="0"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:byte"><xs:fractionDigits value="1"/></xs:restriction></xs:simpleType>
	</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestSizeFacetsAcceptXSDIntegerLexicalForms(t *testing.T) {
	if _, err := Compile(sourceBytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="Length"><xs:restriction base="xs:string"><xs:length value="+1"/></xs:restriction></xs:simpleType>
	  <xs:simpleType name="MinLength"><xs:restriction base="xs:string"><xs:minLength value="-0"/></xs:restriction></xs:simpleType>
	  <xs:simpleType name="MaxLength"><xs:restriction base="xs:string"><xs:maxLength value="+1"/></xs:restriction></xs:simpleType>
	  <xs:simpleType name="FractionDigits"><xs:restriction base="xs:decimal"><xs:fractionDigits value="-0"/></xs:restriction></xs:simpleType>
	  <xs:simpleType name="TotalDigits"><xs:restriction base="xs:decimal"><xs:totalDigits value="+1"/></xs:restriction></xs:simpleType>
	</xs:schema>`))); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	tests := []struct {
		name  string
		facet string
	}{
		{name: "negative_length", facet: `<xs:length value="-1"/>`},
		{name: "negative_minLength", facet: `<xs:minLength value="-1"/>`},
		{name: "zero_totalDigits", facet: `<xs:totalDigits value="0"/>`},
		{name: "negative_zero_totalDigits", facet: `<xs:totalDigits value="-0"/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := "xs:string"
			if strings.Contains(tt.facet, "totalDigits") {
				base = "xs:decimal"
			}
			_, err := Compile(sourceBytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="Bad"><xs:restriction base="`+base+`">`+tt.facet+`</xs:restriction></xs:simpleType>
	</xs:schema>`)))
			expectCode(t, err, ErrSchemaFacet)
		})
	}
}

func TestFacetValueAttributeIsRequired(t *testing.T) {
	tests := []struct {
		name  string
		base  string
		facet string
	}{
		{name: "enumeration", base: "xs:string", facet: `<xs:enumeration/>`},
		{name: "pattern", base: "xs:string", facet: `<xs:pattern/>`},
		{name: "bound", base: "xs:int", facet: `<xs:minInclusive/>`},
		{name: "whiteSpace", base: "xs:string", facet: `<xs:whiteSpace/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="`+tt.base+`">`+tt.facet+`</xs:restriction></xs:simpleType>
</xs:schema>`)))
			expectCode(t, err, ErrSchemaFacet)
		})
	}
}

func TestFacetApplicabilityIsCheckedAtCompileTime(t *testing.T) {
	tests := []struct {
		name  string
		base  string
		facet string
	}{
		{name: "string_bound", base: "xs:string", facet: `<xs:minInclusive value="a"/>`},
		{name: "boolean_bound", base: "xs:boolean", facet: `<xs:maxInclusive value="true"/>`},
		{name: "binary_bound", base: "xs:hexBinary", facet: `<xs:minExclusive value="00"/>`},
		{name: "string_digits", base: "xs:string", facet: `<xs:totalDigits value="1"/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="`+tt.base+`">`+tt.facet+`</xs:restriction></xs:simpleType>
</xs:schema>`)))
			expectCode(t, err, ErrSchemaFacet)
		})
	}

	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType><xs:restriction base="xs:date"><xs:minInclusive value="2020-01-01"/></xs:restriction></xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>2020-01-01</root>`)
	mustNotValidate(t, engine, `<root>2019-12-31</root>`, ErrValidationFacet)
}

func TestParseDecimalCanonical(t *testing.T) {
	tests := []struct {
		in             string
		canonical      string
		integer        string
		integerLexical bool
		totalDigits    uint32
		fractionDigits uint32
	}{
		{in: "7", canonical: "7.0", integer: "7", integerLexical: true, totalDigits: 1, fractionDigits: 0},
		{in: "+000.0100", canonical: "0.01", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 2},
		{in: "-000", canonical: "0.0", integer: "0", integerLexical: true, totalDigits: 1, fractionDigits: 0},
		{in: ".50", canonical: "0.5", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 1},
		{in: "-.50", canonical: "-0.5", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 1},
		{in: "5.", canonical: "5.0", integer: "5", integerLexical: false, totalDigits: 1, fractionDigits: 0},
		{in: "1000.00", canonical: "1000.0", integer: "1000", integerLexical: false, totalDigits: 4, fractionDigits: 0},
		{in: "0.0010", canonical: "0.001", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 3},
		{in: "+000000000000000000000001234567890.0000000000001000", canonical: "1234567890.0000000000001", integer: "1234567890", integerLexical: false, totalDigits: 23, fractionDigits: 13},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseDecimal(tt.in)
			if err != nil {
				t.Fatalf("parseDecimal() error = %v", err)
			}
			if got.Canonical != tt.canonical || got.IntegerCanonical != tt.integer || got.IntegerLexical != tt.integerLexical || got.TotalDigits != tt.totalDigits || got.FractionDigits != tt.fractionDigits {
				t.Fatalf("parseDecimal() = %+v, want canonical=%q integer=%q integerLexical=%v total=%d fraction=%d", got, tt.canonical, tt.integer, tt.integerLexical, tt.totalDigits, tt.fractionDigits)
			}
		})
	}
}

func TestDecimalAndIntegerCanonicalValuesDiverge(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	decimal, err := validateSimpleValueInfo(engine.rt, engine.rt.Builtin.Decimal, "5", nil)
	if err != nil {
		t.Fatalf("validateSimpleValueInfo(decimal) error = %v", err)
	}
	if decimal.Canonical != "5.0" {
		t.Fatalf("decimal canonical = %q, want 5.0", decimal.Canonical)
	}
	integer, err := validateSimpleValueInfo(engine.rt, engine.rt.Builtin.Int, "05", nil)
	if err != nil {
		t.Fatalf("validateSimpleValueInfo(int) error = %v", err)
	}
	if integer.Canonical != "5" {
		t.Fatalf("int canonical = %q, want 5", integer.Canonical)
	}
}

func TestUserDerivedBuiltinLexicalRulesAreEnforced(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="MyInt"><xs:restriction base="xs:int"/></xs:simpleType>
	  <xs:simpleType name="MyUnsignedInt"><xs:restriction base="xs:unsignedInt"/></xs:simpleType>
	  <xs:simpleType name="MyNCName"><xs:restriction base="xs:NCName"/></xs:simpleType>
	  <xs:simpleType name="MyID"><xs:restriction base="xs:ID"/></xs:simpleType>
	  <xs:simpleType name="MyLanguage"><xs:restriction base="xs:language"/></xs:simpleType>
	  <xs:element name="i" type="MyInt"/>
	  <xs:element name="u" type="MyUnsignedInt"/>
	  <xs:element name="n" type="MyNCName"/>
	  <xs:element name="id" type="MyID"/>
	  <xs:element name="l" type="MyLanguage"/>
	</xs:schema>`)
	mustValidate(t, engine, `<i>1</i>`)
	mustValidate(t, engine, `<u>1</u>`)
	mustValidate(t, engine, `<n>name</n>`)
	mustValidate(t, engine, `<id>abc</id>`)
	mustValidate(t, engine, `<l>en-US</l>`)
	mustNotValidate(t, engine, `<i>1.0</i>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<u>-1</u>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<n>a:b</n>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<id>a:b</id>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<l>en_US</l>`, ErrValidationFacet)
}

func TestParseDecimalRejectsInvalidLexicalValues(t *testing.T) {
	for _, in := range []string{"", "+", "-", ".", "+.", "-.", "1.2.3", "12a", "1e2"} {
		t.Run(in, func(t *testing.T) {
			if _, err := parseDecimal(in); err == nil {
				t.Fatal("parseDecimal() error = nil")
			}
		})
	}
}

func TestTotalDigitsCountsIntegerTrailingZeros(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:long">
        <xs:totalDigits value="4"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>9999</root>`)
	mustNotValidate(t, engine, `<root>10000</root>`, ErrValidationFacet)
}

func TestInvalidDecimalBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:integer"><xs:minExclusive value="101"/><xs:maxInclusive value="100"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:integer"><xs:minInclusive value="101"/><xs:maxExclusive value="100"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:decimal"><xs:minInclusive value="10.0"/><xs:maxExclusive value="10"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:byte"><xs:maxInclusive value="128"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:byte"><xs:maxInclusive value="5"/><xs:maxExclusive value="5"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestInvalidTemporalBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:dateTime"><xs:maxInclusive value="1985-04-12T10:30:00"/><xs:maxExclusive value="1985-04-12T10:30:00"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:dateTime"><xs:minExclusive value="1999-05-12T10:31:00"/><xs:maxExclusive value="1981-03-12T10:30:00"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestDateTimeEnumerationComparesEquivalentUTCValues(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:restriction base="xs:dateTime">
      <xs:enumeration value="2002-01-01T12:01:01-00:00"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="t"/>
</xs:schema>`)
	mustValidate(t, engine, `<root>2002-01-01T12:01:01Z</root>`)
	mustValidate(t, engine, `<root>2002-01-01T12:01:01+00:00</root>`)
}

func TestDateTimeBoundsUsePartialTimezoneOrder(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:simpleType name="afterFixedDateTime">
    <xs:restriction base="xs:dateTime">
      <xs:minInclusive value="2000-01-20T12:00:00Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="afterFixedDate">
    <xs:restriction base="xs:date">
      <xs:minInclusive value="2000-01-20Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="dt" type="t:afterFixedDateTime"/>
  <xs:element name="d" type="t:afterFixedDate"/>
</xs:schema>`)
	mustValidate(t, engine, `<dt xmlns="urn:test">2000-01-21T03:00:01</dt>`)
	mustNotValidate(t, engine, `<dt xmlns="urn:test">2000-01-20T12:00:00</dt>`, ErrValidationFacet)
	mustValidate(t, engine, `<d xmlns="urn:test">2000-01-21</d>`)
	mustNotValidate(t, engine, `<d xmlns="urn:test">2000-01-20</d>`, ErrValidationFacet)
}

func TestTimeBoundsUsePartialTimezoneOrder(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="AfterNoonUTC">
    <xs:restriction base="xs:time">
      <xs:minInclusive value="12:00:00Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="BeforeNoonUTC">
    <xs:restriction base="xs:time">
      <xs:maxInclusive value="12:00:00Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="after" type="AfterNoonUTC"/>
  <xs:element name="before" type="BeforeNoonUTC"/>
</xs:schema>`)
	mustValidate(t, engine, `<after>12:00:00Z</after>`)
	mustNotValidate(t, engine, `<after>12:00:00</after>`, ErrValidationFacet)
	mustValidate(t, engine, `<before>12:00:00Z</before>`)
	mustNotValidate(t, engine, `<before>12:00:00</before>`, ErrValidationFacet)
}

func TestTimeRestrictionRejectsIncomparableTimezoneBounds(t *testing.T) {
	tests := []string{
		`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:time">
      <xs:minInclusive value="12:00:00Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Bad">
    <xs:restriction base="Base">
      <xs:minInclusive value="12:00:00"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
		`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:time">
      <xs:maxInclusive value="12:00:00Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Bad">
    <xs:restriction base="Base">
      <xs:maxInclusive value="12:00:00"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
	}
	for _, schema := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(schema)))
		expectCode(t, err, ErrSchemaFacet)
	}
}

func TestAnyURIRejectsInvalidXSD10LexicalValues(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:anyURI"><xs:enumeration value=":a"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:anyURI"><xs:enumeration value="%"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyURI"/>
</xs:schema>`)
	mustValidate(t, engine, `<root>foo&gt;bar</root>`)
	mustNotValidate(t, engine, `<root>\a</root>`, ErrValidationFacet)
}

func TestBase64BinaryRejectsNonZeroPaddingBits(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:base64Binary"><xs:enumeration value="M0SyLMT="/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestDoubleRejectsInvalidXSD10SpecialValues(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:double"/>
</xs:schema>`)
	mustValidate(t, engine, `<root>INF</root>`)
	mustValidate(t, engine, `<root>-INF</root>`)
	mustValidate(t, engine, `<root>NaN</root>`)
	mustNotValidate(t, engine, `<root>+INF</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>inf</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>nan</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>NAN</root>`, ErrValidationFacet)
}

func TestDoubleAcceptsLexicalValuesOutsideFiniteRange(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:double"/>
</xs:schema>`)
	mustValidate(t, engine, `<root>28364477384374416E294</root>`)
	mustValidate(t, engine, `<root>1E-9999</root>`)
}

func TestDoubleBoundsAreValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:double">
        <xs:maxExclusive value="1.1"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>1.0</root>`)
	mustNotValidate(t, engine, `<root>1.1</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>5.55</root>`, ErrValidationFacet)
}

func TestInvalidDoubleBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:double"><xs:minInclusive value="7.7"/><xs:maxInclusive value="1.1"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:double"><xs:minExclusive value="7.7"/><xs:maxExclusive value="1.1"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestDurationBoundsAreValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:duration">
        <xs:maxExclusive value="P1Y1MT1H"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>P1Y</root>`)
	mustNotValidate(t, engine, `<root>P1Y1MT1H</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>P1Y2MT2H</root>`, ErrValidationFacet)

	nist := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:duration">
        <xs:maxExclusive value="P1990Y06M11DT15H00M05S"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, nist, `<root>P1986Y04M24DT00H21M12S</root>`)

	incomparable := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:duration">
        <xs:maxExclusive value="P30D"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, incomparable, `<root>P1M</root>`, ErrValidationFacet)
}

func TestInvalidDurationBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:duration"><xs:minInclusive value="P2Y3MT2H"/><xs:maxInclusive value="P1Y1MT1H"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:duration"><xs:minExclusive value="P2Y3MT2H"/><xs:maxExclusive value="P1Y1MT1H"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestLargeDurationBoundsDoNotOverflow(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Span">
    <xs:restriction base="xs:duration">
      <xs:minInclusive value="-P10675199DT2H48M5.4775808S"/>
      <xs:maxInclusive value="P10675199DT2H48M5.4775807S"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)
}

func TestDurationBoundsCompareFractionalSecondsExactly(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:duration">
        <xs:maxInclusive value="PT0.10000000000000000001S"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>PT0.10000000000000000001S</root>`)
	mustNotValidate(t, engine, `<root>PT0.10000000000000000002S</root>`, ErrValidationFacet)
}

func TestDurationComparisonUsesBoundedDateArithmetic(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:duration">
        <xs:minInclusive value="P1M"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>P1000000000D</root>`)
	mustValidate(t, engine, `<root>PT9223372036854775807S</root>`)
}

func TestGDayBoundsAreValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:gDay">
        <xs:maxExclusive value="---30"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>---15</root>`)
	mustNotValidate(t, engine, `<root>---30</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>---31</root>`, ErrValidationFacet)
}

func TestInvalidGDayBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gDay"><xs:minInclusive value="---30"/><xs:maxInclusive value="---01"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gDay"><xs:minExclusive value="---30"/><xs:maxExclusive value="---01"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestGMonthDayBoundsAreValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:gMonthDay">
        <xs:maxExclusive value="--10-01"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>--03-15</root>`)
	mustNotValidate(t, engine, `<root>--10-01</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>--12-01</root>`, ErrValidationFacet)
}

func TestInvalidGMonthDayBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gMonthDay"><xs:minInclusive value="--10-01"/><xs:maxInclusive value="--01-01"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gMonthDay"><xs:minExclusive value="--10-01"/><xs:maxExclusive value="--01-01"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestGMonthBoundsAreValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:gMonth">
        <xs:maxExclusive value="--02"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>--01</root>`)
	mustNotValidate(t, engine, `<root>--02</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>--12</root>`, ErrValidationFacet)
}

func TestGYearMonthBoundsAreValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:gYearMonth">
        <xs:maxExclusive value="2002-10"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>2002-03</root>`)
	mustNotValidate(t, engine, `<root>2002-10</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>2003-01</root>`, ErrValidationFacet)
}

func TestInvalidGYearMonthBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gYearMonth"><xs:minInclusive value="2002-10"/><xs:maxInclusive value="2002-01"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gYearMonth"><xs:minExclusive value="2002-10"/><xs:maxExclusive value="2002-01"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestGYearBoundsAreValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:gYear">
        <xs:maxExclusive value="2002"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>2001</root>`)
	mustNotValidate(t, engine, `<root>2002</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>2003</root>`, ErrValidationFacet)
}

func TestInvalidGYearBoundsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gYear"><xs:minInclusive value="2002"/><xs:maxInclusive value="2001"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:gYear"><xs:minExclusive value="2002"/><xs:maxExclusive value="2001"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestGDateTimeEnumerationPreservesTimezone(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="GY"><xs:restriction base="xs:gYear"><xs:enumeration value="2000Z"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="GYM"><xs:restriction base="xs:gYearMonth"><xs:enumeration value="2000-01Z"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="GMD"><xs:restriction base="xs:gMonthDay"><xs:enumeration value="--05-01Z"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="GD"><xs:restriction base="xs:gDay"><xs:enumeration value="---01Z"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="GM"><xs:restriction base="xs:gMonth"><xs:enumeration value="--05Z"/></xs:restriction></xs:simpleType>
  <xs:element name="y" type="GY"/>
  <xs:element name="ym" type="GYM"/>
  <xs:element name="md" type="GMD"/>
  <xs:element name="d" type="GD"/>
  <xs:element name="m" type="GM"/>
</xs:schema>`)
	mustValidate(t, engine, `<y>2000+00:00</y>`)
	mustNotValidate(t, engine, `<y>2000</y>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<y>2000+14:00</y>`, ErrValidationFacet)
	mustValidate(t, engine, `<ym>2000-01+00:00</ym>`)
	mustNotValidate(t, engine, `<ym>2000-01</ym>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<ym>2000-01+14:00</ym>`, ErrValidationFacet)
	mustValidate(t, engine, `<md>--05-01+00:00</md>`)
	mustNotValidate(t, engine, `<md>--05-01</md>`, ErrValidationFacet)
	mustValidate(t, engine, `<d>---01+00:00</d>`)
	mustNotValidate(t, engine, `<d>---01</d>`, ErrValidationFacet)
	mustValidate(t, engine, `<m>--05+00:00</m>`)
	mustNotValidate(t, engine, `<m>--05</m>`, ErrValidationFacet)
}

func TestGDateTimeBoundsUseTimezoneAwareStartInstant(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:gYear">
        <xs:minInclusive value="2000Z"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>2000Z</root>`)
	mustNotValidate(t, engine, `<root>2000+14:00</root>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<root>2000</root>`, ErrValidationFacet)
}

func TestInvalidWhitespaceFacetRestrictionIsSchemaError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:normalizedString"><xs:whiteSpace value="preserve"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:token"><xs:whiteSpace value="replace"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		mode whitespaceMode
	}{
		{name: "preserve", in: " a\tb\n", mode: whitespacePreserve, want: " a\tb\n"},
		{name: "replace", in: "a\tb\nc\rd", mode: whitespaceReplace, want: "a b c d"},
		{name: "collapse", in: " \ta  b\nc\r ", mode: whitespaceCollapse, want: "a b c"},
		{name: "collapse unchanged", in: "abc", mode: whitespaceCollapse, want: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeWhitespace(tt.in, tt.mode); got != tt.want {
				t.Fatalf("normalizeWhitespace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListFacetsUseCollapsedLexicalAndCanonicalValues(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="ints">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="patterned">
    <xs:restriction base="ints">
      <xs:pattern value="1 2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="enumerated">
    <xs:restriction base="ints">
      <xs:enumeration value="1 2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="patterned" type="patterned"/>
  <xs:element name="enumerated" type="enumerated"/>
</xs:schema>`)
	mustValidate(t, engine, "<patterned> 1 \n 2 </patterned>")
	mustNotValidate(t, engine, "<patterned>1 3</patterned>", ErrValidationFacet)
	mustValidate(t, engine, "<enumerated>01 2</enumerated>")
	mustNotValidate(t, engine, "<enumerated>1 3</enumerated>", ErrValidationFacet)
}

func TestParseXSDFloatLexical(t *testing.T) {
	valid := []string{"0", "+1", "-1.5", "1.", ".5", "1e2", "1.E-2", "INF", "-INF", "NaN"}
	for _, s := range valid {
		t.Run("valid "+s, func(t *testing.T) {
			if _, err := parseXSDFloat(s, 64); err != nil {
				t.Fatalf("parseXSDFloat(%q) error = %v", s, err)
			}
		})
	}
	invalid := []string{"", "+", ".", "+.", "1e", "1e+", "+INF", "nan", "1 2"}
	for _, s := range invalid {
		t.Run("invalid "+s, func(t *testing.T) {
			if _, err := parseXSDFloat(s, 64); err == nil {
				t.Fatalf("parseXSDFloat(%q) succeeded", s)
			}
		})
	}
}

func TestIsXMLWhitespaceBytes(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{name: "empty", in: nil, want: true},
		{name: "xml whitespace", in: []byte(" \t\r\n"), want: true},
		{name: "text", in: []byte(" a "), want: false},
		{name: "unicode space", in: []byte("\u00a0"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isXMLWhitespaceBytes(tt.in); got != tt.want {
				t.Fatalf("isXMLWhitespaceBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstanceAttributeTabIsPreservedForPatternValidation(t *testing.T) {
	engine := mustCompile(t, "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\"><xs:element name=\"r\"><xs:complexType><xs:attribute name=\"a\"><xs:simpleType><xs:restriction base=\"xs:string\"><xs:pattern value=\"x\ty\"/></xs:restriction></xs:simpleType></xs:attribute></xs:complexType></xs:element></xs:schema>")
	mustValidate(t, engine, "<r a=\"x\ty\"/>")
}

func TestOrderedFacetsAreRejectedForListAndUnionTypes(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="Ints"><xs:list itemType="xs:int"/></xs:simpleType>
  <xs:simpleType name="Bad"><xs:restriction base="Ints"><xs:minInclusive value="1"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U"><xs:union memberTypes="xs:unsignedInt xs:string"/></xs:simpleType>
  <xs:simpleType name="Bad"><xs:restriction base="U"><xs:minInclusive value="1"/></xs:restriction></xs:simpleType>
	</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestDirectListAndUnionEnumerationValuesUseMemberValueSpace(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="Bad"><xs:list itemType="xs:int"><xs:enumeration value="x"/></xs:list></xs:simpleType>
	</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="Bad"><xs:union memberTypes="xs:int"><xs:enumeration value="x"/></xs:union></xs:simpleType>
	</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="One"><xs:union memberTypes="xs:int"><xs:enumeration value="01"/></xs:union></xs:simpleType>
	  <xs:element name="root" type="One"/>
	</xs:schema>`)
	mustValidate(t, engine, `<root>1</root>`)
}

func TestUnionValueFailureReportsUnionFailure(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="U"><xs:union memberTypes="xs:int xs:boolean"/></xs:simpleType>
  <xs:element name="root" type="U"/>
</xs:schema>`)
	err := engine.Validate(strings.NewReader(`<root>nope</root>`))
	expectCode(t, err, ErrValidationFacet)
	if !strings.Contains(err.Error(), "value does not match any union member") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestUnionReportsFailureWhenNoMemberMatches(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U"><xs:union memberTypes="xs:date xs:int"/></xs:simpleType>
  <xs:element name="root" type="U"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<root>not-a-date-or-int</root>`, ErrValidationFacet)
}

func TestUnionAcceptsExpandedYearDateMember(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U"><xs:union memberTypes="xs:date xs:string"/></xs:simpleType>
  <xs:element name="root" type="U"/>
</xs:schema>`)
	mustValidate(t, engine, `<root>10000-01-01</root>`)
}

func TestTimeRejectsSecondSixty(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:time"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<root>13:20:60</root>`, ErrValidationFacet)
}

func TestTimeRestrictionCannotLoosenBaseUpperBound(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:time">
      <xs:maxInclusive value="12:00:00-10:00"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Bad">
    <xs:restriction base="Base">
      <xs:maxInclusive value="12:00:00-14:00"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestComplexContentCannotContainSimpleFacets(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="xs:anyType"><xs:length value="9"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestMissingListItemTypeInvalidatesValues(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="list"><xs:list itemType="absent"/></xs:simpleType>
  <xs:element name="bad" type="list"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<bad>1 2</bad>`, ErrValidationFacet)
}

func TestListItemTypeMustBeSimple(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="complex"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:simpleType name="list"><xs:list itemType="complex"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestSchemaQNameAttributesAreWhitespaceCollapsed(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="    xs:string "/></xs:simpleType>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestQNameLengthFacetsAreAlwaysValid(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="q">
    <xs:restriction base="xs:QName">
      <xs:length value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="q"/>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns:p="urn:p">p:name</root>`)
}

func TestQNameValueRejectsMalformedLexicalName(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:QName"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<root>:name</root>`, ErrValidationFacet)
}

func TestElementValueConstraintRejectsBareNotationUnionMember(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" fixed="25">
    <xs:simpleType>
      <xs:union memberTypes="xs:int xs:NOTATION"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:union memberTypes="xs:int xs:NOTATION"/>
  </xs:simpleType>
  <xs:element name="root" type="t"/>
	</xs:schema>`)
}

func TestNotationDeclarationAndValidation(t *testing.T) {
	e := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:notation name="gif" public="image/gif"/>
  <xs:simpleType name="NotationType">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="tns:gif"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="kind" type="tns:NotationType" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	mustValidate(t, e, `<root xmlns="urn:test" xmlns:tns="urn:test" kind="tns:gif"/>`)

	err := e.Validate(strings.NewReader(`<root xmlns="urn:test" xmlns:tns="urn:test" kind="tns:png"/>`))
	expectCode(t, err, ErrValidationFacet)
}

func TestNotationEnumerationRequiresDeclaredNotation(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:notation name="gif" public="image/gif"/>
  <xs:simpleType name="NotationType">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="tns:png"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestInvalidNotationDeclarationIsSchemaError(t *testing.T) {
	for _, tc := range []struct {
		schema string
		code   ErrorCode
	}{
		{`<xs:notation public="image/gif"/>`, ErrSchemaInvalidAttribute},
		{`<xs:notation name="gif"/>`, ErrSchemaInvalidAttribute},
		{`<xs:notation name="gif" public="image/gif"><xs:element name="bad"/></xs:notation>`, ErrSchemaContentModel},
		{`<xs:notation name="gif" public="image/gif">text</xs:notation>`, ErrSchemaContentModel},
		{`<xs:notation name="gif" public="image/gif"><other/></xs:notation>`, ErrSchemaContentModel},
		{`<xs:annotation><xs:notation name="gif" public="image/gif"/></xs:annotation>`, ErrSchemaContentModel},
	} {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+tc.schema+`</xs:schema>`)))
		expectCode(t, err, tc.code)
	}

	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation>
    <xs:appinfo>
      <xs:notation name="payload" public="not-a-schema-component"/>
    </xs:appinfo>
  </xs:annotation>
</xs:schema>`)
}

func TestSimpleTypeFinalBlocksListAndUnion(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="base" final="list"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:simpleType name="list"><xs:list itemType="base"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="base" final="union"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:simpleType name="union"><xs:union memberTypes="base"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="base" final="extension"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestListItemTypeCannotBeListType(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="items"><xs:list itemType="xs:int"/></xs:simpleType>
  <xs:simpleType name="bad"><xs:list itemType="items"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestSimpleContentRestrictionFacetsApplyToBaseTextType(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:simpleContent>
      <xs:restriction base="base"><xs:minLength value="2"/></xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidate(t, engine, `<root>ab</root>`)
	mustNotValidate(t, engine, `<root>a</root>`, ErrValidationFacet)

	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:simpleContent>
      <xs:extension base="xs:string"><xs:anyAttribute namespace="##any"/></xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:simpleContent>
      <xs:restriction base="base">
        <xs:length value="5"/>
        <xs:attribute name="code"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)

	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:simpleContent><xs:extension base="xs:anySimpleType"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="bad">
    <xs:simpleContent>
      <xs:restriction base="base"><xs:minLength value="1"/></xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestInvalidRegexSyntaxIsSchemaError(t *testing.T) {
	tests := []string{
		`b\z`,
		`a]`,
		`a[]]b`,
		`a[^]b]c`,
		`\ba\b`,
		`a.+?c`,
		`)(`,
		`ab{1,3}?bc`,
		`ab??bc`,
		`[a-f-[]]+`,
		`[[abcd]-[bc]]+`,
		`[^-[bc]]`,
		`foo([7-\w]*)`,
		`a{,2}`,
		`(ab){2,0}`,
		`[^a-d-b-c]`,
		`[a-c-1-4x-z-7-9]*`,
		`[a-a-x-x]+`,
		`[000--0]`,
		`\p{}0`,
		`\p{0}`,
		`\p{Is}`,
	}
	for _, pattern := range tests {
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="t"><xs:restriction base="xs:string"><xs:pattern value="` + pattern + `"/></xs:restriction></xs:simpleType></xs:schema>`
		_, err := Compile(sourceBytes("schema.xsd", []byte(schema)))
		expectCode(t, err, ErrSchemaFacet)
	}
}

func TestUnsupportedRegexEscapesAreExplicit(t *testing.T) {
	tests := []string{
		`\C0`,
		`\I0`,
		`\p{IsBasicLatin}`,
		`[.\w]`,
		`[\w:]`,
		`[\w-]`,
		`[.\D]`,
		`[.\W]`,
		`[.\S]`,
	}
	for _, pattern := range tests {
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="t"><xs:restriction base="xs:string"><xs:pattern value="` + pattern + `"/></xs:restriction></xs:simpleType></xs:schema>`
		_, err := Compile(sourceBytes("schema.xsd", []byte(schema)))
		expectCategoryCode(t, err, UnsupportedErrorCategory, ErrUnsupportedRegex)
	}
}

func TestRegexRepeatQuantifiersRespectGoLimit(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:pattern value="0{0002}"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root>00</root>`)
	mustNotValidate(t, engine, `<root>0{02}</root>`, ErrValidationFacet)

	if _, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="0{1000}"/></xs:restriction></xs:simpleType>
</xs:schema>`))); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	for _, pattern := range []string{
		`0{1001}`,
		`0{1001,}`,
		`0{0,1001}`,
		`0{0001001}`,
	} {
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="` + pattern + `"/></xs:restriction></xs:simpleType></xs:schema>`
		_, err := Compile(sourceBytes("schema.xsd", []byte(schema)))
		expectCategoryCode(t, err, UnsupportedErrorCategory, ErrUnsupportedRegex)
	}

	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="0{1001,1000}"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestSingletonRegexClassEscapesRemainSupported(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="word"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[\w]"/></xs:restriction></xs:simpleType></xs:element>
  <xs:element name="notWord"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[\W]"/></xs:restriction></xs:simpleType></xs:element>
  <xs:element name="notDigit"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[\D]"/></xs:restriction></xs:simpleType></xs:element>
  <xs:element name="notSpace"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[\S]"/></xs:restriction></xs:simpleType></xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<word>A</word>`)
	mustNotValidate(t, engine, `<word> </word>`, ErrValidationFacet)
	mustValidate(t, engine, `<notWord> </notWord>`)
	mustNotValidate(t, engine, `<notWord>A</notWord>`, ErrValidationFacet)
	mustValidate(t, engine, `<notDigit>A</notDigit>`)
	mustNotValidate(t, engine, `<notDigit>1</notDigit>`, ErrValidationFacet)
	mustValidate(t, engine, `<notSpace>A</notSpace>`)
	mustNotValidate(t, engine, "<notSpace>\t</notSpace>", ErrValidationFacet)
}

func TestRegexWhitespaceUsesXMLWhitespace(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="space"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="\s"/></xs:restriction></xs:simpleType></xs:element>
  <xs:element name="notSpace"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="\S"/></xs:restriction></xs:simpleType></xs:element>
</xs:schema>`)
	mustValidate(t, engine, "<space>\t</space>")
	mustValidate(t, engine, "<space>\n</space>")
	mustNotValidate(t, engine, `<space>A</space>`, ErrValidationFacet)
	mustValidate(t, engine, `<notSpace>A</notSpace>`)
	mustNotValidate(t, engine, "<notSpace>\t</notSpace>", ErrValidationFacet)
}

func TestNMTOKENSAndEntityTypes(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="tokens" type="xs:NMTOKENS"/>
  <xs:element name="entity" type="xs:ENTITY"/>
  <xs:element name="entities" type="xs:ENTITIES"/>
</xs:schema>`)
	mustValidate(t, engine, `<tokens>a b c</tokens>`)
	mustNotValidate(t, engine, `<tokens/>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<tokens>a &amp;</tokens>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<entity>declared</entity>`, ErrUnsupportedEntity)
	mustNotValidate(t, engine, `<entities>a b</entities>`, ErrUnsupportedEntity)
}

func TestLanguageDatatype(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="lang" type="xs:language"/></xs:schema>`)
	mustValidate(t, engine, `<lang>en-US</lang>`)
	mustNotValidate(t, engine, `<lang>en_US</lang>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<lang>toolonglang-US</lang>`, ErrValidationFacet)
}

func TestDurationDatatype(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="duration" type="xs:duration"/></xs:schema>`)
	mustValidate(t, engine, `<duration>P1Y2M3DT4H5M6.7S</duration>`)
	mustValidate(t, engine, `<duration>-PT0S</duration>`)
	mustValidate(t, engine, `<duration>P3D</duration>`)
	mustNotValidate(t, engine, `<duration>P</duration>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<duration>PT</duration>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<duration>P1.5Y</duration>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<duration>PT1.0H</duration>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<duration>PT1.0M</duration>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<duration>P1H</duration>`, ErrValidationFacet)
}

func TestBinaryLengthFacetsCountOctets(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="hex">
    <xs:simpleType><xs:restriction base="xs:hexBinary"><xs:length value="2"/></xs:restriction></xs:simpleType>
  </xs:element>
  <xs:element name="b64">
    <xs:simpleType><xs:restriction base="xs:base64Binary"><xs:minLength value="2"/><xs:maxLength value="3"/></xs:restriction></xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<hex>0AFF</hex>`)
	mustNotValidate(t, engine, `<hex>0A</hex>`, ErrValidationFacet)
	mustValidate(t, engine, `<b64>AQID</b64>`)
	mustValidate(t, engine, `<b64>AQI=</b64>`)
	mustNotValidate(t, engine, `<b64>AQ==</b64>`, ErrValidationFacet)
}

func TestBinaryCanonicalFacets(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="hex">
    <xs:simpleType><xs:restriction base="xs:hexBinary"><xs:enumeration value="0AFF"/></xs:restriction></xs:simpleType>
  </xs:element>
  <xs:element name="b64">
    <xs:simpleType><xs:restriction base="xs:base64Binary"><xs:enumeration value="AQI="/></xs:restriction></xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<hex>0aff</hex>`)
	mustValidate(t, engine, `<b64>A Q I =</b64>`)
	mustNotValidate(t, engine, `<b64>AB==</b64>`, ErrValidationFacet)
}

func TestDateTimeBounds(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="day">
    <xs:simpleType>
      <xs:restriction base="xs:date">
        <xs:minInclusive value="2026-01-01"/>
        <xs:maxExclusive value="2027-01-01"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<day>2026-05-03</day>`)
	mustNotValidate(t, engine, `<day>2027-01-01</day>`, ErrValidationFacet)
}

func TestTimeLeapSecondAndOffsetEquivalence(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="fixed" type="xs:time" fixed="23:59:60+02:00"/>
  <xs:element name="bounded">
    <xs:simpleType>
      <xs:restriction base="xs:time">
        <xs:minInclusive value="10:30:00Z"/>
        <xs:maxInclusive value="10:30:00Z"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<fixed>22:00:00Z</fixed>`)
	mustValidate(t, engine, `<bounded>00:30:00+14:00</bounded>`)
	mustNotValidate(t, engine, `<bounded>00:29:59+14:00</bounded>`, ErrValidationFacet)
}

func TestDateTimeLeapSecondAndOffsetEquivalence(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="plain" type="xs:dateTime"/>
  <xs:element name="fixed" type="xs:dateTime" fixed="1998-12-31T23:59:60Z"/>
  <xs:element name="bounded">
    <xs:simpleType>
      <xs:restriction base="xs:dateTime">
        <xs:minInclusive value="1998-12-31T23:59:60Z"/>
        <xs:maxInclusive value="1998-12-31T23:59:60Z"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<plain>1998-12-31T23:59:60Z</plain>`)
	mustValidate(t, engine, `<plain>1998-12-31T23:59:60</plain>`)
	mustValidate(t, engine, `<fixed>1999-01-01T00:00:00Z</fixed>`)
	mustValidate(t, engine, `<bounded>1999-01-01T00:00:00Z</bounded>`)
	mustNotValidate(t, engine, `<plain>1998-12-31T22:59:60Z</plain>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<plain>1998-12-31T23:59:61Z</plain>`, ErrValidationFacet)
}

func TestDateTimeAndTimeAcceptArbitraryFractionalPrecision(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="plain" type="xs:dateTime"/>
  <xs:element name="time" type="xs:time"/>
</xs:schema>`)
	mustValidate(t, engine, `<plain>2000-01-01T00:00:00.123456789Z</plain>`)
	mustValidate(t, engine, `<time>00:00:00.123456789Z</time>`)
	mustValidate(t, engine, `<plain>2000-01-01T00:00:00.1234567891Z</plain>`)
	mustValidate(t, engine, `<time>00:00:00.1234567891Z</time>`)
}

func TestTemporalFacetAcceptsArbitraryFractionalPrecision(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{
			name: "dateTime fixed",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="fixed" type="xs:dateTime" fixed="2000-01-01T00:00:00.1234567891Z"/>
</xs:schema>`,
		},
		{
			name: "dateTime facet",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bounded">
    <xs:restriction base="xs:dateTime">
      <xs:minInclusive value="2000-01-01T00:00:00.1234567891Z"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
		},
		{
			name: "time fixed",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="fixed" type="xs:time" fixed="00:00:00.1234567891Z"/>
</xs:schema>`,
		},
		{
			name: "time facet",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bounded">
    <xs:restriction base="xs:time">
      <xs:minInclusive value="00:00:00.1234567891Z"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(sourceBytes("schema.xsd", []byte(tt.schema)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
		})
	}
}

func TestGDateTimeDatatypesAndExpandedYears(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="ym" type="xs:gYearMonth"/>
  <xs:element name="y" type="xs:gYear"/>
  <xs:element name="md" type="xs:gMonthDay"/>
  <xs:element name="d" type="xs:gDay"/>
  <xs:element name="m" type="xs:gMonth"/>
  <xs:element name="date" type="xs:date"/>
</xs:schema>`)
	mustValidate(t, engine, `<ym>2026-05Z</ym>`)
	mustValidate(t, engine, `<y>2026</y>`)
	mustValidate(t, engine, `<ym>12000-11</ym>`)
	mustValidate(t, engine, `<y>10000</y>`)
	mustValidate(t, engine, `<ym>-12000-11</ym>`)
	mustValidate(t, engine, `<y>-0001</y>`)
	mustValidate(t, engine, `<md>--02-29</md>`)
	mustValidate(t, engine, `<d>---31</d>`)
	mustValidate(t, engine, `<m>--12</m>`)
	mustNotValidate(t, engine, `<ym>2026-13</ym>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<md>--02-30</md>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<date>0000-01-01</date>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<date>+2001-01-01</date>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<ym>99-10</ym>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<ym>02026-05</ym>`, ErrValidationFacet)
	mustValidate(t, engine, `<date>10000-01-01</date>`)
	mustValidate(t, engine, `<date>-0001-01-01</date>`)
	mustValidate(t, engine, `<date>-0001-02-29</date>`)
	mustNotValidate(t, engine, `<date>-0004-02-29</date>`, ErrValidationFacet)
}

func TestTemporalParsesExactXSDLexicalEdges(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="date" type="xs:date"/>
  <xs:element name="time" type="xs:time"/>
  <xs:element name="dateTime" type="xs:dateTime"/>
</xs:schema>`)
	mustValidate(t, engine, `<date>2026-05-18+14:00</date>`)
	mustValidate(t, engine, `<date>2026-05-18-14:00</date>`)
	mustNotValidate(t, engine, `<date>2026-05-18+14:01</date>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<date>2026-05-18+15:00</date>`, ErrValidationFacet)
	mustValidate(t, engine, `<time>24:00:00</time>`)
	mustValidate(t, engine, `<time>24:00:00.0Z</time>`)
	mustNotValidate(t, engine, `<time>24:00:00.1</time>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<time>24:01:00</time>`, ErrValidationFacet)
	mustValidate(t, engine, `<dateTime>2026-05-18T24:00:00Z</dateTime>`)
	mustValidate(t, engine, `<dateTime>-0001-12-31T24:00:00.0</dateTime>`)
	mustNotValidate(t, engine, `<dateTime>2026-05-18T24:00:01</dateTime>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<dateTime>02026-05-18T00:00:00</dateTime>`, ErrValidationFacet)
}

func TestTemporalFixedAndEnumerationUseValueKeys(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="dateTimeFixed" type="xs:dateTime" fixed="9999-12-31T24:00:00Z"/>
  <xs:element name="timeFixed" type="xs:time" fixed="24:00:00.0000000000Z"/>
  <xs:element name="dateFixed" type="xs:date" fixed="2002-10-10+13:00"/>
  <xs:element name="dateEnum">
    <xs:simpleType>
      <xs:restriction base="xs:date">
        <xs:enumeration value="2002-10-10+13:00"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<dateTimeFixed>10000-01-01T00:00:00Z</dateTimeFixed>`)
	mustValidate(t, engine, `<timeFixed>00:00:00Z</timeFixed>`)
	mustValidate(t, engine, `<dateFixed>2002-10-09-11:00</dateFixed>`)
	mustValidate(t, engine, `<dateEnum>2002-10-09-11:00</dateEnum>`)
}
