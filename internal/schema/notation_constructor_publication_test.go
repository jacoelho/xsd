package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublicationChecksCanonicalNotationConstructorEdges(t *testing.T) {
	for _, variety := range []value.Variety{value.List, value.Union} {
		t.Run(map[value.Variety]string{value.List: "list", value.Union: "union"}[variety], func(t *testing.T) {
			c, _ := newEmptyComplexTypePublicationCompiler(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType/></xs:element></xs:schema>`)
			name, err := c.rt.internQName("", "$injected")
			if err != nil {
				t.Fatal(err)
			}
			notation := value.BuiltinType(value.PrimitiveNotation)
			stringType := value.BuiltinType(value.PrimitiveString)
			typ := SimpleType{Name: name, ValueSpec: value.TypeSpec{
				Base: value.AnySimpleTypeID, Variety: variety, Primitive: value.PrimitiveString,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, ListItem: value.NoType,
			}}
			if variety == value.List {
				typ.ValueSpec.ListItem = notation
			} else {
				typ.ValueSpec.Union = []value.TypeID{notation, stringType}
			}
			id, err := c.addSimpleType(typ)
			if err != nil {
				t.Fatal(err)
			}
			if err = c.completeSimpleType(id, typ); err != nil {
				t.Fatal(err)
			}
			// Publication consumes the completed value program. A safe-looking
			// construction record must not hide its invalid constructor edge.
			c.rt.SimpleTypes[id].ValueSpec.ListItem = stringType
			c.rt.SimpleTypes[id].ValueSpec.Union = []value.TypeID{stringType}
			if err = c.compileContentModels(); err != nil {
				t.Fatal(err)
			}
			published, err := c.publishSchema()
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("publishSchema() published a bare NOTATION constructor edge")
			}
			if c.rt.valueBuilder == nil || len(c.rt.SimpleTypes) <= int(id) {
				t.Fatal("publishSchema() consumed the rejected build")
			}
		})
	}
}
