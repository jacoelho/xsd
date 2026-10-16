package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func schemaLexicalAttributeSyntax(n *schemaSyntaxNode, name string) LexicalAttribute {
	value, present := n.attr(name)
	return LexicalAttribute{Value: value, Present: present}
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
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, n.Name.Local+" cannot have attribute "+attr.Name.Local)
	}
	if !schemaAnyURIAttribute(n.Name.Local, attr.Name.Local) {
		return nil
	}
	if _, err := uriref.Check(lex.CollapseXMLWhitespace(attr.Value)); err != nil {
		code := xsderrors.CodeSchemaInvalidAttribute
		if attr.Name.Local == vocab.XSDAttrSchemaLocation {
			code = xsderrors.CodeSchemaReference
		}
		return schemaCompileAt(n, code, "invalid "+attr.Name.Local+": "+err.Error())
	}
	return nil
}

func rejectSchemaNamespaceAttributes(n *schemaSyntaxNode) error {
	for _, attr := range n.Attrs {
		if attr.Name.Space == vocab.XSDNamespaceURI {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "schema namespace attribute "+attr.Name.Local+" is not allowed")
		}
	}
	return nil
}

func checkUnsupportedXSDNodeSyntax(n *schemaSyntaxNode, parentName xml.Name) (bool, error) {
	switch n.Name.Local {
	case redefineChild:
		return false, unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedRedefine, "xs:redefine is not supported")
	case notationChild:
		if parentName != (xml.Name{Space: vocab.XSDNamespaceURI, Local: vocab.XSDElemSchema}) {
			return false, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:notation must be a top-level schema child")
		}
	case assertChild, "alternative", "override", "openContent", "defaultOpenContent":
		return false, unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 feature "+n.Name.Local+" is not supported")
	case anyChild, anyAttribute:
		return rejectUnsupportedWildcardAttributes(n)
	}
	return false, nil
}

func rejectUnsupportedWildcardAttributes(n *schemaSyntaxNode) (bool, error) {
	for _, attr := range []string{vocab.XSDAttrNotNamespace, vocab.XSDAttrNotQName} {
		if _, ok := n.attr(attr); ok {
			return false, unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedXSD11, "XSD 1.1 wildcard attribute "+attr+" is not supported")
		}
	}
	return false, nil
}

func checkSchemaNodeNamesSyntax(n, parent *schemaSyntaxNode) error {
	var parentLocal string
	var parentXSD bool
	if parent != nil {
		parentLocal = parent.Name.Local
		parentXSD = parent.Name.Space == vocab.XSDNamespaceURI
	}
	id, hasID := n.attr(vocab.XSDAttrID)
	name, hasName := n.attr(vocab.XSDAttrName)
	return withSchemaCompileLocation(n, ValidateSchemaNodeNames(SchemaNodeNames{
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

func checkSchemaQNamePartsSyntax(n *schemaSyntaxNode, lexical string) (QNameParts, error) {
	parts, err := ParseQNameParts(lexical)
	if err != nil {
		return QNameParts{}, withSchemaCompileLocation(n, err)
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
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "invalid top-level schema child "+local)
	}
	for _, attr := range forbidden {
		if _, present := n.attr(attr); present {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "top-level "+local+" cannot have "+attr)
		}
	}
	if _, present := n.attr(vocab.XSDAttrName); !present {
		return schemaCompileAt(n, xsderrors.CodeSchemaReference, "top-level "+local+" missing name")
	}
	return nil
}
