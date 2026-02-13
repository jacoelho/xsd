package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateReferences validates cross-component references for schema loading.
func ValidateReferences(sch *parser.Schema) []error {
	var errs []error
	index := buildIterationIndex(sch)

	elementRefsInContent := index.elementRefsInContent
	allConstraints := index.allIdentityConstraints

	if uniquenessErrors := validateIdentityConstraintUniquenessWithConstraints(sch, allConstraints); len(uniquenessErrors) > 0 {
		errs = append(errs, uniquenessErrors...)
	}

	errs = append(errs, validateTopLevelElementReferences(sch)...)
	errs = append(errs, validateContentElementReferences(sch, elementRefsInContent)...)
	errs = append(errs, validateElementDeclarationReferences(sch, allConstraints)...)

	if err := validateNoCyclicSubstitutionGroups(sch); err != nil {
		errs = append(errs, err)
	}

	errs = append(errs, validateLocalIdentityConstraintKeyrefsWithIndex(sch, index, allConstraints)...)
	errs = append(errs, validateLocalIdentityConstraintResolution(sch, index)...)
	errs = append(errs, validateAttributeDeclarations(sch)...)
	errs = append(errs, validateTypeDefinitionReferences(sch)...)
	errs = append(errs, validateEnumerationFacetValuesWithIndex(sch, index)...)
	errs = append(errs, validateInlineTypeReferencesWithIndex(sch, index)...)
	errs = append(errs, validateComplexTypeReferences(sch)...)
	errs = append(errs, validateAttributeGroupReferencesInSchema(sch)...)
	errs = append(errs, validateLocalElementValueConstraints(sch, index)...)
	errs = append(errs, validateGroupReferencesInSchema(sch)...)

	if err := validateNoCyclicAttributeGroups(sch); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func collectElementReferencesInSchemaWithIndex(sch *parser.Schema, index *iterationIndex) []*types.ElementDecl {
	var elementRefsInContent []*types.ElementDecl

	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, traversal.CollectFromContent(ct.Content(), func(p types.Particle) (*types.ElementDecl, bool) {
				decl, ok := p.(*types.ElementDecl)
				return decl, ok && decl.IsReference
			})...)
		}
	}

	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, traversal.CollectFromContent(ct.Content(), func(p types.Particle) (*types.ElementDecl, bool) {
				decl, ok := p.(*types.ElementDecl)
				return decl, ok && decl.IsReference
			})...)
		}
	}

	for _, qname := range index.groupQNames {
		group := sch.Groups[qname]
		for _, particle := range group.Particles {
			if elem, ok := particle.(*types.ElementDecl); ok && elem.IsReference {
				elementRefsInContent = append(elementRefsInContent, elem)
			} else if mg, ok := particle.(*types.ModelGroup); ok {
				elementRefsInContent = append(elementRefsInContent, traversal.CollectFromParticlesWithVisited(mg.Particles, make(map[*types.ModelGroup]bool), func(p types.Particle) (*types.ElementDecl, bool) {
					decl, ok := p.(*types.ElementDecl)
					return decl, ok && decl.IsReference
				})...)
			}
		}
	}

	return elementRefsInContent
}
