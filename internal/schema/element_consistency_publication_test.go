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
  <xs:element name="root"><xs:complexType><xs:sequence><xs:group ref="G"/><xs:element name="a" type="S"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`, body.typ)
			c, _ := newEmptyComplexTypePublicationCompiler(t, document)
			if err := c.compileReachedContentModelsForTest(); err != nil {
				t.Fatal(err)
			}
			name, ok := c.rt.Names.LookupQName("", "S")
			if !ok {
				t.Fatal("missing named type")
			}
			typ := c.rt.GlobalTypes[name]
			setDeclaration := func(scope declarationScope, name QName) {
				if id, simple := typ.Simple(); simple {
					c.rt.SimpleTypes[id].Scope = scope
					c.rt.SimpleTypes[id].Name = name
				} else {
					id, _ := typ.Complex()
					c.rt.ComplexTypes[id].Scope = scope
					c.rt.ComplexTypes[id].Name = name
				}
			}
			setDeclaration(declarationScopeNonGlobal, noQName())
			delete(c.rt.GlobalTypes, name)
			published, err := c.publishSchema()
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil || len(c.rt.Models) == 0 {
				t.Fatal("failed EDC publication returned a schema or consumed the build")
			}
			if _, bound := c.rt.GlobalTypes[name]; bound {
				t.Fatal("failed publication restored the removed type binding")
			}
			setDeclaration(declarationScopeGlobal, name)
			c.rt.GlobalTypes[name] = typ
			if published, err := c.publishSchema(); err != nil || published == nil {
				t.Fatalf("publication retry: %v, %v", published, err)
			}
		})
	}
}
