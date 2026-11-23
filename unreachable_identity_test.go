package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func TestKeyInGroupSharedByZeroCountAndLiveTypesIsEvaluated(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("shared.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G"><xs:sequence>
    <xs:element name="list"><xs:complexType><xs:sequence><xs:element name="k" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
      <xs:key name="unique-k"><xs:selector xpath="k"/><xs:field xpath="."/></xs:key>
    </xs:element>
  </xs:sequence></xs:group>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:group ref="G"/></xs:complexType></xs:element>
    <xs:element name="live"><xs:complexType><xs:group ref="G"/></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<root><live><list><k>a</k><k>b</k></list></live></root>`)); err != nil {
		t.Fatalf("Validate(distinct keys) error = %v, want valid", err)
	}
	err = engine.Validate(strings.NewReader(`<root><live><list><k>a</k><k>a</k></list></live></root>`))
	const want = `validation.identity at 1:31 /root/live/list/k: duplicate value for identity constraint "unique-k"; first seen at /root/live/list/k`
	if err == nil || err.Error() != want {
		t.Fatalf("Validate(duplicate keys) error = %v, want %s", err, want)
	}
}

func TestKeyrefToKeyNoInstanceReachesReportsUnresolvedReference(t *testing.T) {
	for _, test := range []struct {
		name   string
		holder string
	}{
		{name: "key nested in a zero-count element's type", holder: `<xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:sequence>NESTED</xs:sequence></xs:complexType></xs:element>`},
		{name: "key under a zero-count sequence", holder: `<xs:sequence minOccurs="0" maxOccurs="0">NESTED</xs:sequence>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			nested := `<xs:element name="inner"><xs:complexType><xs:sequence><xs:element name="k" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  <xs:key name="hidden"><xs:selector xpath="k"/><xs:field xpath="."/></xs:key></xs:element>`
			engine, err := xsd.Compile(xsd.Bytes("keyref.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    `+strings.Replace(test.holder, "NESTED", nested, 1)+`
    <xs:element name="r" type="xs:string" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType>
    <xs:keyref name="ref" refer="hidden"><xs:selector xpath="r"/><xs:field xpath="."/></xs:keyref>
  </xs:element>
</xs:schema>`)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			err = engine.Validate(strings.NewReader(`<root><r>a</r></root>`))
			const want = `validation.identity at 1:7 /root/r: keyref value does not match any value of referenced identity constraint "hidden"`
			if err == nil || err.Error() != want {
				t.Fatalf("Validate() error = %v, want %s", err, want)
			}
		})
	}
}
