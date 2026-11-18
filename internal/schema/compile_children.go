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
// whatever its value. QName-valued attributes are resolved into the QName
// projection of admitted; see admitSchemaQNameAttribute.
func checkSchemaAttributesSyntax(n *schemaSyntaxNode, placement schemaElementPlacement, admitted startTagAttributes) error {
	for _, attr := range n.Attrs {
		var err error
		switch {
		case xmlstream.IsExpandedNamespaceName(attr.Name):
		case attr.Name.Space == "":
			err = checkSchemaAttributeSyntax(n, attr, placement, admitted)
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

func checkSchemaAttributeSyntax(n *schemaSyntaxNode, attr schemaAttribute, placement schemaElementPlacement, admitted startTagAttributes) error {
	if !schemaElementAttributeAllowed(n.Name.Local, attr.Name.Local) {
		return schemaAttributeNotAllowed(n, attr.Name.Local)
	}
	if err := misplacedSchemaAttributeError(n, attr.Name.Local, placement); err != nil {
		return err
	}
	return admitSchemaAttributeValue(n, attr, admitted)
}

// admitSchemaAttributeValue checks the value of attr, an unqualified attribute
// n may have in its placement, and admits it into admitted.
func admitSchemaAttributeValue(n *schemaSyntaxNode, attr schemaAttribute, admitted startTagAttributes) error {
	if err := withSchemaCompileLocation(n, validateSchemaNameAttribute(attr.Name.Local, attr.Value)); err != nil {
		return err
	}
	if attr.Name.Local == vocab.XSDAttrID && !admitted.ids.declare(attr.Value) {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "duplicate schema id "+diagnostictext.Quote(attr.Value))
	}
	if attr.Name.Local == vocab.XSDAttrXPath && lex.TrimXMLWhitespaceString(attr.Value) == "" {
		return schemaCompileAt(n, xsderrors.CodeSchemaIdentity, diagnostictext.Quote(n.Name.Local)+" xpath is empty")
	}
	if err := withSchemaCompileLocation(n, checkSchemaAttributeValueSyntax(n.Name.Local, attr.Name.Local, attr.Value)); err != nil {
		return err
	}
	if err := withSchemaCompileLocation(n, admitParsedAttributeValue(n.Name.Local, attr, admitted)); err != nil {
		return err
	}
	if schemaNodeHasQNameAttr(n, attr.Name.Local) {
		return admitSchemaQNameAttribute(n, attr, admitted.values)
	}
	return checkSchemaAnyURIAttribute(n, attr)
}

// admitParsedAttributeValue admits attr, an attribute of the XSD element
// named element, into admitted when its value is parsed at the tag: a facet
// value, or the fixed of a facet, which may precede or follow the facet's
// value.
func admitParsedAttributeValue(element string, attr schemaAttribute, admitted startTagAttributes) error {
	values := admitted.values
	switch attr.Name.Local {
	case vocab.XSDAttrValue:
		facet, err := admitFacetValue(element, attr.Value, admitted.patterns)
		facet.fixed = values.facet.fixed
		values.facet = facet
		return err
	case vocab.XSDAttrFixed:
		if !isFacetLocal(element) {
			return nil
		}
		fixed, err := parseBooleanAttr(booleanAttr{Name: attr.Name.Local, Value: attr.Value, HasValue: true})
		values.facet.fixed = fixed
		return err
	}
	return nil
}

func checkSchemaAnyURIAttribute(n *schemaSyntaxNode, attr schemaAttribute) error {
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
