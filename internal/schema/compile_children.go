package schema

import (
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

// checkSchemaAttributesSyntax checks the unqualified attributes of one start
// and returns the first XSD 1.1 attribute XSD 1.1 permits there. The caller
// reports it only after every XSD 1.0 start check has passed.
func checkSchemaAttributesSyntax(n *schemaSyntaxNode, p schemaPlacement) (string, error) {
	var xsd11Attr string
	for _, attr := range n.Attrs {
		if xmlstream.IsExpandedNamespaceName(attr.Name) || attr.Name.Space != "" {
			continue
		}
		permittedXSD11, err := checkSchemaAttributeSyntax(n, attr, p)
		if err != nil {
			return "", err
		}
		if permittedXSD11 && xsd11Attr == "" {
			xsd11Attr = attr.Name.Local
		}
	}
	return xsd11Attr, nil
}

func checkSchemaAttributeSyntax(n *schemaSyntaxNode, attr schemaAttribute, p schemaPlacement) (bool, error) {
	switch schemaAttributeVocabularyOf(n.Name.Local, attr.Name.Local, p.topLevel()) {
	case schemaAttributeUnknown:
		return false, schemaAttributeNotAllowed(n, attr.Name.Local)
	case schemaAttributeLocalOnly:
		return false, schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "top-level "+diagnostictext.Quote(n.Name.Local)+" cannot have "+diagnostictext.Quote(attr.Name.Local))
	case schemaAttributeXSD11:
		if xsd11AttributePermitted(n, attr.Name.Local, p) {
			return true, nil
		}
		return false, schemaAttributeNotAllowed(n, attr.Name.Local)
	case schemaAttributeXSD10:
	}
	if !schemaAnyURIAttribute(n.Name.Local, attr.Name.Local) {
		return false, nil
	}
	if _, err := uriref.Check(lex.CollapseXMLWhitespace(attr.Value)); err != nil {
		code := xsderrors.CodeSchemaInvalidAttribute
		if attr.Name.Local == vocab.XSDAttrSchemaLocation {
			code = xsderrors.CodeSchemaReference
		}
		return false, schemaCompileAt(n, code, "invalid "+diagnostictext.Quote(attr.Name.Local)+": "+err.Error())
	}
	return false, nil
}

func schemaAttributeNotAllowed(n *schemaSyntaxNode, attr string) error {
	return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, diagnostictext.Quote(n.Name.Local)+" cannot have attribute "+diagnostictext.Quote(attr))
}

func rejectSchemaNamespaceAttributes(n *schemaSyntaxNode) error {
	for _, attr := range n.Attrs {
		if attr.Name.Space == vocab.XSDNamespaceURI {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "schema namespace attribute "+diagnostictext.Quote(attr.Name.Local)+" is not allowed")
		}
	}
	return nil
}

func checkUnsupportedXSDNodeSyntax(n *schemaSyntaxNode, p schemaPlacement) (xsd11Use, error) {
	switch n.Name.Local {
	case redefineChild:
		return xsd11None, unsupportedAtSchemaNode(n, xsderrors.CodeUnsupportedRedefine, "xs:redefine is not supported")
	case notationChild:
		if !p.topLevel() {
			return xsd11None, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:notation must be a top-level schema child")
		}
	}
	if !isXSD11Element(n.Name.Local) {
		return xsd11None, nil
	}
	if !xsd11ElementPermitted(n.Name.Local, p) {
		return xsd11Misplaced, nil
	}
	return xsd11None, unsupportedXSD11Element(n)
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
	switch local {
	case annotationChild, includeChild, importChild, redefineChild, notationChild:
		return nil
	case simpleTypeChild, complexTypeChild, attributeGroup, groupChild, attributeChild, elementChild:
	case uniqueChild, keyChild, keyrefChild, selectorChild, fieldChild:
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "identity constraint must be inside element")
	default:
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "invalid top-level schema child "+diagnostictext.Quote(local))
	}
	if _, present := n.attr(vocab.XSDAttrName); !present {
		return schemaCompileAt(n, xsderrors.CodeSchemaReference, "top-level "+diagnostictext.Quote(local)+" missing name")
	}
	return nil
}
