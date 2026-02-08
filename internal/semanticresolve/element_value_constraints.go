package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func validateElementValueConstraints(sch *parser.Schema, decl *types.ElementDecl) error {
	if decl == nil {
		return nil
	}

	resolvedType := typeops.ResolveTypeReference(sch, decl.Type, typeops.TypeReferenceAllowMissing)
	if isDirectNotationType(resolvedType) {
		return fmt.Errorf("element cannot use NOTATION type")
	}

	if !decl.HasDefault && !decl.HasFixed {
		return nil
	}

	if ct, ok := resolvedType.(*types.ComplexType); ok {
		_, isSimpleContent := ct.Content().(*types.SimpleContent)
		if !isSimpleContent && !ct.EffectiveMixed() {
			if decl.HasDefault {
				return fmt.Errorf("element with element-only complex type cannot have default value")
			}
			return fmt.Errorf("element with element-only complex type cannot have fixed value")
		}
	}

	if decl.HasDefault {
		if err := validateDefaultOrFixedValueWithResolvedType(sch, decl.Default, resolvedType, decl.DefaultContext); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueWithResolvedType(sch, decl.Fixed, resolvedType, decl.FixedContext); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}
	return nil
}
