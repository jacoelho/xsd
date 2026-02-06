package resolver

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

func validateReferences(sch *parser.Schema) []error {
	var errs []error

	elementRefsInContent := collectElementReferencesInSchema(sch)
	allConstraints := collectAllIdentityConstraints(sch)

	// per XSD spec 3.11.2: "Constraint definition identities must be unique within an XML Schema"
	// constraints are identified by (name, target namespace)
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
	errs = append(errs, validateDeferredRangeFacetValues(sch)...)
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

// ValidateReferences exposes reference validation for schema loading.
func ValidateReferences(sch *parser.Schema) []error {
	return validateReferences(sch)
}

func collectElementReferencesInSchema(sch *parser.Schema) []*types.ElementDecl {
	var elementRefsInContent []*types.ElementDecl

	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, collectElementReferences(ct.Content())...)
		}
	}

	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, collectElementReferences(ct.Content())...)
		}
	}

	for _, qname := range schema.SortedQNames(sch.Groups) {
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

func validateTopLevelElementReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
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

func validateContentElementReferences(sch *parser.Schema, elementRefsInContent []*types.ElementDecl) []error {
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

func validateElementDeclarationReferences(sch *parser.Schema, allConstraints []*types.IdentityConstraint) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
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

		if decl.SubstitutionGroup != (types.QName{}) {
			headDecl, exists := sch.ElementDecls[decl.SubstitutionGroup]
			if !exists {
				errs = append(errs, fmt.Errorf("element %s substitutionGroup %s does not exist", qname, decl.SubstitutionGroup))
				continue
			}
			if err := validateSubstitutionGroupDerivation(sch, qname, decl, headDecl); err != nil {
				errs = append(errs, err)
			}
			if err := validateSubstitutionGroupFinal(sch, qname, decl, headDecl); err != nil {
				errs = append(errs, err)
			}
		}

		if err := validateKeyrefConstraints(qname, decl.Constraints, allConstraints); err != nil {
			errs = append(errs, err...)
		}

		for _, constraint := range decl.Constraints {
			if err := validateIdentityConstraintResolution(sch, constraint, decl); err != nil {
				errs = append(errs, fmt.Errorf("element %s identity constraint '%s': %w", qname, constraint.Name, err))
			}
		}
	}

	return errs
}

func validateLocalIdentityConstraintKeyrefs(sch *parser.Schema, allConstraints []*types.IdentityConstraint) []error {
	var errs []error

	forEachLocalConstraintElement(sch, func(elem *types.ElementDecl) {
		if err := validateKeyrefConstraints(elem.Name, elem.Constraints, allConstraints); err != nil {
			errs = append(errs, err...)
		}
	})

	return errs
}

func validateLocalIdentityConstraintResolution(sch *parser.Schema) []error {
	var errs []error

	forEachLocalConstraintElement(sch, func(elem *types.ElementDecl) {
		for _, constraint := range elem.Constraints {
			if err := validateIdentityConstraintResolution(sch, constraint, elem); err != nil {
				if errors.Is(err, xpath.ErrInvalidXPath) {
					continue
				}
				errs = append(errs, fmt.Errorf("element %s local identity constraint '%s': %w", elem.Name, constraint.Name, err))
			}
		}
	})

	return errs
}

func forEachLocalConstraintElement(sch *parser.Schema, visit func(*types.ElementDecl)) {
	if sch == nil || visit == nil {
		return
	}
	seen := make(map[*types.ElementDecl]bool)
	validateLocals := func(ct *types.ComplexType) {
		for _, elem := range collectConstraintElementsFromContent(ct.Content()) {
			if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
				continue
			}
			if seen[elem] {
				continue
			}
			seen[elem] = true
			visit(elem)
		}
	}

	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}
	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}
}

