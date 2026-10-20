package schema_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestNotationTypeUseRequiresEffectiveEnumeration(t *testing.T) {
	for _, consumer := range []struct{ name, body string }{
		{"global attribute", `<xs:attribute name="a" type="T"/>`},
		{"local attribute in unused type", `<xs:complexType name="C"><xs:attribute name="a" type="T"/></xs:complexType>`},
		{"simple content in unused type", `<xs:complexType name="C"><xs:simpleContent><xs:extension base="T"/></xs:simpleContent></xs:complexType>`},
	} {
		for _, enumerated := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enumerated=%t", consumer.name, enumerated), func(t *testing.T) {
				base := "Alias"
				declarations := `<xs:simpleType name="Alias"><xs:restriction base="xs:NOTATION"/></xs:simpleType>`
				if enumerated {
					declarations += `<xs:simpleType name="E"><xs:restriction base="Alias"><xs:enumeration value="n"/></xs:restriction></xs:simpleType>`
					base = "E"
				}
				declarations += `<xs:simpleType name="T"><xs:restriction base="` + base + `"/></xs:simpleType>`
				text := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:notation name="n" public="urn:n"/>` + declarations + consumer.body + `</xs:schema>`
				compiled, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("notation-use.xsd", []byte(text))})
				if enumerated {
					if err != nil || compiled == nil {
						t.Fatalf("Compile() = %v, %v; want enumerated inherited use admitted", compiled, err)
					}
					return
				}
				expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaFacet)
				if compiled != nil {
					t.Fatal("Compile() published a bare NOTATION use")
				}
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if !ok || diagnostic.Path() != "notation-use.xsd" || diagnostic.Line() == 0 {
					t.Fatalf("Compile() error = %v; want source path and line", err)
				}
			})
		}
	}
}

func TestPublishSchemaRejectsBareNotationUseAndAllowsRetry(t *testing.T) {
	const text = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="n" public="urn:n"/>
  <xs:simpleType name="Alias"><xs:restriction base="xs:NOTATION"/></xs:simpleType>
  <xs:simpleType name="E"><xs:restriction base="Alias"><xs:enumeration value="n"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="L"><xs:list itemType="E"/></xs:simpleType>
  <xs:simpleType name="U"><xs:union memberTypes="E xs:integer"/></xs:simpleType>
  <xs:attribute name="a" type="E"/>
  <xs:complexType name="C"><xs:simpleContent><xs:extension base="E"><xs:attribute name="local" type="E"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:element name="root" type="E"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(*testing.T, *xsdSchema.SchemaBuild) func()
	}{
		{"element", func(t *testing.T, build *xsdSchema.SchemaBuild) func() {
			t.Helper()
			id := build.GlobalElements[mustQName(t, &build.Names, "root")]
			old := build.Elements[id].Type
			build.Elements[id].Type = xsdSchema.SimpleRef(valuepkg.BuiltinType(valuepkg.PrimitiveNotation))
			return func() { build.Elements[id].Type = old }
		}},
		{"global attribute", func(t *testing.T, build *xsdSchema.SchemaBuild) func() {
			t.Helper()
			id := build.GlobalAttributes[mustQName(t, &build.Names, "a")]
			old := build.Attributes[id].Type
			build.Attributes[id].Type = valuepkg.BuiltinType(valuepkg.PrimitiveNotation)
			return func() { build.Attributes[id].Type = old }
		}},
		{"local attribute", func(t *testing.T, build *xsdSchema.SchemaBuild) func() {
			t.Helper()
			id, _ := build.GlobalTypes[mustQName(t, &build.Names, "C")].Complex()
			set := &build.AttributeUseSets[build.ComplexTypes[id].Attrs]
			old := set.Uses[0].Type
			set.Uses[0].Type = valuepkg.BuiltinType(valuepkg.PrimitiveNotation)
			return func() { set.Uses[0].Type = old }
		}},
		{"simple content", func(t *testing.T, build *xsdSchema.SchemaBuild) func() {
			t.Helper()
			id, _ := build.GlobalTypes[mustQName(t, &build.Names, "C")].Complex()
			old := build.ComplexTypes[id].TextType
			build.ComplexTypes[id].TextType = valuepkg.BuiltinType(valuepkg.PrimitiveNotation)
			return func() { build.ComplexTypes[id].TextType = old }
		}},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			build := mutableSchemaBuild(t, text)
			restore := tc.mutate(t, build)
			expected := mutableSchemaBuild(t, text)
			tc.mutate(t, expected)
			published, err := publishSchema(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("PublishSchema() published a bare NOTATION use")
			}
			retained := func(current *xsdSchema.SchemaBuild) []any {
				return []any{current.Names, current.GlobalTypes, current.GlobalElements, current.GlobalAttributes,
					current.Elements, current.Attributes, current.ComplexTypes,
					current.AttributeUseSets, current.Models, current.Notations}
			}
			if !reflect.DeepEqual(retained(build), retained(expected)) {
				t.Fatal("PublishSchema() changed rejected construction records")
			}
			for i, typ := range build.SimpleTypes {
				want := expected.SimpleTypes[i]
				if typ.ValueSpec.ListItem != want.ValueSpec.ListItem || !reflect.DeepEqual(typ.ValueSpec.Union, want.ValueSpec.Union) {
					t.Fatal("PublishSchema() changed rejected constructor edges")
				}
			}
			restore()
			if _, err := publishSchema(build); err != nil {
				t.Fatalf("PublishSchema() retry = %v", err)
			}
		})
	}
}
