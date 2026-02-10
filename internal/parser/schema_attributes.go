package parser

import (
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseSchemaAttributes(doc *schemaxml.Document, root schemaxml.NodeID, schema *Schema) error {
	if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
		return err
	}
	attrs := make([]schemaAttribute, 0, len(doc.Attributes(root)))
	nsDecls := make([]schemaNamespaceDecl, 0, len(doc.Attributes(root)))
	for _, attr := range doc.Attributes(root) {
		attrs = append(attrs, schemaAttribute{
			namespace: attr.NamespaceURI(),
			local:     attr.LocalName(),
			value:     attr.Value(),
		})
		if isXMLNSDeclaration(attr) {
			prefix := attr.LocalName()
			if prefix == "xmlns" {
				prefix = ""
			}
			nsDecls = append(nsDecls, schemaNamespaceDecl{
				prefix: prefix,
				uri:    attr.Value(),
			})
		}
	}

	return applySchemaRootAttributes(schema, attrs, nsDecls)
}