func validateAttributeDeclarations(sch *parser.Schema) []error {
	var errs []error

	// note: Attribute references are stored in complex types, not as top-level declarations
	// we validate attribute type references when validating complex types
	for _, qname := range schema.SortedQNames(sch.AttributeDecls) {
		decl := sch.AttributeDecls[qname]
		if decl.Type != nil {
			if err := validateTypeReferenceFromType(sch, decl.Type, qname.Namespace); err != nil {
				errs = append(errs, fmt.Errorf("attribute %s: %w", qname, err))
			}
		}

		// validate default/fixed values against the resolved type (including facets)
		// this is done here after type resolution because during structure validation
		// the type might be a placeholder and facets might not be available
		resolvedType := resolveTypeForFinalValidation(sch, decl.Type)
		if _, ok := resolvedType.(*types.ComplexType); ok {
			errs = append(errs, fmt.Errorf("attribute %s: type must be a simple type", qname))
		}
		if decl.HasDefault {
			if err := validateDefaultOrFixedValueWithResolvedType(sch, decl.Default, resolvedType, decl.DefaultContext); err != nil {
				errs = append(errs, fmt.Errorf("attribute %s: invalid default value '%s': %w", qname, decl.Default, err))
			}
		}
		if decl.HasFixed {
			if err := validateDefaultOrFixedValueWithResolvedType(sch, decl.Fixed, resolvedType, decl.FixedContext); err != nil {
				errs = append(errs, fmt.Errorf("attribute %s: invalid fixed value '%s': %w", qname, decl.Fixed, err))
			}
		}
	}

	return errs
}

func validateTypeDefinitionReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if err := validateTypeReferences(sch, qname, typ); err != nil {
			errs = append(errs, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errs
}

func validateEnumerationFacetValues(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		st, ok := sch.TypeDefs[qname].(*types.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}
		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = typeops.ResolveSimpleTypeReference(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}
		for _, facet := range st.Restriction.Facets {
			enum, ok := facet.(*types.Enumeration)
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
				if err := validateDefaultOrFixedValueResolved(sch, val, baseType, ctx, make(map[types.Type]bool), idValuesAllowed); err != nil {
					errs = append(errs, fmt.Errorf("type %s: restriction: enumeration value %d (%q) is not valid for base type %s: %w", qname, i+1, val, baseType.Name().Local, err))
				}
			}
		}
	}

	return errs
}

func validateDeferredRangeFacetValues(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		st, ok := sch.TypeDefs[qname].(*types.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}

		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = typeops.ResolveSimpleTypeReference(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}

		var (
			rangeFacets  []types.Facet
			seenDeferred bool
		)

		for _, facet := range st.Restriction.Facets {
			switch f := facet.(type) {
			case types.Facet:
				if isRangeFacetName(f.Name()) {
					rangeFacets = append(rangeFacets, f)
				}
			case *types.DeferredFacet:
				if !isRangeFacetName(f.FacetName) {
					continue
				}
				seenDeferred = true
				resolved, err := convertDeferredRangeFacetForValidation(f, baseType)
				if err != nil {
					errs = append(errs, fmt.Errorf("type %s: restriction: %w", qname, err))
					continue
				}
				if resolved != nil {
					rangeFacets = append(rangeFacets, resolved)
				}
			}
		}

		if !seenDeferred || len(rangeFacets) == 0 {
			continue
		}

		baseQName := st.Restriction.Base
		if baseQName.IsZero() {
			baseQName = baseType.Name()
		}
		if err := schemacheck.ValidateFacetConstraints(sch, rangeFacets, baseType, baseQName); err != nil {
			errs = append(errs, fmt.Errorf("type %s: restriction: %w", qname, err))
		}
	}

	return errs
}

func isRangeFacetName(name string) bool {
	switch name {
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
		return true
	default:
		return false
	}
}

