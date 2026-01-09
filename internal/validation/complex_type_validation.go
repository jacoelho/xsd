package validation

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateComplexTypeStructure validates structural constraints of a complex type
// Does not validate references (which might be forward references or imports)
// isInline indicates if this complexType is defined inline in an element (local element)
func validateComplexTypeStructure(schema *parser.Schema, ct *types.ComplexType, isInline bool) error {
	if err := validateContentStructure(schema, ct.Content(), isInline); err != nil {
		return fmt.Errorf("content: %w", err)
	}

	if err := validateUPA(schema, ct.Content(), schema.TargetNamespace); err != nil {
		return fmt.Errorf("UPA violation: %w", err)
	}

	if err := validateElementDeclarationsConsistent(schema, ct); err != nil {
		return fmt.Errorf("element declarations consistent: %w", err)
	}

	if err := validateMixedContentDerivation(schema, ct); err != nil {
		return fmt.Errorf("mixed content derivation: %w", err)
	}

	if err := validateWildcardDerivation(schema, ct); err != nil {
		return fmt.Errorf("wildcard derivation: %w", err)
	}

	if err := validateAnyAttributeDerivation(schema, ct); err != nil {
		return fmt.Errorf("anyAttribute derivation: %w", err)
	}

	for _, attr := range ct.Attributes() {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute %s: %w", attr.Name, err)
		}
	}

	if content := ct.Content(); content != nil {
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

	if err := validateAttributeUniqueness(schema, ct); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}

	if err := validateIDAttributeCount(schema, ct); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}

	if err := validateNoCircularDerivation(schema, ct); err != nil {
		return fmt.Errorf("circular derivation: %w", err)
	}

	if err := validateDerivationConstraints(schema, ct); err != nil {
		return fmt.Errorf("derivation constraints: %w", err)
	}

	return nil
}
