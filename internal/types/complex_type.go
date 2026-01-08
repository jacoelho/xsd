package types

import "fmt"

// ComplexType represents a complex type definition
type ComplexType struct {
	QName        QName
	content      Content
	attributes   []*AttributeDecl
	AttrGroups   []QName
	anyAttribute *AnyAttribute
	mixed        bool
	Abstract     bool
	Final        DerivationSet
	Block        DerivationSet
	// Resolved base type (can be set in struct literal or assigned directly)
	ResolvedBase Type
	// Explicit derivation method (extension or restriction)
	DerivationMethod DerivationMethod
	// targetNamespace of the schema where this type was originally declared
	SourceNamespace NamespaceURI
}

// NewComplexType creates a new complex type with the provided name and namespace.
func NewComplexType(name QName, sourceNamespace NamespaceURI) *ComplexType {
	return &ComplexType{
		QName:           name,
		SourceNamespace: sourceNamespace,
	}
}

// NewComplexTypeFromParsed validates a parsed complex type and returns it if valid.
func NewComplexTypeFromParsed(ct *ComplexType) (*ComplexType, error) {
	if ct == nil {
		return nil, fmt.Errorf("complexType is nil")
	}
	content := ct.Content()
	switch typed := content.(type) {
	case *SimpleContent:
		if typed.Extension != nil && typed.Restriction != nil {
			return nil, fmt.Errorf("simpleContent cannot have both extension and restriction")
		}
		if (typed.Extension != nil || typed.Restriction != nil) && typed.BaseTypeQName().IsZero() {
			return nil, fmt.Errorf("simpleContent must declare a base type")
		}
	case *ComplexContent:
		if typed.Extension != nil && typed.Restriction != nil {
			return nil, fmt.Errorf("complexContent cannot have both extension and restriction")
		}
		if (typed.Extension != nil || typed.Restriction != nil) && typed.BaseTypeQName().IsZero() {
			return nil, fmt.Errorf("complexContent must declare a base type")
		}
	}
	return ct, nil
}

// Name returns the QName of the complex type.
func (c *ComplexType) Name() QName {
	return c.QName
}

// ComponentName returns the QName of this component.
// Implements SchemaComponent interface.
func (c *ComplexType) ComponentName() QName {
	return c.QName
}

// DeclaredNamespace returns the targetNamespace where this component was declared.
// Implements SchemaComponent interface.
func (c *ComplexType) DeclaredNamespace() NamespaceURI {
	return c.SourceNamespace
}

// Copy creates a copy of the complex type with remapped QNames.
func (c *ComplexType) Copy(opts CopyOptions) *ComplexType {
	clone := *c
	clone.QName = opts.RemapQName(c.QName)
	clone.SourceNamespace = opts.SourceNamespace
	clone.attributes = copyAttributeDecls(c.attributes, opts)
	clone.AttrGroups = copyQNameSlice(c.AttrGroups, opts.RemapQName)
	clone.anyAttribute = copyAnyAttribute(c.anyAttribute)
	// remap base type references inside content (simple/complex content).
	if content := c.Content(); content != nil {
		clone.SetContent(content.Copy(opts))
	}
	return &clone
}

// IsBuiltin reports whether the complex type is built-in.
func (c *ComplexType) IsBuiltin() bool {
	return false
}

// BaseType returns the base type for this complex type
// If ResolvedBase is nil, returns anyType (the base of all types)
func (c *ComplexType) BaseType() Type {
	if c.ResolvedBase == nil {
		return GetBuiltin(TypeNameAnyType)
	}
	return c.ResolvedBase
}

// ResolvedBaseType returns the resolved base type, or nil if at root.
// Implements DerivedType interface.
func (c *ComplexType) ResolvedBaseType() Type {
	return c.ResolvedBase
}

// PrimitiveType returns the primitive type for this complex type
// Complex types don't have primitive types, so this always returns nil
func (c *ComplexType) PrimitiveType() Type {
	return nil
}

// FundamentalFacets returns the fundamental facets for this complex type
// Complex types don't have fundamental facets, so this always returns nil
func (c *ComplexType) FundamentalFacets() *FundamentalFacets {
	return nil
}

// WhiteSpace returns the whitespace normalization for this complex type.
// Complex types do not define whiteSpace, so this returns WhiteSpacePreserve.
func (c *ComplexType) WhiteSpace() WhiteSpace {
	return WhiteSpacePreserve
}

// Content returns the content model (implements ComplexTypeDefinition)
func (c *ComplexType) Content() Content {
	return c.content
}

// SetContent sets the content model
func (c *ComplexType) SetContent(content Content) {
	c.content = content
}

// Attributes returns the attribute declarations (implements ComplexTypeDefinition)
func (c *ComplexType) Attributes() []*AttributeDecl {
	return c.attributes
}

// SetAttributes sets the attribute declarations
func (c *ComplexType) SetAttributes(attributes []*AttributeDecl) {
	c.attributes = attributes
}

// AnyAttribute returns the wildcard attribute if present (implements ComplexTypeDefinition)
func (c *ComplexType) AnyAttribute() *AnyAttribute {
	return c.anyAttribute
}

// SetAnyAttribute sets the wildcard attribute
func (c *ComplexType) SetAnyAttribute(anyAttr *AnyAttribute) {
	c.anyAttribute = anyAttr
}

// Mixed returns true if this type allows mixed content (implements ComplexTypeDefinition)
func (c *ComplexType) Mixed() bool {
	return c.mixed
}

// SetMixed sets whether this type allows mixed content
func (c *ComplexType) SetMixed(mixed bool) {
	c.mixed = mixed
}

// IsExtension returns true if this complex type is derived by extension
func (c *ComplexType) IsExtension() bool {
	return c.DerivationMethod == DerivationExtension
}

// IsRestriction returns true if this complex type is derived by restriction
func (c *ComplexType) IsRestriction() bool {
	return c.DerivationMethod == DerivationRestriction
}

// IsDerived returns true if this complex type is derived (has a derivation method)
func (c *ComplexType) IsDerived() bool {
	return c.DerivationMethod != 0
}
