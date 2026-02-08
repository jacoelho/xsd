package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func applyTopLevelElementAttributes(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, decl *types.ElementDecl) error {
	if ok, value, err := parseBoolAttribute(doc, elem, "nillable"); err != nil {
		return err
	} else if ok {
		decl.Nillable = value
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "abstract"); err != nil {
		return err
	} else if ok {
		decl.Abstract = value
	}

	if doc.HasAttribute(elem, "default") {
		decl.Default = doc.GetAttribute(elem, "default")
		decl.HasDefault = true
		decl.DefaultContext = namespaceContextForElement(doc, elem, schema)
	}

	if doc.HasAttribute(elem, "fixed") {
		decl.Fixed = doc.GetAttribute(elem, "fixed")
		decl.HasFixed = true
		decl.FixedContext = namespaceContextForElement(doc, elem, schema)
	}
	if decl.HasDefault || decl.HasFixed {
		decl.ValueContext = namespaceContextForElement(doc, elem, schema)
	}

	if err := applyTopLevelElementDerivations(doc, elem, schema, decl); err != nil {
		return err
	}
	if err := applyTopLevelElementSubstitutionGroup(doc, elem, schema, decl); err != nil {
		return err
	}
	if err := applyTopLevelElementConstraints(doc, elem, schema, decl); err != nil {
		return err
	}

	return nil
}

func applyTopLevelElementDerivations(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, decl *types.ElementDecl) error {
	if doc.HasAttribute(elem, "block") {
		blockAttr := doc.GetAttribute(elem, "block")
		if types.TrimXMLWhitespace(blockAttr) == "" {
			return fmt.Errorf("block attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(blockAttr, types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid block attribute value '%s': %w", blockAttr, err)
		}
		decl.Block = block
	} else if schema.BlockDefault != 0 {
		decl.Block = schema.BlockDefault & types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction)
	}

	if doc.HasAttribute(elem, "final") {
		finalAttr := doc.GetAttribute(elem, "final")
		if types.TrimXMLWhitespace(finalAttr) == "" {
			return fmt.Errorf("final attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(finalAttr, types.DerivationSet(types.DerivationExtension|types.DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid final attribute value '%s': %w", finalAttr, err)
		}
		decl.Final = final
	} else if schema.FinalDefault != 0 {
		decl.Final = schema.FinalDefault & types.DerivationSet(types.DerivationExtension|types.DerivationRestriction)
	}

	return nil
}

func applyTopLevelElementSubstitutionGroup(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, decl *types.ElementDecl) error {
	if subGroup := doc.GetAttribute(elem, "substitutionGroup"); subGroup != "" {
		subGroupQName, err := resolveElementQName(doc, subGroup, elem, schema)
		if err != nil {
			return fmt.Errorf("resolve substitutionGroup %s: %w", subGroup, err)
		}
		decl.SubstitutionGroup = subGroupQName

		if schema.SubstitutionGroups[subGroupQName] == nil {
			schema.SubstitutionGroups[subGroupQName] = []types.QName{}
		}
		schema.SubstitutionGroups[subGroupQName] = append(schema.SubstitutionGroups[subGroupQName], decl.Name)
	}
	return nil
}

func applyTopLevelElementConstraints(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, decl *types.ElementDecl) error {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
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
