package grammar

import "github.com/jacoelho/xsd/internal/types"

// TypeKind classifies compiled types.
type TypeKind int

const (
	// TypeKindBuiltin indicates a built-in XSD type.
	TypeKindBuiltin TypeKind = iota
	// TypeKindSimple indicates a user-defined simple type.
	TypeKindSimple
	// TypeKindComplex indicates a user-defined complex type.
	TypeKindComplex
)

// CompiledType is a fully-resolved type definition.
// All base types, attributes, and content models are pre-resolved.
type CompiledType struct {
	Original              types.Type
	SimpleContentType     *CompiledType
	BaseType              *CompiledType
	AnyAttribute          *types.AnyAttribute
	ContentModel          *CompiledContentModel
	PrimitiveType         *CompiledType
	ItemType              *CompiledType
	QName                 types.QName
	IDTypeName            string
	DerivationChain       []*CompiledType
	AllAttributes         []*CompiledAttribute
	Facets                []types.Facet
	MemberTypes           []*CompiledType
	DerivationMethod      types.DerivationMethod
	Final                 types.DerivationSet
	Block                 types.DerivationSet
	Kind                  TypeKind
	Abstract              bool
	Mixed                 bool
	IsNotationType        bool
	IsQNameOrNotationType bool
}

// TextType returns the simple type used to validate text content, or nil if no text schemacheck.
// For simple types, returns self. For complex types with simpleContent, returns the base simple type.
// For mixed content, returns nil (text is unrestricted xs:string).
func (ct *CompiledType) TextType() *CompiledType {
	if ct == nil {
		return nil
	}
	switch ct.Kind {
	case TypeKindSimple, TypeKindBuiltin:
		return ct
	case TypeKindComplex:
		// mixed content has unrestricted text - no type validation
		if ct.Mixed {
			return nil
		}
		if ct.SimpleContentType != nil {
			return ct.SimpleContentType
		}
	}
	return nil
}

// HasContentModel returns true if the type has a content model for child elements.
func (ct *CompiledType) HasContentModel() bool {
	return ct != nil && ct.ContentModel != nil && !ct.ContentModel.Empty
}

// AllowsText returns true if the type allows text content.
func (ct *CompiledType) AllowsText() bool {
	if ct == nil {
		return false
	}
	return ct.TextType() != nil || ct.Mixed
}

// HasAttributes returns true if the type has attribute declarations.
func (ct *CompiledType) HasAttributes() bool {
	return ct != nil && (len(ct.AllAttributes) > 0 || ct.AnyAttribute != nil)
}
