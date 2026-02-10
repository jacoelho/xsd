package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

func validateElementValueConstraints(sch *parser.Schema, decl *model.ElementDecl) error {
	if decl == nil {
		return nil
	}

	resolvedType := typeresolve.ResolveTypeReference(sch, decl.Type, typeresolve.TypeReferenceAllowMissing)
	if isDirectNotationType(resolvedType) {
		return fmt.Errorf("element cannot use NOTATION type")
	}

	if !decl.HasDefault && !decl.HasFixed {
		return nil
	}

	if ct, ok := resolvedType.(*model.ComplexType); ok {
		_, isSimpleContent := ct.Content().(*model.SimpleContent)
		if !isSimpleContent && !ct.EffectiveMixed() {
			if decl.HasDefault {
				return fmt.Errorf("element with element-only complex type cannot have default value")
			}
			return fmt.Errorf("element with element-only complex type cannot have fixed value")
		}
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
