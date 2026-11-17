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

// checkSchemaAttributesSyntax checks every attribute of one start in
// start-tag order, so the first violating attribute is the one reported. Each
// unqualified attribute is checked against the XSD 1.0 vocabulary of its
// element in its placement before its value: a forbidden attribute is rejected
// whatever its value. QName-valued attributes are resolved into qnames; see
// admitSchemaQNameAttribute. qnames may be nil only for an element that has no
// QName-valued attribute.
func checkSchemaAttributesSyntax(n *schemaSyntaxNode, placement schemaElementPlacement, qnames *semanticQNameProjection) error {
	for _, attr := range n.Attrs {
		var err error
		switch {
		case xmlstream.IsExpandedNamespaceName(attr.Name):
		case attr.Name.Space == "":
			err = checkSchemaAttributeSyntax(n, attr, placement, qnames)
		case attr.Name.Space == vocab.XSDNamespaceURI:
			err = schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "schema namespace attribute "+diagnostictext.Quote(attr.Name.Local)+" is not allowed")
		case attr.Name.Space == vocab.XMLNamespaceURI:
			err = checkXMLAttributeSyntax(n, attr)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func checkXMLAttributeSyntax(n *schemaSyntaxNode, attr schemaAttribute) error {
	switch {
	case attr.Name.Local == vocab.XMLAttrBase:
		if _, err := uriref.Check(attr.Value); err != nil {
			return schemaCompileAt(n, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
		}
	case attr.Name.Local == vocab.XMLAttrLang && n.Name.Local == vocab.XSDElemDocumentation:
		if !lex.IsLanguage(lex.CollapseXMLWhitespace(attr.Value)) {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "invalid xml:lang on xs:documentation")
		}
	}
	return nil
}

func checkSchemaAttributeSyntax(n *schemaSyntaxNode, attr schemaAttribute, placement schemaElementPlacement, qnames *semanticQNameProjection) error {
	if !schemaElementAttributeAllowed(n.Name.Local, attr.Name.Local) {
		return schemaAttributeNotAllowed(n, attr.Name.Local)
	}
	if err := misplacedSchemaAttributeError(n, attr.Name.Local, placement); err != nil {
		return err
	}
	if err := withSchemaCompileLocation(n, validateSchemaNameAttribute(attr.Name.Local, attr.Value)); err != nil {
		return err
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

func checkSchemaQNamePartsSyntax(n *schemaSyntaxNode, lexical string) (qNameParts, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return qNameParts{}, withSchemaCompileLocation(n, err)
	}
	return parts, nil
}
