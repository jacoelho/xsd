package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

type derivationRestrictionParser func(*schemaxml.Document, schemaxml.NodeID, *Schema) (*model.Restriction, model.QName, error)
type derivationExtensionParser func(*schemaxml.Document, schemaxml.NodeID, *Schema) (*model.Extension, model.QName, error)

type parsedDerivationContent struct {
	restriction *model.Restriction
	extension   *model.Extension
	base        model.QName
}

func parseDerivationContent(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, context string, parseRestriction derivationRestrictionParser, parseExtension derivationExtensionParser) (parsedDerivationContent, error) {
	parsed := parsedDerivationContent{}
	seenDerivation := false
	seenAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			_, err := handleSingleLeadingAnnotation(
				"annotation",
				&seenAnnotation,
				seenDerivation,
				fmt.Sprintf("%s: at most one annotation is allowed", context),
				fmt.Sprintf("%s: annotation must appear before restriction or extension", context),
			)
			if err != nil {
				return parsed, err
			}
			continue
		case "restriction":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return parsed, err
			}
			if seenDerivation {
				return parsed, fmt.Errorf("%s must have exactly one derivation child (restriction or extension)", context)
			}
			seenDerivation = true
			restriction, baseQName, err := parseRestriction(doc, child, schema)
			if err != nil {
				return parsed, err
			}
			parsed.base = baseQName
			parsed.restriction = restriction
		case "extension":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return parsed, err
			}
			if seenDerivation {
				return parsed, fmt.Errorf("%s must have exactly one derivation child (restriction or extension)", context)
			}
			seenDerivation = true
			extension, baseQName, err := parseExtension(doc, child, schema)
			if err != nil {
				return parsed, err
			}
			parsed.base = baseQName
			parsed.extension = extension
		default:
			return parsed, fmt.Errorf("%s has unexpected child element '%s'", context, doc.LocalName(child))
		}
	}

	if !seenDerivation {
		return parsed, fmt.Errorf("%s must have exactly one derivation child (restriction or extension)", context)
	}

	return parsed, nil
}

func parseDerivationBaseQName(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, kind string) (model.QName, error) {
	if err := validateOptionalID(doc, elem, kind, schema); err != nil {
		return model.QName{}, err
	}

	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return model.QName{}, fmt.Errorf("%s missing base", kind)
	}

	baseQName, err := resolveQNameWithPolicy(doc, base, elem, schema, useDefaultNamespace)
	if err != nil {
		return model.QName{}, err
	}
	return baseQName, nil
}
