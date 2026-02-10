package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
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

func collectElementReferencesInSchema(sch *parser.Schema) []*model.ElementDecl {
	var elementRefsInContent []*model.ElementDecl

	for _, qname := range traversal.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, traversal.CollectFromContent(ct.Content(), func(p model.Particle) (*model.ElementDecl, bool) {
				decl, ok := p.(*model.ElementDecl)
				return decl, ok && decl.IsReference
			})...)
		}
	}

	for _, qname := range traversal.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, traversal.CollectFromContent(ct.Content(), func(p model.Particle) (*model.ElementDecl, bool) {
				decl, ok := p.(*model.ElementDecl)
				return decl, ok && decl.IsReference
			})...)
		}
	}

	for _, qname := range traversal.SortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		for _, particle := range group.Particles {
			if elem, ok := particle.(*model.ElementDecl); ok && elem.IsReference {
				elementRefsInContent = append(elementRefsInContent, elem)
			} else if mg, ok := particle.(*model.ModelGroup); ok {
				elementRefsInContent = append(elementRefsInContent, traversal.CollectFromParticlesWithVisited(mg.Particles, make(map[*model.ModelGroup]bool), func(p model.Particle) (*model.ElementDecl, bool) {
					decl, ok := p.(*model.ElementDecl)
					return decl, ok && decl.IsReference
				})...)
			}
		}
	}

	return elementRefsInContent
}
