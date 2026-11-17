package schema

import (
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// Unsupported vocabulary is the opt-out XSD-namespace syntax the library
// does not implement: xs:redefine, reported as
// xsderrors.CodeUnsupportedRedefine, and XSD 1.1 vocabulary, reported as
// xsderrors.CodeUnsupportedXSD11. Whether a name is unsupported vocabulary
// depends only on the carrying element's local name, never on its ancestors,
// siblings or attribute values, so it is reported wherever it appears and
// whatever its placement or values.
//
// The root element must be xs:schema before its vocabulary is judged, so a
// root of any other name is a schema.root error whatever it carries.
// Otherwise rejectUnsupportedVocabulary is the first schema check of every
// admitted XSD-namespace start tag, so the first unsupported vocabulary in
// document order wins over the parent's content model and the element's own
// XSD 1.0 checks, and loses only to an XSD 1.0 error already reported at an
// earlier start or end tag. Content of xs:appinfo and xs:documentation is
// never schema grammar, so nothing in it is vocabulary. Text is judged when
// its parent ends or rejects a child, so text before unsupported vocabulary
// does not win over it. The abstract substitution-group head xs:facet, which
// the XSD 1.1 schema for schemas also declares, is not vocabulary: no XSD 1.1
// schema document can contain it.

// rejectUnsupportedVocabulary reports n, an XSD-namespace start tag, when its
// name is unsupported vocabulary or one of its unqualified attributes is XSD
// 1.1 vocabulary. The element name is checked before the attributes, which
// are checked in start-tag order.
func rejectUnsupportedVocabulary(n *schemaSyntaxNode) error {
	if n.Name.Local == vocab.XSDElemRedefine {
		return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedRedefine, "xs:redefine is not supported")
	}
	if isXSD11Element(n.Name.Local) {
		return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 feature "+n.Name.Local+" is not supported")
	}
	for _, attr := range n.Attrs {
		if attr.Name.Space != "" || xmlstream.IsExpandedNamespaceName(attr.Name) {
			continue
		}
		if isXSD11Attribute(n.Name.Local, attr.Name.Local) {
			return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 attribute "+attr.Name.Local+" on xs:"+n.Name.Local+" is not supported")
		}
	}
	return nil
}

// isXSD11Element reports the XSD-namespace elements that the XSD 1.1 schema
// for schemas adds over XSD 1.0.
func isXSD11Element(local string) bool {
	switch local {
	case vocab.XSDElemAlternative, vocab.XSDElemAssert, vocab.XSDElemAssertion, vocab.XSDElemDefaultOpenContent,
		vocab.XSDElemExplicitTimezone, vocab.XSDElemOpenContent, vocab.XSDElemOverride:
		return true
	default:
		return false
	}
}

// isXSD11Attribute reports the unqualified attributes that the XSD 1.1 schema
// for schemas adds to an XSD 1.0 element. Attributes of XSD 1.1 elements are
// not listed: the element itself is already vocabulary.
func isXSD11Attribute(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema:
		return attr == vocab.XSDAttrDefaultAttributes || attr == vocab.XSDAttrXPathDefaultNamespace
	case vocab.XSDElemComplexType:
		return attr == vocab.XSDAttrDefaultAttributesApply
	case vocab.XSDElemElement:
		return attr == vocab.XSDAttrTargetNamespace
	case vocab.XSDElemAttribute:
		return attr == vocab.XSDAttrTargetNamespace || attr == vocab.XSDAttrInheritable
	case vocab.XSDElemAny, vocab.XSDElemAnyAttribute:
		return attr == vocab.XSDAttrNotNamespace || attr == vocab.XSDAttrNotQName
	case vocab.XSDElemSelector, vocab.XSDElemField:
		return attr == vocab.XSDAttrXPathDefaultNamespace
	case vocab.XSDElemUnique, vocab.XSDElemKey, vocab.XSDElemKeyref:
		return attr == vocab.XSDAttrRef
	default:
		return false
	}
}
