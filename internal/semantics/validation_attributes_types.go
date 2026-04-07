package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateAttributeDeclarations(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.AttributeDecls) {
		decl := sch.AttributeDecls[qname]
		errs = append(errs, validateAttributeDeclarationTypeReference(sch, qname, decl)...)
		resolvedType := parser.ResolveTypeReferenceAllowMissing(sch, decl.Type)
		errs = append(errs, validateAttributeDeclarationResolvedType(sch, qname, decl, resolvedType)...)
	}

	return errs
}

func validateAttributeDeclarationTypeReference(sch *parser.Schema, qname model.QName, decl *model.AttributeDecl) []error {
	if decl.Type == nil {
		return nil
	}
	if err := validateTypeReferenceFromTypeAtLocation(sch, decl.Type, qname.Namespace, noOriginLocation); err != nil {
		return []error{fmt.Errorf("attribute %s: %w", qname, err)}
	}
	return nil
}

func validateAttributeDeclarationResolvedType(sch *parser.Schema, qname model.QName, decl *model.AttributeDecl, resolvedType model.Type) []error {
	var errs []error

	if _, ok := resolvedType.(*model.ComplexType); ok {
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

	return errs
}

func validateTypeDefinitionReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if err := validateTypeReferences(sch, qname, typ); err != nil {
			errs = append(errs, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errs
}
