package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

// ValidateStructure validates that a parsed schema conforms to XSD structural constraints.
// Reference validation is handled separately during the resolver phase.
func ValidateStructure(schema *parser.Schema) []error {
	var errors []error

	for qname, decl := range schema.ElementDecls {
		if err := validateElementDeclStructure(schema, qname, decl); err != nil {
			errors = append(errors, fmt.Errorf("element %s: %w", qname, err))
		}
	}

	for qname, decl := range schema.AttributeDecls {
		if err := validateAttributeDeclStructure(schema, qname, decl); err != nil {
			errors = append(errors, fmt.Errorf("attribute %s: %w", qname, err))
		}
	}

	for qname, typ := range schema.TypeDefs {
		if err := validateTypeDefStructure(schema, qname, typ); err != nil {
			errors = append(errors, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	for qname, group := range schema.Groups {
		if err := validateGroupStructure(qname, group); err != nil {
			errors = append(errors, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	for qname, ag := range schema.AttributeGroups {
		if err := validateAttributeGroupStructure(schema, qname, ag); err != nil {
			errors = append(errors, fmt.Errorf("attributeGroup %s: %w", qname, err))
		}
	}

	return errors
}
