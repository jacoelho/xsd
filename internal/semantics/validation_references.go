package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
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

func validateTopLevelElementReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if decl.IsReference {
			refDecl, exists := sch.ElementDecls[decl.Name]
			if !exists {
				errs = append(errs, fmt.Errorf("element reference %s does not exist", decl.Name))
			} else if refDecl.IsReference {
				errs = append(errs, fmt.Errorf("element reference %s points to another reference %s (circular or invalid)", qname, decl.Name))
			}
		}
	}

	return errs
}

func validateContentElementReferences(sch *parser.Schema, elementRefsInContent []*model.ElementDecl) []error {
	var errs []error

	for _, elemRef := range elementRefsInContent {
		refDecl, exists := sch.ElementDecls[elemRef.Name]
		if !exists {
			errs = append(errs, fmt.Errorf("element reference %s in content model does not exist", elemRef.Name))
		} else if refDecl.IsReference {
			errs = append(errs, fmt.Errorf("element reference %s in content model points to another reference (circular or invalid)", elemRef.Name))
		}
	}

	return errs
}

func validateElementDeclarationReferences(sch *parser.Schema, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if decl.Type != nil {
			origin := sch.ElementOrigins[qname]
			if err := validateTypeReferenceFromTypeAtLocation(sch, decl.Type, qname.Namespace, origin); err != nil {
				errs = append(errs, fmt.Errorf("element %s: %w", qname, err))
			}
		}

		if err := validateElementValueConstraints(sch, decl); err != nil {
			errs = append(errs, fmt.Errorf("element %s: %w", qname, err))
		}

		errs = append(errs, validateElementSubstitutionGroupReferences(sch, qname, decl)...)
		errs = append(errs, validateElementIdentityConstraintReferences(sch, qname, decl, allConstraints)...)
	}

	return errs
}

func validateElementSubstitutionGroupReferences(sch *parser.Schema, qname model.QName, decl *model.ElementDecl) []error {
	if decl.SubstitutionGroup == (model.QName{}) {
		return nil
	}

	headDecl, exists := sch.ElementDecls[decl.SubstitutionGroup]
	if !exists {
		return []error{
			fmt.Errorf("element %s substitutionGroup %s does not exist", qname, decl.SubstitutionGroup),
		}
	}

	var errs []error
	if err := validateSubstitutionGroupDerivation(sch, qname, decl, headDecl); err != nil {
		errs = append(errs, err)
	}
	if err := validateSubstitutionGroupFinal(sch, qname, decl, headDecl); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func validateElementIdentityConstraintReferences(sch *parser.Schema, qname model.QName, decl *model.ElementDecl, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	if err := ValidateKeyrefConstraints(qname, decl.Constraints, allConstraints); err != nil {
		errs = append(errs, err...)
	}

	for _, constraint := range decl.Constraints {
		if err := ValidateIdentityConstraintResolution(sch, constraint, decl); err != nil {
			errs = append(errs, fmt.Errorf("element %s identity constraint '%s': %w", qname, constraint.Name, err))
		}
	}

	return errs
}

func validateGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.Groups) {
		group := sch.Groups[qname]
		if err := validateGroupReferences(sch, qname, group); err != nil {
			errs = append(errs, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	return errs
}

func validateAttributeGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.AttributeGroups) {
		ag := sch.AttributeGroups[qname]
		errs = append(errs, validateAttributeGroupReferences(sch, qname, ag)...)
		errs = append(errs, validateAttributeGroupAttributeReferences(sch, qname, ag)...)
		errs = append(errs, validateAttributeGroupAttributeTypes(sch, qname, ag)...)
	}

	return errs
}

