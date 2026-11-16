package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func schemaLexicalAttributeSyntax(n *schemaSyntaxNode, name string) lexicalAttribute {
	value, present := n.attr(name)
	return lexicalAttribute{Value: value, Present: present}
}

func checkXMLBaseAttributeSyntax(n *schemaSyntaxNode) error {
	value, ok := n.attrNS(vocab.XMLNamespaceURI, vocab.XMLAttrBase)
	if !ok {
		return nil
	}
	if _, err := uriref.Check(value); err != nil {
		return schemaCompileAt(n, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
	}
	return nil
}

func checkSchemaAttributesSyntax(n *schemaSyntaxNode) error {
	for _, attr := range n.Attrs {
		if xmlstream.IsExpandedNamespaceName(attr.Name) || attr.Name.Space != "" {
			continue
		}
		if err := checkSchemaAttributeSyntax(n, attr); err != nil {
			return err
		}
	}
	return nil
}

func checkSchemaAttributeSyntax(n *schemaSyntaxNode, attr schemaAttribute) error {
	if !schemaElementAttributeAllowed(n.Name.Local, attr.Name.Local) {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, diagnostictext.Quote(n.Name.Local)+" cannot have attribute "+diagnostictext.Quote(attr.Name.Local))
	}
	if !schemaAnyURIAttribute(n.Name.Local, attr.Name.Local) {
		return nil
	}
	if _, err := uriref.Check(lex.CollapseXMLWhitespace(attr.Value)); err != nil {
		code := xsderrors.CodeSchemaInvalidAttribute
		if attr.Name.Local == vocab.XSDAttrSchemaLocation {
			code = xsderrors.CodeSchemaReference
		}
		return schemaCompileAt(n, code, "invalid "+diagnostictext.Quote(attr.Name.Local)+": "+err.Error())
	}
	return nil
}

func rejectSchemaNamespaceAttributes(n *schemaSyntaxNode) error {
	for _, attr := range n.Attrs {
		if attr.Name.Space == vocab.XSDNamespaceURI {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "schema namespace attribute "+diagnostictext.Quote(attr.Name.Local)+" is not allowed")
		}
	}
	return nil
}

func checkUnsupportedXSDNodeSyntax(n *schemaSyntaxNode, parentName xml.Name) error {
	topLevel := parentName == xml.Name{Space: vocab.XSDNamespaceURI, Local: vocab.XSDElemSchema}
	switch n.Name.Local {
	case redefineChild:
		return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedRedefine, "xs:redefine is not supported")
	case notationChild:
		if !topLevel {
			return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:notation must be a top-level schema child")
		}
	case assertChild, "assertion", "alternative", "override", "openContent", "defaultOpenContent", "explicitTimezone":
		return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 feature "+n.Name.Local+" is not supported")
	}
	if err := rejectXSD11Attributes(n, xsd11Attributes[n.Name.Local]); err != nil || topLevel {
		return err
	}
	return rejectXSD11Attributes(n, xsd11LocalAttributes[n.Name.Local])
}

func rejectXSD11Attributes(n *schemaSyntaxNode, attrs []string) error {
	for _, attr := range attrs {
		if _, ok := n.attr(attr); ok {
			return unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 attribute "+attr+" on xs:"+n.Name.Local+" is not supported")
		}
	}
	return nil
}

// xsd11Attributes lists, by element, the unqualified attributes XSD 1.1 adds
// to XSD 1.0 elements; xsd11LocalAttributes adds those it allows only on local
// declarations. XSD 1.1 is an opt-out, so these are reported as unsupported
// instead of as unknown XSD 1.0 attributes.
const xsd11AttrXPathDefaultNamespace = "xpathDefaultNamespace"

var (
	xsd11Attributes = map[string][]string{
		vocab.XSDElemSchema: {"defaultAttributes", xsd11AttrXPathDefaultNamespace},
		anyChild:            {vocab.XSDAttrNotNamespace, vocab.XSDAttrNotQName},
		anyAttribute:        {vocab.XSDAttrNotNamespace, vocab.XSDAttrNotQName},
		complexTypeChild:    {"defaultAttributesApply"},
		selectorChild:       {xsd11AttrXPathDefaultNamespace},
		fieldChild:          {xsd11AttrXPathDefaultNamespace},
		uniqueChild:         {vocab.XSDAttrRef},
		keyChild:            {vocab.XSDAttrRef},
		keyrefChild:         {vocab.XSDAttrRef},
		attributeChild:      {"inheritable"},
	}
	xsd11LocalAttributes = map[string][]string{
		attributeChild: {vocab.XSDAttrTargetNamespace},
		elementChild:   {vocab.XSDAttrTargetNamespace},
	}
)

func checkSchemaNodeNamesSyntax(n, parent *schemaSyntaxNode) error {
	var parentLocal string
	var parentXSD bool
	if parent != nil {
		parentLocal = parent.Name.Local
		parentXSD = parent.Name.Space == vocab.XSDNamespaceURI
	}
	id, hasID := n.attr(vocab.XSDAttrID)
	name, hasName := n.attr(vocab.XSDAttrName)
	return withSchemaCompileLocation(n, validateSchemaNodeNames(schemaNodeNames{
		Local:       n.Name.Local,
		ParentLocal: parentLocal,
		ID:          id,
		Name:        name,
		XSD:         n.Name.Space == vocab.XSDNamespaceURI,
		ParentXSD:   parentXSD,
		HasID:       hasID,
		HasName:     hasName,
	}))
}

func checkSchemaQNamePartsSyntax(n *schemaSyntaxNode, lexical string) (qNameParts, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return qNameParts{}, withSchemaCompileLocation(n, err)
	}
	return parts, nil
}

func checkTopLevelSchemaChildSyntax(n *schemaSyntaxNode) error {
	local := n.Name.Local
	var forbidden []string
	switch local {
	case annotationChild, includeChild, importChild, redefineChild, notationChild:
		return nil
	case simpleTypeChild, complexTypeChild:
	case attributeGroup:
		forbidden = []string{vocab.XSDAttrRef}
	case groupChild:
		forbidden = []string{vocab.XSDAttrRef, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs}
	case attributeChild:
		forbidden = []string{vocab.XSDAttrRef, vocab.XSDAttrForm, vocab.XSDAttrUse}
	case elementChild:
		forbidden = []string{vocab.XSDAttrRef, vocab.XSDAttrForm, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs}
	case uniqueChild, keyChild, keyrefChild, selectorChild, fieldChild:
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "identity constraint must be inside element")
	default:
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "invalid top-level schema child "+diagnostictext.Quote(local))
	}
	for _, attr := range forbidden {
		if _, present := n.attr(attr); present {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "top-level "+diagnostictext.Quote(local)+" cannot have "+diagnostictext.Quote(attr))
		}
	}
	if _, present := n.attr(vocab.XSDAttrName); !present {
		return schemaCompileAt(n, xsderrors.CodeSchemaReference, "top-level "+diagnostictext.Quote(local)+" missing name")
	}
	return nil
}