func convertDeferredRangeFacetForValidation(df *types.DeferredFacet, baseType types.Type) (types.Facet, error) {
	if df == nil || baseType == nil {
		return nil, nil
	}

	switch df.FacetName {
	case "minInclusive":
		return types.NewMinInclusive(df.FacetValue, baseType)
	case "maxInclusive":
		return types.NewMaxInclusive(df.FacetValue, baseType)
	case "minExclusive":
		return types.NewMinExclusive(df.FacetValue, baseType)
	case "maxExclusive":
		return types.NewMaxExclusive(df.FacetValue, baseType)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", df.FacetName)
	}
}

func validateInlineTypeReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if decl.Type != nil && !decl.Type.IsBuiltin() {
			// skip if the type is a reference to a named type (already validated above)
			if _, exists := sch.TypeDefs[decl.Type.Name()]; !exists {
				if err := validateTypeReferences(sch, qname, decl.Type); err != nil {
					errs = append(errs, fmt.Errorf("element %s inline type: %w", qname, err))
				}
				// also validate attribute group references for inline complex types
				if ct, ok := decl.Type.(*types.ComplexType); ok {
					for _, agRef := range ct.AttrGroups {
						if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
							errs = append(errs, err)
						}
					}
					for _, attr := range ct.Attributes() {
						if attr.IsReference {
							if err := validateAttributeReference(sch, qname, attr, "element"); err != nil {
								errs = append(errs, err)
							}
						}
					}
				}
			}
		}
	}

	return errs
}

func validateComplexTypeReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*types.ComplexType)
		if !ok {
			continue
		}
		for _, agRef := range ct.AttrGroups {
			if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
				errs = append(errs, err)
			}
		}

		if cc, ok := ct.Content().(*types.ComplexContent); ok {
			if cc.Extension != nil {
				for _, agRef := range cc.Extension.AttrGroups {
					if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
						errs = append(errs, err)
					}
				}
			}
			if cc.Restriction != nil {
				for _, agRef := range cc.Restriction.AttrGroups {
					if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
		if sc, ok := ct.Content().(*types.SimpleContent); ok {
			if sc.Extension != nil {
				for _, agRef := range sc.Extension.AttrGroups {
					if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}

		for _, attr := range ct.Attributes() {
			if attr.IsReference {
				if err := validateAttributeReference(sch, qname, attr, "type"); err != nil {
					errs = append(errs, err)
				}
			} else if attr.Type != nil {
				if err := validateTypeReferenceFromType(sch, attr.Type, qname.Namespace); err != nil {
					errs = append(errs, fmt.Errorf("type %s attribute: %w", qname, err))
				}
			}
		}

		origin := sch.TypeOrigins[qname]
		if err := validateContentReferences(sch, ct.Content(), origin); err != nil {
			errs = append(errs, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errs
}

func validateAttributeGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.AttributeGroups) {
		ag := sch.AttributeGroups[qname]
		for _, agRef := range ag.AttrGroups {
			if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
				errs = append(errs, err)
			}
		}

		for _, attr := range ag.Attributes {
			if attr.IsReference {
				if err := validateAttributeReference(sch, qname, attr, "attributeGroup"); err != nil {
					errs = append(errs, err)
				}
			}
		}

		for _, attr := range ag.Attributes {
			if attr.Type != nil {
				if err := validateTypeReferenceFromType(sch, attr.Type, qname.Namespace); err != nil {
					errs = append(errs, fmt.Errorf("attributeGroup %s attribute %s: %w", qname, attr.Name, err))
				}
			}
		}
	}

	return errs
}

func validateLocalElementValueConstraints(sch *parser.Schema) []error {
	var errs []error

	seenLocal := make(map[*types.ElementDecl]bool)
	validateLocals := func(ct *types.ComplexType) {
		for _, elem := range schemacheck.CollectAllElementDeclarationsFromType(sch, ct) {
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
	}
	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}
	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}

	return errs
}

func validateGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range schema.SortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		if err := validateGroupReferences(sch, qname, group); err != nil {
			errs = append(errs, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	return errs
}
