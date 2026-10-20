package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

func TestValidateBuiltinDeclarationCounts(t *testing.T) {
	t.Parallel()

	valid := BuiltinDeclarationCounts{
		SimpleTypes:      BuiltinSimpleTypeCount(),
		Attributes:       4,
		ComplexTypes:     BuiltinComplexTypeCount(),
		Wildcards:        1,
		AttributeUseSets: 1,
		Models:           1,
	}
	if err := ValidateBuiltinDeclarationCounts(valid); err != nil {
		t.Fatalf("ValidateBuiltinDeclarationCounts(valid) error = %v", err)
	}
	for name, mutate := range map[string]func(*BuiltinDeclarationCounts){
		"simple types":   func(c *BuiltinDeclarationCounts) { c.SimpleTypes-- },
		"attributes":     func(c *BuiltinDeclarationCounts) { c.Attributes-- },
		"complex types":  func(c *BuiltinDeclarationCounts) { c.ComplexTypes-- },
		"wildcards":      func(c *BuiltinDeclarationCounts) { c.Wildcards = 0 },
		"attribute sets": func(c *BuiltinDeclarationCounts) { c.AttributeUseSets = 0 },
		"models":         func(c *BuiltinDeclarationCounts) { c.Models = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			counts := valid
			mutate(&counts)
			if err := ValidateBuiltinDeclarationCounts(counts); err == nil {
				t.Fatal("ValidateBuiltinDeclarationCounts() unexpectedly succeeded")
			}
		})
	}
}

func TestBuiltinXSIAttributeDeclarations(t *testing.T) {
	t.Parallel()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		local string
		typ   SimpleTypeID
	}{
		{"type", value.BuiltinType(value.PrimitiveQName)},
		{"nil", value.BuiltinType(value.PrimitiveBoolean)},
		{"noNamespaceSchemaLocation", value.BuiltinType(value.PrimitiveAnyURI)},
	} {
		q, ok := c.rt.Names.LookupQName(vocab.XSINamespaceURI, tt.local)
		if !ok {
			t.Fatalf("missing xsi:%s name", tt.local)
		}
		id, ok := c.rt.GlobalAttributes[q]
		if !ok || c.rt.Attributes[id].Type != tt.typ {
			t.Errorf("xsi:%s declaration does not use type %d", tt.local, tt.typ)
		}
	}
	q, ok := c.rt.Names.LookupQName(vocab.XSINamespaceURI, "schemaLocation")
	if !ok {
		t.Fatal("missing xsi:schemaLocation name")
	}
	id, ok := c.rt.GlobalAttributes[q]
	if !ok {
		t.Fatal("missing xsi:schemaLocation declaration")
	}
	list := c.rt.SimpleTypes[c.rt.Attributes[id].Type]
	if list.Scope != DeclarationScopeNonGlobal {
		t.Fatal("schemaLocation list must be anonymous")
	}
	view, ok := c.rt.valueBuilder.TypeView(c.rt.Attributes[id].Type)
	if !ok || view.Variety != value.List || view.Base != value.AnySimpleTypeID ||
		view.ListItem != value.BuiltinType(value.PrimitiveAnyURI) || view.Whitespace != value.WhitespaceCollapse {
		t.Fatalf("schemaLocation type = %+v, want collapsed list of anyURI", view)
	}
}

func TestPublicationRejectsIntrinsicXSIAttributeDrift(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*testing.T, *schemaBuild, QName, AttributeID){
		"missing binding": func(_ *testing.T, b *schemaBuild, q QName, _ AttributeID) { delete(b.GlobalAttributes, q) },
		"wrong type": func(_ *testing.T, b *schemaBuild, _ QName, id AttributeID) {
			b.Attributes[id].Type = value.BuiltinType(value.PrimitiveString)
		},
		"consistent rename": func(t *testing.T, b *schemaBuild, q QName, id AttributeID) {
			t.Helper()
			renamed, err := b.internQName(vocab.XSINamespaceURI, "other")
			if err != nil {
				t.Fatal(err)
			}
			delete(b.GlobalAttributes, q)
			b.Attributes[id].Name = renamed
			b.GlobalAttributes[renamed] = id
		},
		"extra XSI declaration": func(t *testing.T, b *schemaBuild, _ QName, _ AttributeID) {
			t.Helper()
			q, err := b.internQName(vocab.XSINamespaceURI, "other")
			if err != nil {
				t.Fatal(err)
			}
			id, err := NextAttributeID(len(b.Attributes))
			if err != nil {
				t.Fatal(err)
			}
			b.GlobalAttributes[q] = id
			b.Attributes = append(b.Attributes, AttributeDecl{Name: q, Type: value.BuiltinType(value.PrimitiveBoolean)})
		},
		"list item": func(_ *testing.T, b *schemaBuild, _ QName, _ AttributeID) {
			b.SimpleTypes[builtinSchemaLocationTypeID].ValueSpec.ListItem = value.BuiltinType(value.PrimitiveString)
		},
		"list facet": func(_ *testing.T, b *schemaBuild, _ QName, _ AttributeID) {
			b.SimpleTypes[builtinSchemaLocationTypeID].ValueSpec.Facets.Length = value.CardinalityFacet{Present: true, Value: 2}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, _ := newEmptyComplexTypePublicationCompiler(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType/></xs:element></xs:schema>`)
			q, _ := c.rt.Names.LookupQName(vocab.XSINamespaceURI, "nil")
			mutate(t, &c.rt, q, c.rt.GlobalAttributes[q])
			if err := c.compileContentModels(); err != nil {
				t.Fatal(err)
			}
			if published, err := c.publishSchema(); err == nil || published != nil {
				t.Fatalf("publishSchema() = %v, %v, want rejection", published, err)
			}
		})
	}
}

func TestIntrinsicTypesDoNotConsumeSourceNodeAllowance(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		limit    int
		document string
	}{
		{1, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`},
		{3, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="T"><xs:restriction base="xs:string"/></xs:simpleType></xs:schema>`},
	} {
		if _, err := Compile(Options{MaxSchemaInstantiatedNodes: tt.limit}, []source.Source{source.Bytes("schema.xsd", []byte(tt.document))}); err != nil {
			t.Fatalf("Compile(MaxSchemaInstantiatedNodes=%d) error = %v", tt.limit, err)
		}
	}
}
