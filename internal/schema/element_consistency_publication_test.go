package schema

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublishSchemaRejectsRepeatedAnonymousTypeAndAllowsRetry(t *testing.T) {
	t.Parallel()
	for _, body := range []struct {
		name string
		typ  string
	}{
		{"simple", `<xs:simpleType name="S"><xs:restriction base="xs:string"/></xs:simpleType>`},
		{"complex", `<xs:complexType name="S"/>`},
	} {
		t.Run(body.name, func(t *testing.T) {
			t.Parallel()
			document := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  %s
  <xs:element name="a" type="S"/>
  <xs:group name="G"><xs:sequence><xs:element ref="a"/></xs:sequence></xs:group>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:group ref="G"/><xs:group ref="G"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`, body.typ)
			c, _ := newEmptyComplexTypePublicationCompiler(t, document)
			if err := c.compileContentModels(); err != nil {
				t.Fatal(err)
			}
			name, ok := c.rt.Names.LookupQName("", "S")
			if !ok {
				t.Fatal("missing named type")
			}
			typ := c.rt.GlobalTypes[name]
			setScope := func(scope DeclarationScope) {
				if id, simple := typ.Simple(); simple {
					c.rt.SimpleTypes[id].Scope = scope
				} else {
					id, _ := typ.Complex()
					c.rt.ComplexTypes[id].Scope = scope
				}
			}
			// Anonymous construction records still have an internal QName. Only
			// declaration provenance and its exact binding establish a type name.
			setScope(DeclarationScopeNonGlobal)
			delete(c.rt.GlobalTypes, name)
			published, err := c.publishSchema()
			expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
			if published != nil || len(c.rt.Models) == 0 {
				t.Fatal("failed EDC publication returned a schema or consumed the build")
			}
			if _, bound := c.rt.GlobalTypes[name]; bound {
				t.Fatal("failed publication restored the removed type binding")
			}
			setScope(DeclarationScopeGlobal)
			c.rt.GlobalTypes[name] = typ
			if published, err := c.publishSchema(); err != nil || published == nil {
				t.Fatalf("publication retry: %v, %v", published, err)
			}
		})
	}
}
