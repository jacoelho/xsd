package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func TestNamespaceAttributeSourceResolution(t *testing.T) {
	for _, ns := range []struct {
		name, uri, prefix, attribute, valid, invalid string
	}{
		{"XML", "http://www.w3.org/XML/1998/namespace", "xml", "lang", "en", "fr"},
		{"XLink", "http://www.w3.org/1999/xlink", "xl", "actuate", "onLoad", "bogus"},
	} {
		for _, assembly := range []string{"resolver", "explicit roots", "reversed roots"} {
			t.Run(ns.name+"/"+assembly, func(t *testing.T) {
				const location = "https://schemas.example.test/attributes.xsd"
				declarations := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="%s">
  <xs:attribute name="%s"><xs:simpleType><xs:restriction base="xs:token">
    <xs:enumeration value="%s"/>
  </xs:restriction></xs:simpleType></xs:attribute>
</xs:schema>`, ns.uri, ns.attribute, ns.valid)
				importLocation := ""
				if assembly == "resolver" {
					importLocation = ` schemaLocation="` + location + `"`
				}
				principal := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:%s="%s">
  <xs:import namespace="%s"%s/>
  <xs:element name="root"><xs:complexType><xs:attribute ref="%s:%s"/></xs:complexType></xs:element>
</xs:schema>`, ns.prefix, ns.uri, ns.uri, importLocation, ns.prefix, ns.attribute)
				root := xsd.Bytes("schema.xsd", []byte(principal))
				dependency := xsd.Bytes("attributes.xsd", []byte(declarations))
				var sources []xsd.SchemaSource
				calls := 0
				switch assembly {
				case "resolver":
					root = root.WithResolver(xsd.ResolverFunc(func(base, reference string) (xsd.SchemaSource, error) {
						calls++
						if base != "schema.xsd" || reference != location {
							t.Fatalf("ResolveSchema(%q, %q), want schema.xsd and %s", base, reference, location)
						}
						return dependency, nil
					}))
					sources = []xsd.SchemaSource{root}
				case "explicit roots":
					sources = []xsd.SchemaSource{root, dependency}
				case "reversed roots":
					sources = []xsd.SchemaSource{dependency, root}
				}
				engine, err := xsd.Compile(sources...)
				if err != nil {
					t.Fatal(err)
				}
				if assembly == "resolver" && calls != 1 {
					t.Fatalf("resolver calls = %d, want 1", calls)
				}
				for _, instance := range []struct {
					value string
					valid bool
				}{
					{ns.valid, true},
					{ns.invalid, false},
					{" " + ns.valid + " ", true},
				} {
					doc := fmt.Sprintf(`<root xmlns:%s="%s" %s:%s="%s"/>`, ns.prefix, ns.uri, ns.prefix, ns.attribute, instance.value)
					if err := engine.Validate(strings.NewReader(doc)); (err == nil) != instance.valid {
						t.Errorf("Validate(%s) = %v, want valid=%v", doc, err, instance.valid)
					}
				}
			})
		}
	}
}

func TestNamespaceAttributeResolverFailure(t *testing.T) {
	for _, namespace := range []string{"http://www.w3.org/XML/1998/namespace", "http://www.w3.org/1999/xlink"} {
		t.Run(namespace, func(t *testing.T) {
			failure := errors.New("catalog unavailable")
			principal := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import namespace="%s" schemaLocation="attributes.xsd"/>
</xs:schema>`, namespace)
			root := xsd.Bytes("schema.xsd", []byte(principal)).WithResolver(xsd.ResolverFunc(func(_, _ string) (xsd.SchemaSource, error) {
				return xsd.SchemaSource{}, failure
			}))
			if _, err := xsd.Compile(root); !errors.Is(err, failure) {
				t.Fatalf("Compile() = %v, want resolver failure", err)
			}
		})
	}
}

func TestXMLAttributeReferenceRequiresImport(t *testing.T) {
	principal := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:attribute ref="xml:lang"/></xs:complexType></xs:element>
</xs:schema>`
	declarations := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.w3.org/XML/1998/namespace">
  <xs:attribute name="lang" type="xs:language"/>
</xs:schema>`
	if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(principal)), xsd.Bytes("xml.xsd", []byte(declarations))); err == nil {
		t.Fatal("Compile() accepted a foreign attribute reference without an import")
	}
}
