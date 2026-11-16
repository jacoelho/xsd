package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// XSD 1.1 is an opt-out: its schema-namespace vocabulary is reported as
// unsupported.xsd_1_1 only where XSD 1.1 permits it. A use that XSD 1.1 also
// rejects keeps its XSD 1.0 diagnostic. Placement is decided from the element's
// own attributes, its parent and, for derivations, its grandparent
// (st11.html XML Representation Summaries, src-element, src-attribute,
// src-wildcard and src-identity-constraint). Sibling order and rules that need
// a resolved component, such as src-element 4.3 and src-attribute 6.3, are not
// decided here: a construct that passes these checks is unsupported even if
// XSD 1.1 would reject it for those reasons.

// schemaPlacement locates an admitted schema element start for the XSD 1.1
// placement rules. The zero value is the document root.
type schemaPlacement struct {
	parent      xml.Name
	grandparent xml.Name
	// parentIsReference reports that the parent carries ref, so XSD 1.1
	// permits it only an annotation child.
	parentIsReference bool
	// withinMisplacedXSD11 reports that an ancestor is an XSD 1.1 element
	// that XSD 1.1 does not permit there; nothing below it is an XSD 1.1 use.
	withinMisplacedXSD11 bool
}

// xsd11Use is the XSD 1.1 outcome of admitting one element start.
type xsd11Use uint8

const (
	xsd11None xsd11Use = iota
	// xsd11Misplaced marks an XSD 1.1 element that XSD 1.1 does not permit
	// at its position; compilation rejects it as an XSD 1.0 child.
	xsd11Misplaced
	// xsd11IdentityReference marks a permitted identity-constraint ref whose
	// permission still depends on the children (src-identity-constraint 4);
	// the parse frame decides it at the first child or at the end.
	xsd11IdentityReference
)

func (p schemaPlacement) root() bool {
	return p.parent.Local == ""
}

func (p schemaPlacement) topLevel() bool {
	return isXSDName(p.parent, vocab.XSDElemSchema)
}

func (p schemaPlacement) rootChild() bool {
	return p.topLevel() && p.grandparent.Local == ""
}

func (p schemaPlacement) parentIs(locals ...string) bool {
	for _, local := range locals {
		if isXSDName(p.parent, local) {
			return true
		}
	}
	return false
}

// derivationOf reports a restriction or extension parent whose own parent is
// one of contents.
func (p schemaPlacement) derivationOf(contents ...string) bool {
	return p.parentIs(vocab.XSDElemRestriction, vocab.XSDElemExtension) && p.grandparentIs(contents...)
}

func (p schemaPlacement) restrictionOf(contents ...string) bool {
	return p.parentIs(vocab.XSDElemRestriction) && p.grandparentIs(contents...)
}

func (p schemaPlacement) grandparentIs(locals ...string) bool {
	for _, local := range locals {
		if isXSDName(p.grandparent, local) {
			return true
		}
	}
	return false
}

func (p schemaPlacement) attributeUseParent() bool {
	return p.parentIs(vocab.XSDElemComplexType, vocab.XSDElemAttributeGroup) ||
		p.derivationOf(vocab.XSDElemComplexContent, vocab.XSDElemSimpleContent)
}

func isXSDName(name xml.Name, local string) bool {
	return name.Space == vocab.XSDNamespaceURI && name.Local == local
}

func isXSD11Element(local string) bool {
	switch local {
	case vocab.XSDElemAlternative, vocab.XSDElemAssert, vocab.XSDElemAssertion, vocab.XSDElemDefaultOpenContent,
		vocab.XSDElemExplicitTimezone, vocab.XSDElemOpenContent, vocab.XSDElemOverride:
		return true
	default:
		return false
	}
}

// isXSD11Attribute reports the unqualified attributes XSD 1.1 adds to an
// XSD 1.0 element.
func isXSD11Attribute(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema:
		return attr == vocab.XSDAttrDefaultAttributes || attr == vocab.XSDAttrXPathDefaultNamespace
	case vocab.XSDElemAny, vocab.XSDElemAnyAttribute:
		return attr == vocab.XSDAttrNotNamespace || attr == vocab.XSDAttrNotQName
	case vocab.XSDElemComplexType:
		return attr == vocab.XSDAttrDefaultAttributesApply
	case vocab.XSDElemSelector, vocab.XSDElemField:
		return attr == vocab.XSDAttrXPathDefaultNamespace
	case vocab.XSDElemUnique, vocab.XSDElemKey, vocab.XSDElemKeyref:
		return attr == vocab.XSDAttrRef
	case vocab.XSDElemAttribute:
		return attr == vocab.XSDAttrInheritable || attr == vocab.XSDAttrTargetNamespace
	case vocab.XSDElemElement:
		return attr == vocab.XSDAttrTargetNamespace
	default:
		return false
	}
}

func xsd11ElementPermitted(local string, p schemaPlacement) bool {
	if p.withinMisplacedXSD11 || p.parentIsReference {
		return false
	}
	switch local {
	case vocab.XSDElemAssert:
		return p.parentIs(vocab.XSDElemComplexType) || p.derivationOf(vocab.XSDElemComplexContent, vocab.XSDElemSimpleContent)
	case vocab.XSDElemAssertion, vocab.XSDElemExplicitTimezone:
		return p.restrictionOf(vocab.XSDElemSimpleType, vocab.XSDElemSimpleContent)
	case vocab.XSDElemAlternative:
		return p.parentIs(vocab.XSDElemElement)
	case vocab.XSDElemOverride, vocab.XSDElemDefaultOpenContent:
		return p.rootChild()
	case vocab.XSDElemOpenContent:
		return p.parentIs(vocab.XSDElemComplexType) || p.derivationOf(vocab.XSDElemComplexContent)
	default:
		return false
	}
}