func validateAttributeGroupReferences(sch *parser.Schema, qname model.QName, ag *model.AttributeGroup) []error {
	var errs []error
	for _, agRef := range ag.AttrGroups {
		if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateAttributeGroupAttributeReferences(sch *parser.Schema, qname model.QName, ag *model.AttributeGroup) []error {
	var errs []error
	for _, attr := range ag.Attributes {
		if !attr.IsReference {
			continue
		}
		if err := validateAttributeReference(sch, qname, attr, "attributeGroup"); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateAttributeGroupAttributeTypes(sch *parser.Schema, qname model.QName, ag *model.AttributeGroup) []error {
	var errs []error
	for _, attr := range ag.Attributes {
		if attr.Type == nil {
			continue
		}
		if err := validateTypeReferenceFromTypeAtLocation(sch, attr.Type, qname.Namespace, noOriginLocation); err != nil {
			errs = append(errs, fmt.Errorf("attributeGroup %s attribute %s: %w", qname, attr.Name, err))
		}
	}
	return errs
}

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

func validateComplexTypeReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*model.ComplexType)
		if !ok {
			continue
		}
		errs = append(errs, validateComplexTypeAttributeReferences(sch, qname, ct)...)
		if err := validateComplexTypeParticleReferences(sch, qname, ct, sch.TypeOrigins[qname]); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func validateComplexTypeAttributeReferences(sch *parser.Schema, qname model.QName, ct *model.ComplexType) []error {
	var errs []error

	for _, agRef := range collectComplexTypeAttrGroupRefs(ct) {
		if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
			errs = append(errs, err)
		}
	}

	for _, attr := range ct.Attributes() {
		if attr.IsReference {
			if err := validateAttributeReference(sch, qname, attr, "type"); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if attr.Type == nil {
			continue
		}
		if err := validateTypeReferenceFromTypeAtLocation(sch, attr.Type, qname.Namespace, noOriginLocation); err != nil {
			errs = append(errs, fmt.Errorf("type %s attribute: %w", qname, err))
		}
	}

	return errs
}

func validateComplexTypeParticleReferences(sch *parser.Schema, qname model.QName, ct *model.ComplexType, origin string) error {
	if err := model.WalkContentParticles(ct.Content(), func(particle model.Particle) error {
		return validateParticleReferences(sch, particle, origin)
	}); err != nil {
		return fmt.Errorf("type %s: %w", qname, err)
	}

	return nil
}

func collectComplexTypeAttrGroupRefs(ct *model.ComplexType) []model.QName {
	if ct == nil {
		return nil
	}
	out := make([]model.QName, 0, len(ct.AttrGroups))
	out = append(out, ct.AttrGroups...)

	if cc, ok := ct.Content().(*model.ComplexContent); ok {
		if cc.Extension != nil {
			out = append(out, cc.Extension.AttrGroups...)
		}
		if cc.Restriction != nil {
			out = append(out, cc.Restriction.AttrGroups...)
		}
	}
	if sc, ok := ct.Content().(*model.SimpleContent); ok {
		if sc.Extension != nil {
			out = append(out, sc.Extension.AttrGroups...)
		}
	}
	return out
}

func validateLocalElementValueConstraints(sch *parser.Schema, index *iterationIndex) []error {
	if index == nil {
		index = buildIterationIndex(sch)
	}

	seenLocal := make(map[*model.ElementDecl]bool)

	var errs []error
	errs = append(errs, validateLocalElementValueConstraintsFromElements(sch, index, seenLocal)...)
	errs = append(errs, validateLocalElementValueConstraintsFromTypes(sch, index, seenLocal)...)
	return errs
}

func validateLocalElementValueConstraintsFromElements(sch *parser.Schema, index *iterationIndex, seenLocal map[*model.ElementDecl]bool) []error {
	var errs []error
	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		ct, ok := decl.Type.(*model.ComplexType)
		if !ok {
			continue
		}
		errs = append(errs, validateLocalElementValueConstraintsInComplexType(sch, ct, seenLocal)...)
	}
	return errs
}

func validateLocalElementValueConstraintsFromTypes(sch *parser.Schema, index *iterationIndex, seenLocal map[*model.ElementDecl]bool) []error {
	var errs []error
	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*model.ComplexType)
		if !ok {
			continue
		}
		errs = append(errs, validateLocalElementValueConstraintsInComplexType(sch, ct, seenLocal)...)
	}
	return errs
}

