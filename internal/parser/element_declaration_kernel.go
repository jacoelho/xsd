package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func applyElementValueConstraintFields(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, hasDefault bool, defaultValue string, hasFixed bool, fixedValue string, decl *model.ElementDecl) {
	if hasDefault {
		decl.Default = defaultValue
		decl.HasDefault = true
		decl.DefaultContext = namespaceContextForElement(doc, elem, schema)
	}

	if hasFixed {
		decl.Fixed = fixedValue
		decl.HasFixed = true
		decl.FixedContext = namespaceContextForElement(doc, elem, schema)
	}

	if decl.HasDefault || decl.HasFixed {
		decl.ValueContext = namespaceContextForElement(doc, elem, schema)
	}
}

func applyElementBlockDerivation(schema *Schema, decl *model.ElementDecl, hasBlock bool, blockValue string) error {
	if hasBlock {
		if model.TrimXMLWhitespace(blockValue) == "" {
			return fmt.Errorf("block attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(blockValue, model.DerivationSet(model.DerivationSubstitution|model.DerivationExtension|model.DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid block attribute value '%s': %w", blockValue, err)
		}
		decl.Block = block
		return nil
	}

	if schema.BlockDefault != 0 {
		decl.Block = schema.BlockDefault & model.DerivationSet(model.DerivationSubstitution|model.DerivationExtension|model.DerivationRestriction)
	}
	return nil
}

func appendElementIdentityConstraints(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, decl *model.ElementDecl) error {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "key", "keyref", "unique":
			constraint, err := parseIdentityConstraint(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse identity constraint: %w", err)
			}
			decl.Constraints = append(decl.Constraints, constraint)
		}
	}
	return nil
}
