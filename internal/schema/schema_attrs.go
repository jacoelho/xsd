package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// nameReferenceSource keeps the name and reference attributes of a declaration
// together for source admission.
type nameReferenceSource struct {
	Name      lexicalAttribute
	Reference lexicalAttribute
}

// validateLocalElementSource validates that a local xs:element has a name or
// reference source.
func validateLocalElementSource(source nameReferenceSource) error {
	if !source.Name.Present && !source.Reference.Present {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "local element missing name or ref")
	}
	return nil
}

// validateAttributeUseSource validates that an xs:attribute use has a name or
// reference source.
func validateAttributeUseSource(source nameReferenceSource) error {
	if !source.Name.Present && !source.Reference.Present {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "attribute missing name or ref")
	}
	return nil
}

// validateAttributeGroupUseSource validates that an xs:attributeGroup use has
// a ref source.
func validateAttributeGroupUseSource(reference lexicalAttribute) error {
	if !reference.Present {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "attributeGroup use missing ref")
	}
	return nil
}

// validateGroupUseSource validates that an xs:group use has a ref source.
func validateGroupUseSource(reference lexicalAttribute) error {
	if !reference.Present {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "group use missing ref")
	}
	return nil
}

// schemaElementPlacement is where an XSD element appears, as far as the
// unqualified attributes it admits depend on it.
type schemaElementPlacement uint8

const (
	schemaPlacementOther schemaElementPlacement = iota
	// schemaPlacementTopLevel is a child of xs:schema.
	schemaPlacementTopLevel
	// schemaPlacementReference is a reference; see isSchemaReference.
	schemaPlacementReference
)

// schemaElementPlacementOf returns the placement of n, an XSD element whose
// parent is named parent.
func schemaElementPlacementOf(n *schemaSyntaxNode, parent xml.Name) schemaElementPlacement {
	switch {
	case isXSDName(parent, vocab.XSDElemSchema):
		return schemaPlacementTopLevel
	case isSchemaReference(n, parent):
		return schemaPlacementReference
	default:
		return schemaPlacementOther
	}
}

// isSchemaReference reports whether start, an XSD element whose parent is
// named parent, is a reference rather than a declaration or definition: an
// xs:group or xs:attributeGroup outside xs:schema, or a local xs:element or
// xs:attribute carrying ref (src-element.2.2, src-attribute.3.2). A reference
// has (annotation?) content and admits only the attributes
// referenceAttributeAllowed lists.
func isSchemaReference(start *schemaSyntaxNode, parent xml.Name) bool {
	if isXSDName(parent, vocab.XSDElemSchema) {
		return false
	}
	switch start.Name.Local {
	case vocab.XSDElemGroup, vocab.XSDElemAttributeGroup:
		return true
	case vocab.XSDElemElement, vocab.XSDElemAttribute:
		_, hasRef := start.attr(vocab.XSDAttrRef)
		return hasRef
	default:
		return false
	}
}

// schemaAttributeVocabulary classifies an unqualified attribute of an XSD
// element.
type schemaAttributeVocabulary uint8

const (
	schemaAttributeUnknown schemaAttributeVocabulary = iota
	schemaAttributeXSD10
	// schemaAttributeLocalOnly is an XSD 1.0 attribute that a top-level
	// declaration or definition cannot carry.
	schemaAttributeLocalOnly
	// schemaAttributeDeclarationOnly is an XSD 1.0 attribute of the element
	// that a reference cannot carry.
	schemaAttributeDeclarationOnly
)

// schemaAttributeVocabularyOf is the single owner of which unqualified XSD 1.0
// attributes each XSD element admits in each placement. XSD 1.1 attributes
// never reach it; see rejectXSD11Vocabulary. Elements outside the XSD 1.0
// vocabulary admit every attribute; compilation rejects them.
func schemaAttributeVocabularyOf(element, attr string, placement schemaElementPlacement) schemaAttributeVocabulary {
	switch {
	case placement == schemaPlacementTopLevel && topLevelAttributeForbidden(element, attr):
		return schemaAttributeLocalOnly
	case !schemaElementAttributeAllowed(element, attr):
		return schemaAttributeUnknown
	case placement == schemaPlacementReference && !referenceAttributeAllowed(element, attr):
		return schemaAttributeDeclarationOnly
	default:
		return schemaAttributeXSD10
	}
}

func topLevelAttributeForbidden(element, attr string) bool {
	switch element {
	case vocab.XSDElemAttributeGroup:
		return attr == vocab.XSDAttrRef
	case vocab.XSDElemGroup:
		return attr == vocab.XSDAttrRef || attr == vocab.XSDAttrMinOccurs || attr == vocab.XSDAttrMaxOccurs
	case vocab.XSDElemAttribute:
		return attr == vocab.XSDAttrRef || attr == vocab.XSDAttrForm || attr == vocab.XSDAttrUse
	case vocab.XSDElemElement:
		return attr == vocab.XSDAttrRef || attr == vocab.XSDAttrForm || attr == vocab.XSDAttrMinOccurs || attr == vocab.XSDAttrMaxOccurs
	default:
		return false
	}
}

func referenceAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemElement, vocab.XSDElemGroup:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrRef, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
			return true
		}
	case vocab.XSDElemAttribute:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrRef, vocab.XSDAttrUse, vocab.XSDAttrDefault, vocab.XSDAttrFixed:
			return true
		}
	case vocab.XSDElemAttributeGroup:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrRef
	}
	return false
}

