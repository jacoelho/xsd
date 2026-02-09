package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func validateAttributeValueConstraintsForType(sch *parser.Schema, typ types.Type) error {
	ct, ok := typ.(*types.ComplexType)
	if !ok {
		return nil
	}
	validateAttrs := func(attrs []*types.AttributeDecl) error {
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

func validateAttributeValueConstraints(sch *parser.Schema, decl *types.AttributeDecl) error {
	resolvedType := typeops.ResolveTypeReference(sch, decl.Type, typeops.TypeReferenceAllowMissing)
	if _, ok := resolvedType.(*types.ComplexType); ok {
		return fmt.Errorf("type must be a simple type")
	}
	if isDirectNotationType(resolvedType) {
		return fmt.Errorf("attribute cannot use NOTATION type")
	}
	if decl.HasDefault {
		if err := validateDefaultOrFixedValueResolved(sch, decl.Default, resolvedType, decl.DefaultContext, make(map[types.Type]bool), idValuesDisallowed); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueResolved(sch, decl.Fixed, resolvedType, decl.FixedContext, make(map[types.Type]bool), idValuesDisallowed); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}
	return nil
}
