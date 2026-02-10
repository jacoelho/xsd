package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func applyElementConstraints(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, attrs *elementAttrScan, decl *model.ElementDecl) error {
	if attrs.hasNillable {
		value, err := parseBoolValue("nillable", attrs.nillable)
		if err != nil {
			return err
		}
		decl.Nillable = value
	}

	if attrs.hasDefault {
		decl.Default = attrs.defaultVal
		decl.HasDefault = true
		decl.DefaultContext = namespaceContextForElement(doc, elem, schema)
	}

	if attrs.hasFixed {
		decl.Fixed = attrs.fixedVal
		decl.HasFixed = true
		decl.FixedContext = namespaceContextForElement(doc, elem, schema)
	}
	if decl.HasDefault || decl.HasFixed {
		decl.ValueContext = namespaceContextForElement(doc, elem, schema)
	}

	if attrs.hasBlock {
		blockAttr := attrs.block
		if model.TrimXMLWhitespace(blockAttr) == "" {
			return fmt.Errorf("block attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(blockAttr, model.DerivationSet(model.DerivationSubstitution|model.DerivationExtension|model.DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid block attribute value '%s': %w", blockAttr, err)
		}
		decl.Block = block
	} else if schema.BlockDefault != 0 {
		decl.Block = schema.BlockDefault & model.DerivationSet(model.DerivationSubstitution|model.DerivationExtension|model.DerivationRestriction)
	}

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
