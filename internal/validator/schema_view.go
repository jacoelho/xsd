package validator

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

type schemaView interface {
	Element(qname types.QName) *grammar.CompiledElement
	LocalElement(qname types.QName) *grammar.CompiledElement
	Type(qname types.QName) *grammar.CompiledType
	Attribute(qname types.QName) *grammar.CompiledAttribute
	Notation(qname types.QName) *types.NotationDecl
	SubstitutionGroup(head types.QName) []*grammar.CompiledElement
	SubstitutionGroupHead(member types.QName) *grammar.CompiledElement
	ElementsWithConstraints() []*grammar.CompiledElement
}

type baseSchemaView struct {
	schema *grammar.CompiledSchema
}

func newBaseSchemaView(schema *grammar.CompiledSchema) *baseSchemaView {
	return &baseSchemaView{schema: schema}
}

func (v *baseSchemaView) Element(qname types.QName) *grammar.CompiledElement {
	if v.schema == nil {
		return nil
	}
	return v.schema.Elements[qname]
}

func (v *baseSchemaView) LocalElement(qname types.QName) *grammar.CompiledElement {
	if v.schema == nil {
		return nil
	}
	return v.schema.LocalElements[qname]
}

func (v *baseSchemaView) Type(qname types.QName) *grammar.CompiledType {
	if v.schema == nil {
		return nil
	}
	return v.schema.Types[qname]
}

func (v *baseSchemaView) Attribute(qname types.QName) *grammar.CompiledAttribute {
	if v.schema == nil {
		return nil
	}
	return v.schema.Attributes[qname]
}

func (v *baseSchemaView) Notation(qname types.QName) *types.NotationDecl {
	if v.schema == nil {
		return nil
	}
	return v.schema.NotationDecls[qname]
}

func (v *baseSchemaView) SubstitutionGroup(head types.QName) []*grammar.CompiledElement {
	if v.schema == nil {
		return nil
	}
	return v.schema.SubstitutionGroups[head]
}

func (v *baseSchemaView) SubstitutionGroupHead(member types.QName) *grammar.CompiledElement {
	if v.schema == nil {
		return nil
	}
	for head, subs := range v.schema.SubstitutionGroups {
		for _, sub := range subs {
			if sub.QName == member {
				return v.schema.Elements[head]
			}
		}
	}
	return nil
}

func (v *baseSchemaView) ElementsWithConstraints() []*grammar.CompiledElement {
	if v.schema == nil {
		return nil
	}
	return v.schema.ElementsWithConstraints
}

type overlaySchemaView struct {
	base                    schemaView
	elements                map[types.QName]*grammar.CompiledElement
	localElements           map[types.QName]*grammar.CompiledElement
	types                   map[types.QName]*grammar.CompiledType
	attributes              map[types.QName]*grammar.CompiledAttribute
	notationDecls           map[types.QName]*types.NotationDecl
	substitutionGroups      map[types.QName][]*grammar.CompiledElement
	elementsWithConstraints []*grammar.CompiledElement
}

func newOverlaySchemaView(base schemaView) *overlaySchemaView {
	return &overlaySchemaView{base: base}
}

func (v *overlaySchemaView) Element(qname types.QName) *grammar.CompiledElement {
	if v.elements != nil {
		if elem := v.elements[qname]; elem != nil {
			return elem
		}
	}
	return v.base.Element(qname)
}

func (v *overlaySchemaView) LocalElement(qname types.QName) *grammar.CompiledElement {
	if v.localElements != nil {
		if elem := v.localElements[qname]; elem != nil {
			return elem
		}
	}
	return v.base.LocalElement(qname)
}

func (v *overlaySchemaView) Type(qname types.QName) *grammar.CompiledType {
	if v.types != nil {
		if typ := v.types[qname]; typ != nil {
			return typ
		}
	}
	return v.base.Type(qname)
}

func (v *overlaySchemaView) Attribute(qname types.QName) *grammar.CompiledAttribute {
	if v.attributes != nil {
		if attr := v.attributes[qname]; attr != nil {
			return attr
		}
	}
	return v.base.Attribute(qname)
}

func (v *overlaySchemaView) Notation(qname types.QName) *types.NotationDecl {
	if v.notationDecls != nil {
		if decl := v.notationDecls[qname]; decl != nil {
			return decl
		}
	}
	return v.base.Notation(qname)
}

func (v *overlaySchemaView) SubstitutionGroup(head types.QName) []*grammar.CompiledElement {
	if v.substitutionGroups != nil {
		if subs, ok := v.substitutionGroups[head]; ok {
			return subs
		}
	}
	return v.base.SubstitutionGroup(head)
}

func (v *overlaySchemaView) SubstitutionGroupHead(member types.QName) *grammar.CompiledElement {
	if v.substitutionGroups != nil {
		for head, subs := range v.substitutionGroups {
			for _, sub := range subs {
				if sub.QName == member {
					return v.Element(head)
				}
			}
		}
	}
	return v.base.SubstitutionGroupHead(member)
}

func (v *overlaySchemaView) ElementsWithConstraints() []*grammar.CompiledElement {
	if v.elementsWithConstraints != nil {
		return v.elementsWithConstraints
	}
	return v.base.ElementsWithConstraints()
}
