package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// validateAttributeDeclStructure validates structural constraints of an attribute declaration
// Does not validate references (which might be forward references or imports)
func validateAttributeDeclStructure(schemaDef *schema.Schema, qname types.QName, decl *types.AttributeDecl) error {
	// This is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid attribute name '%s': must be a valid NCName", qname.Local)
	}
	if qname.Local == "xmlns" {
		return fmt.Errorf("invalid attribute name '%s': reserved XMLNS name", qname.Local)
	}
	effectiveNamespace := qname.Namespace
	if !decl.IsReference {
		switch decl.Form {
		case types.FormQualified:
			effectiveNamespace = schemaDef.TargetNamespace
		case types.FormUnqualified:
			effectiveNamespace = ""
		default:
			if schemaDef.AttributeFormDefault == schema.Qualified {
				effectiveNamespace = schemaDef.TargetNamespace
			}
		}
	}
	if effectiveNamespace == xml.XSINamespace {
		return fmt.Errorf("invalid attribute name '%s': attributes in the xsi namespace are not allowed", qname.Local)
	}

	// Validate inline types (simpleType defined inline in the attribute)
	if decl.Type != nil {
		if st, ok := decl.Type.(*types.SimpleType); ok {
			if err := validateSimpleTypeStructure(schemaDef, st); err != nil {
				return fmt.Errorf("inline simpleType: %w", err)
			}
		}
	}

	// Per XSD spec: cannot have both default and fixed on an attribute
	// (au-props-correct constraint 2)
	if decl.Default != "" && decl.Fixed != "" {
		return fmt.Errorf("attribute cannot have both 'default' and 'fixed' values")
	}

	// Per XSD spec: if use="required", default is not allowed
	// (au-props-correct constraint 1)
	if decl.Use == types.Required && decl.Default != "" {
		return fmt.Errorf("attribute with use='required' cannot have a default value")
	}
	// Validate default value if present
	if decl.Default != "" {
		if err := validateDefaultOrFixedValue(decl.Default, decl.Type); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}

	// Validate fixed value if present
	if decl.HasFixed {
		if err := validateDefaultOrFixedValue(decl.Fixed, decl.Type); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}

	// Don't validate type references - they might be forward references or from imports

	return nil
}

// validateAttributeGroupStructure validates structural constraints of an attribute group
func validateAttributeGroupStructure(schema *schema.Schema, qname types.QName, ag *types.AttributeGroup) error {
	// This is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid attributeGroup name '%s': must be a valid NCName", qname.Local)
	}

	// Validate attribute declarations - only structural constraints
	for _, attr := range ag.Attributes {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute: %w", err)
		}
	}

	if err := validateAttributeGroupUniqueness(schema, ag); err != nil {
		return err
	}

	// Don't validate attribute group references - they might be forward references

	return nil
}