// schemaReferenceLabel names a reference element, one of those
// isSchemaReference admits, in diagnostics.
func schemaReferenceLabel(element string) string {
	switch element {
	case vocab.XSDElemElement:
		return "element ref"
	case vocab.XSDElemAttribute:
		return "attribute ref"
	case vocab.XSDElemAttributeGroup:
		return "attributeGroup use"
	default:
		return "group"
	}
}

func schemaElementAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema, vocab.XSDElemInclude, vocab.XSDElemImport, vocab.XSDElemAppinfo, vocab.XSDElemDocumentation:
		return schemaDocumentAttributeAllowed(element, attr)
	case vocab.XSDElemSimpleType, vocab.XSDElemRestriction, vocab.XSDElemExtension, vocab.XSDElemList, vocab.XSDElemUnion:
		return simpleDerivationAttributeAllowed(element, attr)
	case vocab.XSDElemComplexType, vocab.XSDElemAnnotation, vocab.XSDElemSimpleContent, vocab.XSDElemComplexContent:
		return complexDerivationAttributeAllowed(element, attr)
	case vocab.XSDElemGroup, vocab.XSDElemAll, vocab.XSDElemChoice, vocab.XSDElemSequence:
		return modelGroupAttributeAllowed(element, attr)
	case vocab.XSDElemElement:
		return isElementAttribute(attr)
	case vocab.XSDElemAttribute:
		return isAttributeAttribute(attr)
	case vocab.XSDElemAttributeGroup:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrName || attr == vocab.XSDAttrRef
	case vocab.XSDElemAny:
		return isAnyParticleAttribute(attr)
	case vocab.XSDElemAnyAttribute:
		return isAnyAttributeAttribute(attr)
	case vocab.XSDElemUnique, vocab.XSDElemKey:
		return isIdentityAttribute(attr)
	case vocab.XSDElemKeyref:
		return isKeyrefAttribute(attr)
	case vocab.XSDElemSelector, vocab.XSDElemField:
		return isIdentityXPathAttribute(attr)
	case vocab.XSDElemNotation:
		return isNotationAttribute(attr)
	default:
		if _, ok := facetMaskForLocal(element); ok {
			return facetAttributeAllowed(element, attr)
		}
		return true
	}
}

func schemaAnyURIAttribute(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema:
		return attr == vocab.XSDAttrTargetNamespace
	case vocab.XSDElemInclude:
		return attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemImport:
		return attr == vocab.XSDAttrNamespace || attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemAppinfo, vocab.XSDElemDocumentation:
		return attr == vocab.XSDAttrSource
	case vocab.XSDElemNotation:
		return attr == vocab.XSDAttrSystem
	default:
		return false
	}
}

func schemaDocumentAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrTargetNamespace, vocab.XSDAttrVersion, vocab.XSDAttrFinalDefault, vocab.XSDAttrBlockDefault, vocab.XSDAttrAttributeFormDefault, vocab.XSDAttrElementFormDefault:
			return true
		}
	case vocab.XSDElemInclude:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemImport:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrNamespace || attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemAppinfo, vocab.XSDElemDocumentation:
		return attr == vocab.XSDAttrSource
	}
	return false
}

func simpleDerivationAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemSimpleType:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrName || attr == vocab.XSDAttrFinal
	case vocab.XSDElemRestriction, vocab.XSDElemExtension:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrBase
	case vocab.XSDElemList:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrItemType
	case vocab.XSDElemUnion:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrMemberTypes
	}
	return false
}

func complexDerivationAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemComplexType:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrMixed, vocab.XSDAttrAbstract, vocab.XSDAttrBlock, vocab.XSDAttrFinal:
			return true
		}
	case vocab.XSDElemAnnotation, vocab.XSDElemSimpleContent:
		return attr == vocab.XSDAttrID
	case vocab.XSDElemComplexContent:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrMixed
	}
	return false
}

func modelGroupAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemGroup:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRef, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
			return true
		}
	case vocab.XSDElemAll, vocab.XSDElemChoice, vocab.XSDElemSequence:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrMinOccurs || attr == vocab.XSDAttrMaxOccurs
	}
	return false
}

func facetAttributeAllowed(element, attr string) bool {
	if attr == vocab.XSDAttrID || attr == vocab.XSDAttrValue {
		return true
	}
	if attr != vocab.XSDAttrFixed {
		return false
	}
	return element != vocab.XSDFacetPattern && element != vocab.XSDFacetEnumeration
}

func isElementAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRef, vocab.XSDAttrType, vocab.XSDAttrSubstitutionGroup,
		vocab.XSDAttrNillable, vocab.XSDAttrDefault, vocab.XSDAttrFixed, vocab.XSDAttrForm,
		vocab.XSDAttrBlock, vocab.XSDAttrFinal, vocab.XSDAttrAbstract, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
		return true
	default:
		return false
	}
}

func isAttributeAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRef, vocab.XSDAttrType, vocab.XSDAttrUse, vocab.XSDAttrDefault, vocab.XSDAttrFixed, vocab.XSDAttrForm:
		return true
	default:
		return false
	}
}

func isAnyParticleAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrNamespace, vocab.XSDAttrProcessContents, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
		return true
	default:
		return false
	}
}

func isAnyAttributeAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrNamespace, vocab.XSDAttrProcessContents:
		return true
	default:
		return false
	}
}

func isIdentityAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName:
		return true
	default:
		return false
	}
}

func isKeyrefAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRefer:
		return true
	default:
		return false
	}
}

func isIdentityXPathAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrXPath:
		return true
	default:
		return false
	}
}

func isNotationAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrPublic, vocab.XSDAttrSystem:
		return true
	default:
		return false
	}
}
