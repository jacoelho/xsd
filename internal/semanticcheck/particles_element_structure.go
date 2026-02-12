package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
)

func validateElementParticle(schema *parser.Schema, elem *model.ElementDecl) error {
	if err := validateElementConstraints(elem); err != nil {
		return err
	}
	if err := validateElementConstraintNames(elem); err != nil {
		return err
	}
	if elem.IsReference {
		return validateReferencedElementType(schema, elem)
	}
	if elem.Type == nil {
		return nil
	}
	return validateInlineElementType(schema, elem)
}

func validateElementConstraints(elem *model.ElementDecl) error {
	for _, constraint := range elem.Constraints {
		if err := validateIdentityConstraint(constraint); err != nil {
			return fmt.Errorf("element '%s' identity constraint '%s': %w", elem.Name, constraint.Name, err)
		}
	}
	return nil
}

func validateElementConstraintNames(elem *model.ElementDecl) error {
	constraintNames := make(map[string]bool)
	for _, constraint := range elem.Constraints {
		if constraintNames[constraint.Name] {
			return fmt.Errorf("element '%s': duplicate identity constraint name '%s'", elem.Name, constraint.Name)
		}
		constraintNames[constraint.Name] = true
	}
	return nil
}

func validateReferencedElementType(schema *parser.Schema, elem *model.ElementDecl) error {
	if refDecl, exists := schema.ElementDecls[elem.Name]; exists {
		if refDecl.Type == nil {
			return fmt.Errorf("referenced element '%s' must have a type", elem.Name)
		}
	}
	return nil
}

func validateInlineElementType(schema *parser.Schema, elem *model.ElementDecl) error {
	if st, ok := elem.Type.(*model.SimpleType); ok && st.QName.IsZero() {
		if err := validateSimpleTypeStructure(schema, st); err != nil {
			return fmt.Errorf("inline simpleType in element '%s': %w", elem.Name, err)
		}
		return nil
	}
	if complexType, ok := elem.Type.(*model.ComplexType); ok && complexType.QName.IsZero() {
		if err := validateComplexTypeStructure(schema, complexType, typeDefinitionInline); err != nil {
			return fmt.Errorf("inline complexType in element '%s': %w", elem.Name, err)
		}
	}
	return nil
}

// validateGroupStructure validates structural constraints of a group definition
// Does not validate references (which might be forward references or imports)
func validateGroupStructure(groupQName model.QName, group *model.ModelGroup) error {
	if !qname.IsValidNCName(groupQName.Local) {
		return fmt.Errorf("invalid group name '%s': must be a valid NCName", groupQName.Local)
	}

	if group.MinOccurs.IsZero() {
		return fmt.Errorf("group '%s' cannot have minOccurs='0'", groupQName.Local)
	}
	if group.MaxOccurs.IsUnbounded() {
		return fmt.Errorf("group '%s' cannot have maxOccurs='unbounded'", groupQName.Local)
	}
	if !group.MinOccurs.IsOne() || !group.MaxOccurs.IsOne() {
		return fmt.Errorf("group '%s' must have minOccurs='1' and maxOccurs='1' (got minOccurs=%s, maxOccurs=%s)", groupQName.Local, group.MinOccurs, group.MaxOccurs)
	}

	return nil
}
