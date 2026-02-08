package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateReferences validates cross-component references for schema loading.
func ValidateReferences(sch *parser.Schema) []error {
	var errs []error

	elementRefsInContent := collectElementReferencesInSchema(sch)
	allConstraints := collectAllIdentityConstraints(sch)

	if uniquenessErrors := validateIdentityConstraintUniqueness(sch); len(uniquenessErrors) > 0 {
		errs = append(errs, uniquenessErrors...)
	}

	errs = append(errs, validateTopLevelElementReferences(sch)...)
	errs = append(errs, validateContentElementReferences(sch, elementRefsInContent)...)
	errs = append(errs, validateElementDeclarationReferences(sch, allConstraints)...)

	if err := validateNoCyclicSubstitutionGroups(sch); err != nil {
		errs = append(errs, err)
	}

	errs = append(errs, validateLocalIdentityConstraintKeyrefs(sch, allConstraints)...)
	errs = append(errs, validateLocalIdentityConstraintResolution(sch)...)
	errs = append(errs, validateAttributeDeclarations(sch)...)
	errs = append(errs, validateTypeDefinitionReferences(sch)...)
	errs = append(errs, validateEnumerationFacetValues(sch)...)
	errs = append(errs, validateInlineTypeReferences(sch)...)
	errs = append(errs, validateComplexTypeReferences(sch)...)
	errs = append(errs, validateAttributeGroupReferencesInSchema(sch)...)
	errs = append(errs, validateLocalElementValueConstraints(sch)...)
	errs = append(errs, validateGroupReferencesInSchema(sch)...)

	if err := validateNoCyclicAttributeGroups(sch); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func collectElementReferencesInSchema(sch *parser.Schema) []*types.ElementDecl {
	var elementRefsInContent []*types.ElementDecl

	for _, qname := range sortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, collectElementReferences(ct.Content())...)
		}
	}

	for _, qname := range sortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, collectElementReferences(ct.Content())...)
		}
	}

	for _, qname := range sortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		for _, particle := range group.Particles {
			if elem, ok := particle.(*types.ElementDecl); ok && elem.IsReference {
				elementRefsInContent = append(elementRefsInContent, elem)
			} else if mg, ok := particle.(*types.ModelGroup); ok {
				elementRefsInContent = append(elementRefsInContent, collectElementReferencesFromParticles(mg.Particles)...)
			}
		}
	}

	return elementRefsInContent
}
