package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// ValidateReferences validates cross-component references for schema loading.
func ValidateReferences(sch *parser.Schema) []error {
	if sch == nil {
		return []error{fmt.Errorf("nil schema")}
	}
	var errs []error
	index := buildIterationIndex(sch)

	elementRefsInContent := index.elementRefsInContent
	allConstraints := index.allIdentityConstraints

	if uniquenessErrors := ValidateIdentityConstraintUniqueness(allConstraints); len(uniquenessErrors) > 0 {
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

func collectElementReferencesInSchemaWithIndex(sch *parser.Schema, index *iterationIndex) []*model.ElementDecl {
	var elementRefsInContent []*model.ElementDecl

	elementRefsInContent = append(elementRefsInContent, collectElementReferencesFromElementDecls(sch, index)...)
	elementRefsInContent = append(elementRefsInContent, collectElementReferencesFromTypeDefs(sch, index)...)
	elementRefsInContent = append(elementRefsInContent, collectElementReferencesFromGroups(sch, index)...)

	return elementRefsInContent
}

func collectElementReferencesFromElementDecls(sch *parser.Schema, index *iterationIndex) []*model.ElementDecl {
	var refs []*model.ElementDecl
	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			refs = append(refs, collectElementReferencesFromContent(ct.Content())...)
		}
	}

	return refs
}

func collectElementReferencesFromTypeDefs(sch *parser.Schema, index *iterationIndex) []*model.ElementDecl {
	var refs []*model.ElementDecl
	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			refs = append(refs, collectElementReferencesFromContent(ct.Content())...)
		}
	}

	return refs
}

func collectElementReferencesFromGroups(sch *parser.Schema, index *iterationIndex) []*model.ElementDecl {
	var refs []*model.ElementDecl
	for _, qname := range index.groupQNames {
		group := sch.Groups[qname]
		refs = append(refs, collectElementReferencesFromGroupParticles(group.Particles)...)
	}

	return refs
}

func collectElementReferencesFromContent(content model.Content) []*model.ElementDecl {
	return model.CollectFromContent(content, collectReferenceElementDecl)
}

func collectElementReferencesFromGroupParticles(particles []model.Particle) []*model.ElementDecl {
	var refs []*model.ElementDecl
	for _, particle := range particles {
		if elem, ok := particle.(*model.ElementDecl); ok && elem.IsReference {
			refs = append(refs, elem)
			continue
		}

		mg, ok := particle.(*model.ModelGroup)
		if !ok {
			continue
		}
		refs = append(refs, model.CollectFromParticlesWithVisited(mg.Particles, make(map[*model.ModelGroup]bool), collectReferenceElementDecl)...)
	}

	return refs
}

func collectReferenceElementDecl(p model.Particle) (*model.ElementDecl, bool) {
	decl, ok := p.(*model.ElementDecl)
	return decl, ok && decl.IsReference
}