func validateLocalElementValueConstraintsInComplexType(sch *parser.Schema, ct *model.ComplexType, seenLocal map[*model.ElementDecl]bool) []error {
	var errs []error
	for _, elem := range CollectElementDeclsFromComplexType(sch, ct) {
		if elem == nil || elem.IsReference {
			continue
		}
		if seenLocal[elem] {
			continue
		}
		seenLocal[elem] = true
		if err := validateElementValueConstraints(sch, elem); err != nil {
			errs = append(errs, fmt.Errorf("local element %s: %w", elem.Name, err))
		}
	}
	return errs
}

func validateLocalIdentityConstraintKeyrefsWithIndex(sch *parser.Schema, index *iterationIndex, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	forEachLocalConstraintElement(sch, index, func(elem *model.ElementDecl) {
		if err := ValidateKeyrefConstraints(elem.Name, elem.Constraints, allConstraints); err != nil {
			errs = append(errs, err...)
		}
	})

	return errs
}

func validateLocalIdentityConstraintResolution(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error

	forEachLocalConstraintElement(sch, index, func(elem *model.ElementDecl) {
		for _, constraint := range elem.Constraints {
			if err := ValidateIdentityConstraintResolution(sch, constraint, elem); err != nil {
				if errors.Is(err, runtime.ErrInvalidXPath) {
					continue
				}
				errs = append(errs, fmt.Errorf("element %s local identity constraint '%s': %w", elem.Name, constraint.Name, err))
			}
		}
	})

	return errs
}

func forEachLocalConstraintElement(sch *parser.Schema, index *iterationIndex, visit func(*model.ElementDecl)) {
	if sch == nil || visit == nil {
		return
	}
	if index == nil {
		index = buildIterationIndex(sch)
	}
	for _, elem := range index.localConstraintElems {
		visit(elem)
	}
}

func validateEnumerationFacetValuesWithIndex(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error
	if index == nil {
		index = buildIterationIndex(sch)
	}

	for _, qname := range index.typeQNames {
		st, ok := sch.TypeDefs[qname].(*model.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}
		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = parser.ResolveSimpleTypeReferenceAllowMissing(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}
		for _, facet := range st.Restriction.Facets {
			enum, ok := facet.(*model.Enumeration)
			if !ok {
				continue
			}
			values := enum.Values()
			contexts := enum.ValueContexts()
			for i, val := range values {
				var ctx map[string]string
				if i < len(contexts) {
					ctx = contexts[i]
				}
				if err := validateDefaultOrFixedValueResolved(sch, val, baseType, ctx, idValuesAllowed); err != nil {
					errs = append(errs, fmt.Errorf("type %s: restriction: enumeration value %d (%q) is not valid for base type %s: %w", qname, i+1, val, baseType.Name().Local, err))
				}
			}
		}
	}

	return errs
}

func validateInlineTypeReferencesWithIndex(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error
	if index == nil {
		index = buildIterationIndex(sch)
	}

	for _, qname := range index.elementQNames {
		errs = append(errs, validateInlineElementTypeReferences(sch, qname)...)
	}

	return errs
}

func validateInlineElementTypeReferences(sch *parser.Schema, qname model.QName) []error {
	decl := sch.ElementDecls[qname]
	if decl == nil || decl.Type == nil || decl.Type.IsBuiltin() {
		return nil
	}
	if _, exists := sch.TypeDefs[decl.Type.Name()]; exists {
		return nil
	}

	var errs []error
	if err := validateTypeReferences(sch, qname, decl.Type); err != nil {
		errs = append(errs, fmt.Errorf("element %s inline type: %w", qname, err))
	}
	if ct, ok := decl.Type.(*model.ComplexType); ok {
		errs = append(errs, validateInlineComplexTypeReferences(sch, qname, ct)...)
	}
	return errs
}

func validateInlineComplexTypeReferences(sch *parser.Schema, qname model.QName, ct *model.ComplexType) []error {
	var errs []error
	for _, agRef := range ct.AttrGroups {
		if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
			errs = append(errs, err)
		}
	}
	for _, attr := range ct.Attributes() {
		if !attr.IsReference {
			continue
		}
		if err := validateAttributeReference(sch, qname, attr, "element"); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
