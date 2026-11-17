package schema

import (
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// xsderrors.CodeUnsupportedXSD11 defines what XSD 1.1 vocabulary reports.
// Whether a name is vocabulary depends only on the carrying element's local
// name, never on its ancestors, siblings or attribute values.
//
// rejectXSD11Vocabulary is the first schema check of every admitted
// XSD-namespace start tag, so the first XSD 1.1 vocabulary in document order
// wins over the parent's content model and the element's own XSD 1.0 checks,
// and loses only to an XSD 1.0 error already reported at an earlier start or
// end tag. Text is judged when its parent ends or rejects a child, so text
// before XSD 1.1 vocabulary does not win over it. The abstract
// substitution-group head xs:facet, which the XSD 1.1 schema for schemas also
// declares, is not vocabulary: no XSD 1.1 schema document can contain it.

// rejectXSD11Vocabulary reports n, an XSD-namespace start tag, when its name
// or one of its unqualified attributes is XSD 1.1 vocabulary. The element name
// is checked before the attributes, which are checked in start-tag order.
func rejectXSD11Vocabulary(n *schemaSyntaxNode) error {
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
