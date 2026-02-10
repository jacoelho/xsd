package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateComplexTypeStructure validates structural constraints of a complex type.
// Does not validate references (which might be forward references or imports).
func validateComplexTypeStructure(schema *parser.Schema, complexType *model.ComplexType, context typeDefinitionContext) error {
	if err := validateContentStructure(schema, complexType.Content(), context); err != nil {
		return fmt.Errorf("content: %w", err)
	}
	if err := ValidateUPA(schema, complexType.Content(), schema.TargetNamespace); err != nil {
		return fmt.Errorf("UPA violation: %w", err)
	}
	if err := validateElementDeclarationsConsistent(schema, complexType); err != nil {
		return fmt.Errorf("element declarations consistent: %w", err)
	}
	if err := validateMixedContentDerivation(schema, complexType); err != nil {
		return fmt.Errorf("mixed content derivation: %w", err)
	}
	if err := validateWildcardDerivation(schema, complexType); err != nil {
		return fmt.Errorf("wildcard derivation: %w", err)
	}
	if err := validateAnyAttributeDerivation(schema, complexType); err != nil {
		return fmt.Errorf("anyAttribute derivation: %w", err)
	}
	if _, err := collectAnyAttributeFromType(schema, complexType); err != nil {
		return fmt.Errorf("anyAttribute: %w", err)
	}

	for _, attr := range complexType.Attributes() {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute %s: %w", attr.Name, err)
		}
	}
	if content := complexType.Content(); content != nil {
		if ext := content.ExtensionDef(); ext != nil {
			for _, attr := range ext.Attributes {
				if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
					return fmt.Errorf("extension attribute %s: %w", attr.Name, err)
				}
			}
		}
		if restr := content.RestrictionDef(); restr != nil {
			for _, attr := range restr.Attributes {
				if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
					return fmt.Errorf("restriction attribute %s: %w", attr.Name, err)
				}
			}
		}
	}

	if err := validateAttributeUniqueness(schema, complexType); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}
	if err := validateExtensionAttributeUniqueness(schema, complexType); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}
	if err := validateIDAttributeCount(schema, complexType); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}
	if err := validateNoCircularDerivation(schema, complexType); err != nil {
		return fmt.Errorf("circular derivation: %w", err)
	}
	if err := validateDerivationConstraints(schema, complexType); err != nil {
		return fmt.Errorf("derivation constraints: %w", err)
	}

	return nil
}
