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

// checkSchemaAttributesSyntax checks the unqualified attributes of one start,
// in start-tag order, against the XSD 1.0 vocabulary of its element in its
// placement, and resolves its QName-valued attributes into qnames; see
// admitSchemaQNameAttribute. qnames may be nil only for an element that has no
// QName-valued attribute.
func checkSchemaAttributesSyntax(n *schemaSyntaxNode, placement schemaElementPlacement, qnames *semanticQNameProjection) error {
	for _, attr := range n.Attrs {
		if xmlstream.IsExpandedNamespaceName(attr.Name) || attr.Name.Space != "" {
			continue
		}
		if err := checkSchemaAttributeSyntax(n, attr, placement, qnames); err != nil {
			return err
		}
	}
	return nil
}

func checkSchemaAttributeSyntax(n *schemaSyntaxNode, attr schemaAttribute, placement schemaElementPlacement, qnames *semanticQNameProjection) error {
	switch schemaAttributeVocabularyOf(n.Name.Local, attr.Name.Local, placement) {
	case schemaAttributeUnknown:
		return schemaAttributeNotAllowed(n, attr.Name.Local)
	case schemaAttributeMisplaced:
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, misplacedSchemaAttributeMessage(n.Name.Local, attr.Name.Local, placement))
	case schemaAttributeXSD10:
	}
	if schemaNodeHasQNameAttr(n, attr.Name.Local) {
		return admitSchemaQNameAttribute(n, attr, qnames)
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

func checkSchemaNodeNamesSyntax(n *schemaSyntaxNode) error {
	id, hasID := n.attr(vocab.XSDAttrID)
	name, hasName := n.attr(vocab.XSDAttrName)
	return withSchemaCompileLocation(n, validateSchemaNodeNames(schemaNodeNames{
		ID:      id,
		Name:    name,
		HasID:   hasID,
		HasName: hasName,
	}))
}

func checkSchemaQNamePartsSyntax(n *schemaSyntaxNode, lexical string) (qNameParts, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return qNameParts{}, withSchemaCompileLocation(n, err)
	}
	return parts, nil
}

// checkRequiredSchemaAttributesSyntax checks that n carries the attributes
// its placement requires; see requiredSchemaAttributeMissing.
func checkRequiredSchemaAttributesSyntax(n *schemaSyntaxNode, parent xml.Name, placement schemaElementPlacement) error {
	if message := requiredSchemaAttributeMissing(n, parent, placement); message != "" {
		return schemaCompileAt(n, xsderrors.CodeSchemaReference, message)
	}
	return nil
}
