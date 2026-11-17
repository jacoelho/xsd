package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
)

// schemaElementPlacement is where an XSD element appears, as far as the
// unqualified attributes it admits or requires depend on it.
type schemaElementPlacement uint8

const (
	schemaPlacementOther schemaElementPlacement = iota
	// schemaPlacementTopLevel is a child of xs:schema.
	schemaPlacementTopLevel
	// schemaPlacementReference is a reference; see isSchemaReference.
	schemaPlacementReference
	// schemaPlacementLocal is an xs:element or xs:attribute declaration, or
	// an xs:complexType or xs:simpleType definition, that is neither
	// top-level nor a reference.
	schemaPlacementLocal
	// schemaPlacementNamedGroupModel is the model group of a top-level
	// xs:group. A nested xs:group is a reference, whose model group child
	// admitChildContent rejects before attributes are checked.
	schemaPlacementNamedGroupModel
	// schemaPlacementContentDerivation is an xs:restriction or xs:extension
	// child of xs:simpleContent or xs:complexContent.
	schemaPlacementContentDerivation
)

// schemaElementPlacementOf returns the placement of n, an XSD element whose
// parent is named parent. xs:redefine is rejected at its own start, so no
// child of it is classified.
func schemaElementPlacementOf(n *schemaSyntaxNode, parent xml.Name) schemaElementPlacement {
	switch {
	case isXSDName(parent, vocab.XSDElemSchema):
		return schemaPlacementTopLevel
	case isSchemaReference(n, parent):
		return schemaPlacementReference
	}
	switch n.Name.Local {
	case vocab.XSDElemElement, vocab.XSDElemAttribute, vocab.XSDElemComplexType, vocab.XSDElemSimpleType:
		if localComponentParent(n.Name.Local, parent) {
			return schemaPlacementLocal
		}
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
		if isXSDName(parent, vocab.XSDElemGroup) {
			return schemaPlacementNamedGroupModel
		}
	case vocab.XSDElemRestriction, vocab.XSDElemExtension:
		if isXSDName(parent, vocab.XSDElemSimpleContent) || isXSDName(parent, vocab.XSDElemComplexContent) {
			return schemaPlacementContentDerivation
		}
	}
	return schemaPlacementOther
}

// localComponentParent reports whether the schema for schemas admits a local
// declaration or definition named local under parent. A component under any
// other parent is misplaced, and its parent's content model reports it.
func localComponentParent(local string, parent xml.Name) bool {
	if parent.Space != vocab.XSDNamespaceURI {
		return false
	}
	switch local {
	case vocab.XSDElemElement:
		switch parent.Local {
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
			return true
		}
	case vocab.XSDElemAttribute:
		switch parent.Local {
		case vocab.XSDElemComplexType, vocab.XSDElemRestriction, vocab.XSDElemExtension, vocab.XSDElemAttributeGroup:
			return true
		}
	case vocab.XSDElemComplexType:
		return parent.Local == vocab.XSDElemElement
	case vocab.XSDElemSimpleType:
		switch parent.Local {
		case vocab.XSDElemElement, vocab.XSDElemAttribute, vocab.XSDElemRestriction, vocab.XSDElemList, vocab.XSDElemUnion:
			return true
		}
	}
	return false
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
	// schemaAttributeMisplaced is an XSD 1.0 attribute of the element that
	// its placement forbids.
	schemaAttributeMisplaced
)

// schemaAttributeVocabularyOf and requiredSchemaAttributeMissing are the
// single owner of which unqualified XSD 1.0 attributes each XSD element
// admits and requires in each placement, as the schema for schemas types of
// that placement and src-element.2.1 and src-attribute.3.1 declare them.
// Both run on the start tag, so their violations precede every child in
// document order. Unsupported vocabulary never reaches them; see
// rejectUnsupportedVocabulary. Elements outside the XSD 1.0 vocabulary never
// reach them: their parent's content rejects them first; see
// childContentViolation.
func schemaAttributeVocabularyOf(element, attr string, placement schemaElementPlacement) schemaAttributeVocabulary {
	switch {
	case !schemaElementAttributeAllowed(element, attr):
		return schemaAttributeUnknown
	case placementForbidsAttribute(element, attr, placement):
		return schemaAttributeMisplaced
	default:
		return schemaAttributeXSD10
	}
}

