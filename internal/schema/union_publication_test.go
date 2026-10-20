package schema

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublicationRejectsUnionFacetProjectionDrift(t *testing.T) {
	t.Parallel()
	const document = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="Plain"><xs:union memberTypes="xs:integer"/></xs:simpleType>
	<xs:simpleType name="Restricted"><xs:restriction base="Plain"><xs:enumeration value="1"/></xs:restriction></xs:simpleType>
	<xs:element name="root"><xs:complexType/></xs:element>
	</xs:schema>`
	for _, name := range []string{"Plain", "Restricted"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, _ := newEmptyComplexTypePublicationCompiler(t, document)
			if err := c.compileContentModels(); err != nil {
				t.Fatal(err)
			}
			q, _ := c.rt.Names.LookupQName("", name)
			id, ok := c.rt.GlobalTypes[q].Simple()
			if !ok {
				t.Fatal("missing simple type")
			}
			c.rt.SimpleTypes[id].unionHasFacets = !c.rt.SimpleTypes[id].unionHasFacets
			published, err := c.publishSchema()
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("published a corrupted union facet projection")
			}
			c.rt.SimpleTypes[id].unionHasFacets = !c.rt.SimpleTypes[id].unionHasFacets
			if published, err := c.publishSchema(); err != nil || published == nil {
				t.Fatalf("publication after restoring projection = %v, %v", published, err)
			}
		})
	}
}
