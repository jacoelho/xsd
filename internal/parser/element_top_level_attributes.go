package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func applyTopLevelElementAttributes(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, decl *model.ElementDecl) error {
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

	hasDefault := doc.HasAttribute(elem, "default")
	hasFixed := doc.HasAttribute(elem, "fixed")
	applyElementValueConstraintFields(
		doc,
		elem,
		schema,
		hasDefault,
		doc.GetAttribute(elem, "default"),
		hasFixed,
		doc.GetAttribute(elem, "fixed"),
		decl,
	)

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

func applyTopLevelElementDerivations(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, decl *model.ElementDecl) error {
	if err := applyElementBlockDerivation(schema, decl, doc.HasAttribute(elem, "block"), doc.GetAttribute(elem, "block")); err != nil {
		return err
	}

	if doc.HasAttribute(elem, "final") {
		finalAttr := doc.GetAttribute(elem, "final")
		if model.TrimXMLWhitespace(finalAttr) == "" {
			return fmt.Errorf("final attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(finalAttr, model.DerivationSet(model.DerivationExtension|model.DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid final attribute value '%s': %w", finalAttr, err)
		}
		decl.Final = final
	} else if schema.FinalDefault != 0 {
		decl.Final = schema.FinalDefault & model.DerivationSet(model.DerivationExtension|model.DerivationRestriction)
	}

	return nil
}

func applyTopLevelElementSubstitutionGroup(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, decl *model.ElementDecl) error {
	if subGroup := doc.GetAttribute(elem, "substitutionGroup"); subGroup != "" {
		subGroupQName, err := resolveQNameWithPolicy(doc, subGroup, elem, schema, useDefaultNamespace)
		if err != nil {
			return fmt.Errorf("resolve substitutionGroup %s: %w", subGroup, err)
		}
		decl.SubstitutionGroup = subGroupQName

		if schema.SubstitutionGroups[subGroupQName] == nil {
			schema.SubstitutionGroups[subGroupQName] = []model.QName{}
		}
		schema.SubstitutionGroups[subGroupQName] = append(schema.SubstitutionGroups[subGroupQName], decl.Name)
	}
	return nil
}

func applyTopLevelElementConstraints(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, decl *model.ElementDecl) error {
	return appendElementIdentityConstraints(doc, elem, schema, decl)
}