func placementForbidsAttribute(element, attr string, placement schemaElementPlacement) bool {
	switch placement {
	case schemaPlacementTopLevel:
		return topLevelAttributeForbidden(element, attr)
	case schemaPlacementReference:
		return !referenceAttributeAllowed(element, attr)
	case schemaPlacementLocal:
		return localAttributeForbidden(element, attr)
	case schemaPlacementNamedGroupModel:
		return attr == vocab.XSDAttrMinOccurs || attr == vocab.XSDAttrMaxOccurs
	case schemaPlacementOther, schemaPlacementContentDerivation:
	}
	return false
}

// misplacedSchemaAttributeMessage describes attr, which placement forbids on
// element. Both are XSD 1.0 vocabulary names, never unbounded schema text.
func misplacedSchemaAttributeMessage(element, attr string, placement schemaElementPlacement) string {
	switch placement {
	case schemaPlacementTopLevel:
		return "top-level " + diagnostictext.Quote(element) + " cannot have " + diagnostictext.Quote(attr)
	case schemaPlacementReference:
		return schemaReferenceLabel(element) + " cannot have attribute " + diagnostictext.Quote(attr)
	case schemaPlacementLocal:
		return "local " + element + " cannot have " + attr
	case schemaPlacementOther, schemaPlacementNamedGroupModel, schemaPlacementContentDerivation:
	}
	return "top-level model group cannot have " + attr
}

// requiredSchemaAttributeMissing returns why n, an XSD element in placement
// whose parent is named parent, lacks an attribute its placement requires,
// or "" when it lacks none. The reason names only XSD 1.0 vocabulary.
func requiredSchemaAttributeMissing(n *schemaSyntaxNode, parent xml.Name, placement schemaElementPlacement) string {
	required := requiredSchemaAttribute(n.Name.Local, placement)
	if required == "" {
		return ""
	}
	if _, ok := n.attr(required); ok {
		return ""
	}
	element := n.Name.Local
	switch placement {
	case schemaPlacementTopLevel:
		return "top-level " + diagnostictext.Quote(element) + " missing name"
	case schemaPlacementLocal:
		if element == vocab.XSDElemElement {
			return "local element missing name or ref"
		}
		return "attribute missing name or ref"
	case schemaPlacementReference:
		return element + " use missing ref"
	case schemaPlacementOther, schemaPlacementNamedGroupModel, schemaPlacementContentDerivation:
	}
	return parent.Local + " " + element + " missing base"
}

// requiredSchemaAttribute returns the attribute that element requires in
// placement, or "" when it requires none. A local declaration has no ref by
// definition of its placement, so it requires name.
func requiredSchemaAttribute(element string, placement schemaElementPlacement) string {
	switch placement {
	case schemaPlacementTopLevel:
		switch element {
		case vocab.XSDElemSimpleType, vocab.XSDElemComplexType, vocab.XSDElemAttributeGroup, vocab.XSDElemGroup, vocab.XSDElemAttribute, vocab.XSDElemElement:
			return vocab.XSDAttrName
		}
	case schemaPlacementLocal:
		if element == vocab.XSDElemElement || element == vocab.XSDElemAttribute {
			return vocab.XSDAttrName
		}
	case schemaPlacementReference:
		return vocab.XSDAttrRef
	case schemaPlacementContentDerivation:
		return vocab.XSDAttrBase
	case schemaPlacementOther, schemaPlacementNamedGroupModel:
	}
	return ""
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

func localAttributeForbidden(element, attr string) bool {
	switch element {
	case vocab.XSDElemElement:
		return attr == vocab.XSDAttrAbstract || attr == vocab.XSDAttrFinal || attr == vocab.XSDAttrSubstitutionGroup
	case vocab.XSDElemComplexType:
		return attr == vocab.XSDAttrName || attr == vocab.XSDAttrAbstract || attr == vocab.XSDAttrFinal || attr == vocab.XSDAttrBlock
	case vocab.XSDElemSimpleType:
		return attr == vocab.XSDAttrName || attr == vocab.XSDAttrFinal
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
		return false
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
