package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/xmltree"
)

// validateAttributeDeclStructure validates structural constraints of an attribute declaration
// Does not validate references (which might be forward references or imports)
func validateAttributeDeclStructure(schemaDef *parser.Schema, attrQName model.QName, decl *model.AttributeDecl) error {
	if !qname.IsValidNCName(attrQName.Local) {
		return fmt.Errorf("invalid attribute name '%s': must be a valid NCName", attrQName.Local)
	}
	if attrQName.Local == "xmlns" {
		return fmt.Errorf("invalid attribute name '%s': reserved XMLNS name", attrQName.Local)
	}
	effectiveNamespace := attrQName.Namespace
	if !decl.IsReference {
		switch decl.Form {
		case model.FormQualified:
			effectiveNamespace = schemaDef.TargetNamespace
		case model.FormUnqualified:
			effectiveNamespace = ""
		default:
			if schemaDef.AttributeFormDefault == parser.Qualified {
				effectiveNamespace = schemaDef.TargetNamespace
			}
		}
	}
	if effectiveNamespace == xmltree.XSINamespace {
		return fmt.Errorf("invalid attribute name '%s': attributes in the xsi namespace are not allowed", attrQName.Local)
	}

	if decl.Type != nil {
		if st, ok := decl.Type.(*model.SimpleType); ok {
			if err := validateSimpleTypeStructure(schemaDef, st); err != nil {
				return fmt.Errorf("inline simpleType: %w", err)
			}
		}
	}

	if decl.HasDefault && decl.HasFixed {
		return fmt.Errorf("attribute cannot have both 'default' and 'fixed' values")
	}
	if decl.Use == model.Required && decl.HasDefault {
		return fmt.Errorf("attribute with use='required' cannot have a default value")
	}
	if decl.Use == model.Prohibited && decl.HasDefault {
		return fmt.Errorf("attribute with use='prohibited' cannot have a default value")
	}
	if decl.HasDefault {
		if err := validateDefaultOrFixedValueWithContext(schemaDef, decl.Default, decl.Type, decl.DefaultContext); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueWithContext(schemaDef, decl.Fixed, decl.Type, decl.FixedContext); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}

	return nil
}

// validateAttributeGroupStructure validates structural constraints of an attribute group
func validateAttributeGroupStructure(schema *parser.Schema, groupQName model.QName, ag *model.AttributeGroup) error {
	if !qname.IsValidNCName(groupQName.Local) {
		return fmt.Errorf("invalid attributeGroup name '%s': must be a valid NCName", groupQName.Local)
	}
	for _, attr := range ag.Attributes {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute: %w", err)
		}
	}
	if err := validateAttributeGroupUniqueness(schema, ag); err != nil {
		return err
	}
	return nil
}
