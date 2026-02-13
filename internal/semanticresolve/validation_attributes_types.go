package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/types"
)

func validateAttributeDeclarations(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range traversal.SortedQNames(sch.AttributeDecls) {
		decl := sch.AttributeDecls[qname]
		if decl.Type != nil {
			if err := validateTypeReferenceFromTypeAtLocation(sch, decl.Type, qname.Namespace, noOriginLocation); err != nil {
				errs = append(errs, fmt.Errorf("attribute %s: %w", qname, err))
			}
		}

		resolvedType := typeresolve.ResolveTypeReference(sch, decl.Type, typeresolve.TypeReferenceAllowMissing)
		if _, ok := resolvedType.(*types.ComplexType); ok {
			errs = append(errs, fmt.Errorf("attribute %s: type must be a simple type", qname))
		}
		if decl.HasDefault {
			if err := validateDefaultOrFixedValueResolved(sch, decl.Default, resolvedType, decl.DefaultContext, idValuesDisallowed); err != nil {
				errs = append(errs, fmt.Errorf("attribute %s: invalid default value '%s': %w", qname, decl.Default, err))
			}
		}
		if decl.HasFixed {
			if err := validateDefaultOrFixedValueResolved(sch, decl.Fixed, resolvedType, decl.FixedContext, idValuesDisallowed); err != nil {
				errs = append(errs, fmt.Errorf("attribute %s: invalid fixed value '%s': %w", qname, decl.Fixed, err))
			}
		}
	}

	return errs
}

func validateTypeDefinitionReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range traversal.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if err := validateTypeReferences(sch, qname, typ); err != nil {
			errs = append(errs, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errs
}