func xsd11AttributePermitted(n *schemaSyntaxNode, attr string, p schemaPlacement) bool {
	if p.withinMisplacedXSD11 || p.parentIsReference {
		return false
	}
	has := func(local string) bool {
		_, ok := n.attr(local)
		return ok
	}
	switch n.Name.Local {
	case vocab.XSDElemSchema:
		return p.root()
	case vocab.XSDElemAny:
		return p.parentIs(vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll) && wildcardNamespaceExclusive(has)
	case vocab.XSDElemAnyAttribute:
		return p.attributeUseParent() && wildcardNamespaceExclusive(has)
	case vocab.XSDElemComplexType:
		return p.parentIs(vocab.XSDElemElement) != has(vocab.XSDAttrName) && (p.rootChild() || p.parentIs(vocab.XSDElemElement))
	case vocab.XSDElemSelector, vocab.XSDElemField:
		return p.parentIs(vocab.XSDElemUnique, vocab.XSDElemKey, vocab.XSDElemKeyref) && p.grandparentIs(vocab.XSDElemElement)
	case vocab.XSDElemUnique, vocab.XSDElemKey, vocab.XSDElemKeyref:
		return p.parentIs(vocab.XSDElemElement) && !has(vocab.XSDAttrName) && !has(vocab.XSDAttrRefer)
	case vocab.XSDElemAttribute, vocab.XSDElemElement:
		return xsd11DeclarationAttributePermitted(n, attr, p)
	default:
		return false
	}
}

// xsd11DeclarationAttributePermitted applies the start-tag conditions of st11
// src-element 1, 2.1 and 4.1-4.2 and src-attribute 1-3 and 5-6.2. The
// conditions on type and anonymous type children (src-element 3,
// src-attribute 3.2 and 4) are not decided here.
func xsd11DeclarationAttributePermitted(n *schemaSyntaxNode, attr string, p schemaPlacement) bool {
	has := func(local string) bool {
		_, ok := n.attr(local)
		return ok
	}
	if !declarationValueConstraintPermitted(n) {
		return false
	}
	if n.Name.Local == vocab.XSDElemElement {
		return p.parentIs(vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll) && localDeclarationNamed(has)
	}
	if attr == vocab.XSDAttrTargetNamespace {
		return p.attributeUseParent() && localDeclarationNamed(has)
	}
	if p.rootChild() {
		return has(vocab.XSDAttrName)
	}
	if !p.attributeUseParent() || has(vocab.XSDAttrName) == has(vocab.XSDAttrRef) {
		return false
	}
	return !has(vocab.XSDAttrRef) || !has(vocab.XSDAttrForm) && !has(vocab.XSDAttrType)
}

// declarationValueConstraintPermitted applies st11 src-element 1 and
// src-attribute 1, 2 and 5.
func declarationValueConstraintPermitted(n *schemaSyntaxNode) bool {
	_, hasDefault := n.attr(vocab.XSDAttrDefault)
	_, hasFixed := n.attr(vocab.XSDAttrFixed)
	if hasDefault && hasFixed {
		return false
	}
	use, hasUse := n.attr(vocab.XSDAttrUse)
	if !hasUse {
		return true
	}
	use = lex.CollapseXMLWhitespace(use)
	return (!hasDefault || use == attributeUseOptionalLexical) && (!hasFixed || use != attributeUseProhibitedLexical)
}

// wildcardNamespaceExclusive reports st11 src-wildcard: namespace and
// notNamespace are not both present.
func wildcardNamespaceExclusive(has func(string) bool) bool {
	return !has(vocab.XSDAttrNamespace) || !has(vocab.XSDAttrNotNamespace)
}

// localDeclarationNamed reports the src-element 4.1-4.2 and src-attribute
// 6.1-6.2 conditions for a local targetNamespace.
func localDeclarationNamed(has func(string) bool) bool {
	return has(vocab.XSDAttrName) && !has(vocab.XSDAttrRef) && !has(vocab.XSDAttrForm)
}

func unsupportedXSD11Element(n *schemaSyntaxNode) error {
	return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 feature "+n.Name.Local+" is not supported")
}

func unsupportedXSD11Attribute(n *schemaSyntaxNode, attr string) error {
	return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 attribute "+attr+" on xs:"+n.Name.Local+" is not supported")
}

// admitIdentityReferenceChild decides a pending identity-constraint ref at
// its next child: XSD 1.1 permits only one leading annotation, so any other
// child makes the ref an XSD 1.0 attribute error on the identity constraint.
func admitIdentityReferenceChild(identity *typedSchemaParseFrame, child xml.Name) error {
	if isXSDName(child, vocab.XSDElemAnnotation) && len(identity.node.children) == 0 {
		return nil
	}
	return schemaAttributeNotAllowed(&identity.syntax, vocab.XSDAttrRef)
}

// completeIdentityReference decides a pending identity-constraint ref at its
// end, before the XSD 1.0 name and selector requirements are applied.
func completeIdentityReference(identity *typedSchemaParseFrame) error {
	if identity.node.hasNonWhitespaceText {
		return schemaAttributeNotAllowed(&identity.syntax, vocab.XSDAttrRef)
	}
	return unsupportedXSD11Attribute(&identity.syntax, vocab.XSDAttrRef)
}
