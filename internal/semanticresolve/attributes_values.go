package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
)

func validateAttributeValueConstraintsForType(sch *parser.Schema, typ model.Type) error {
	ct, ok := typ.(*model.ComplexType)
	if !ok {
		return nil
	}
	validateAttrs := func(attrs []*model.AttributeDecl) error {
		for _, attr := range attrs {
			if err := validateAttributeValueConstraints(sch, attr); err != nil {
				return fmt.Errorf("attribute %s: %w", attr.Name, err)
			}
		}
		return nil
	}
	if err := validateAttrs(ct.Attributes()); err != nil {
		return err
	}
	if ext := ct.Content().ExtensionDef(); ext != nil {
		if err := validateAttrs(ext.Attributes); err != nil {
			return err
		}
	}
	if restr := ct.Content().RestrictionDef(); restr != nil {
		if err := validateAttrs(restr.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func validateAttributeValueConstraints(sch *parser.Schema, decl *model.AttributeDecl) error {
	resolvedType := typeops.ResolveTypeReference(sch, decl.Type, typeops.TypeReferenceAllowMissing)
	if _, ok := resolvedType.(*model.ComplexType); ok {
		return fmt.Errorf("type must be a simple type")
	}
	if isDirectNotationType(resolvedType) {
		return fmt.Errorf("attribute cannot use NOTATION type")
	}
	if decl.HasDefault {
		if err := validateDefaultOrFixedValueResolved(sch, decl.Default, resolvedType, decl.DefaultContext, make(map[model.Type]bool), idValuesDisallowed); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueResolved(sch, decl.Fixed, resolvedType, decl.FixedContext, make(map[model.Type]bool), idValuesDisallowed); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}
	return nil
}
