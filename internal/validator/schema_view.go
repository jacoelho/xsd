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
