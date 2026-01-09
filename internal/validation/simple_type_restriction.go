package validation

import (
	"fmt"

	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateRestriction validates a simple type restriction
func validateRestriction(schema *schema.Schema, st *types.SimpleType, restriction *types.Restriction) error {
	var baseType types.Type

	// use ResolvedBase if available (handles inline simpleType bases)
	if st.ResolvedBase != nil {
		baseType = st.ResolvedBase
	} else if !restriction.Base.IsZero() {
		// fall back to resolving from QName if ResolvedBase is not set
		baseTypeName := restriction.Base.Local

		// check if it's a built-in type
		if restriction.Base.Namespace == types.XSDNamespace {
			bt := types.GetBuiltin(types.TypeName(baseTypeName))
			if bt == nil {
				// unknown built-in type - might be a forward reference issue, skip for now
				baseType = nil
			} else {
				baseType = bt
			}
		} else {
			// check if it's a user-defined type in this schema
			if defType, ok := schema.TypeDefs[restriction.Base]; ok {
				baseType = defType
			}
		}
	}

	// convert facets to []types.Facet for validation
	// also process deferred facets (range facets that couldn't be constructed during parsing)
	facetList := make([]types.Facet, 0, len(restriction.Facets))
	var deferredFacets []*types.DeferredFacet
	for _, f := range restriction.Facets {
		if facet, ok := f.(types.Facet); ok {
			facetList = append(facetList, facet)
		} else if df, ok := f.(*types.DeferredFacet); ok {
			deferredFacets = append(deferredFacets, df)
		}
	}

	baseQName := restriction.Base
	if baseQName.IsZero() && baseType != nil {
		// for inline simpleType bases, use the base type's QName
		baseQName = baseType.Name()
	}

	// simple type restrictions must have a simple type base.
	// anyType is a complex type and cannot be restricted by a simpleType.
	if baseQName.Namespace == types.XSDNamespace && baseQName.Local == string(types.TypeNameAnyType) {
		return fmt.Errorf("simpleType restriction cannot have base type anyType")
	}

	// per XSD 1.0 tests: anySimpleType cannot be used as a restriction base in schema definitions.
	if baseQName.Namespace == types.XSDNamespace && baseQName.Local == string(types.TypeNameAnySimpleType) {
		return fmt.Errorf("simpleType restriction cannot have base type anySimpleType")
	}

	if _, isComplex := baseType.(*types.ComplexType); isComplex {
		return fmt.Errorf("simpleType restriction cannot have complex base type '%s'", baseQName)
	}

	// validate deferred facets - check applicability now that base type is resolved
	for _, df := range deferredFacets {
		if err := validateDeferredFacetApplicability(df, baseType, baseQName); err != nil {
			return err
		}
	}

	// convert deferred facets to actual facets now that base type is resolved
	// this is needed for facet inheritance validation
	for _, df := range deferredFacets {
		resolvedFacet, err := convertDeferredFacet(df, baseType)
		if err != nil {
			// if we can't convert, the facet will just not be validated for inheritance
			// this is a fallback - most cases should be handled above
			continue
		}
		if resolvedFacet != nil {
			facetList = append(facetList, resolvedFacet)
		}
	}

	if err := validateFacetConstraints(facetList, baseType, baseQName); err != nil {
		return err
	}

	// validate facet inheritance (A9)
	if baseType != nil {
		if err := validateFacetInheritance(facetList, baseType); err != nil {
			return err
		}
	}

	// validate whiteSpace restriction: derived type can only be stricter, not relaxed
	// order of restrictiveness: preserve < replace < collapse
	if err := validateWhiteSpaceRestriction(st, baseType, baseQName); err != nil {
		return err
	}

	// XSD 1.0 spec: NOTATION type cannot be used directly; must have enumeration facet
	// however, if restricting a NOTATION-derived type that already has enumeration, additional
	// restrictions (like length facets) are allowed without re-specifying enumeration.
	isDirectNotation := !baseQName.IsZero() &&
		baseQName.Namespace == types.XSDNamespace &&
		baseQName.Local == string(types.TypeNameNOTATION)
	if isDirectNotation {
		// directly restricting xs:NOTATION - must have enumeration in this restriction
		if !hasEnumerationFacet(facetList) {
			return fmt.Errorf("NOTATION restriction must have enumeration facet")
		}
		if err := validateNotationEnumeration(schema, facetList, schema.TargetNamespace); err != nil {
			return err
		}
	} else if hasEnumerationFacet(facetList) {
		// if this restriction adds enumeration facets, validate them against declared notations
		// (if the base type is NOTATION-derived)
		isNotation := false
		if baseType != nil {
			isNotation = isNotationType(baseType)
		} else if !baseQName.IsZero() {
			if defType, ok := schema.TypeDefs[baseQName]; ok {
				isNotation = isNotationType(defType)
			}
		}
		if isNotation {
			if err := validateNotationEnumeration(schema, facetList, schema.TargetNamespace); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateSimpleContentRestrictionFacets validates facets in a simpleContent restriction
func validateSimpleContentRestrictionFacets(schema *schema.Schema, restriction *types.Restriction) error {
	if restriction == nil {
		return nil
	}

	baseType, baseQName := resolveSimpleContentBaseType(schema, restriction.Base)
	if baseQName.IsZero() {
		baseQName = restriction.Base
	}

	if baseQName.Namespace == types.XSDNamespace && baseQName.Local == string(types.TypeNameAnyType) {
		return fmt.Errorf("simpleContent restriction cannot have base type anyType")
	}
	if baseQName.Namespace == types.XSDNamespace && baseQName.Local == string(types.TypeNameAnySimpleType) {
		if len(restriction.Facets) > 0 {
			return fmt.Errorf("simpleContent restriction cannot apply facets to base type anySimpleType")
		}
	}

	if _, isComplex := baseType.(*types.ComplexType); isComplex {
		return fmt.Errorf("simpleContent restriction cannot have complex base type '%s'", baseQName)
	}

	// convert facets to []types.Facet for validation
	facetList := make([]types.Facet, 0, len(restriction.Facets))
	var deferredFacets []*types.DeferredFacet
	for _, f := range restriction.Facets {
		if facet, ok := f.(types.Facet); ok {
			facetList = append(facetList, facet)
		} else if df, ok := f.(*types.DeferredFacet); ok {
			deferredFacets = append(deferredFacets, df)
		}
	}

	for _, df := range deferredFacets {
		if err := validateDeferredFacetApplicability(df, baseType, baseQName); err != nil {
			return err
		}
	}

	if err := validateFacetConstraints(facetList, baseType, baseQName); err != nil {
		return err
	}

	if baseType != nil {
		if baseST, ok := baseType.(*types.SimpleType); ok {
			if err := validateFacetInheritance(facetList, baseST); err != nil {
				return err
			}
			if err := validateLengthFacetInheritance(facetList, baseST); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateLengthFacetInheritance(derivedFacets []types.Facet, baseType *types.SimpleType) error {
	if baseType == nil || baseType.Restriction == nil {
		return nil
	}
	baseFacets := facetsFromRestriction(baseType.Restriction)

	baseLength, hasBaseLength := findIntFacet(baseFacets, "length")
	baseMin, hasBaseMin := findIntFacet(baseFacets, "minLength")
	baseMax, hasBaseMax := findIntFacet(baseFacets, "maxLength")

	derivedLength, hasDerivedLength := findIntFacet(derivedFacets, "length")
	derivedMin, hasDerivedMin := findIntFacet(derivedFacets, "minLength")
	derivedMax, hasDerivedMax := findIntFacet(derivedFacets, "maxLength")

	if hasBaseLength {
		if hasDerivedLength && derivedLength != baseLength {
			return fmt.Errorf("facet length: derived value (%d) must equal base value (%d) in a restriction", derivedLength, baseLength)
		}
		if hasDerivedMin && derivedMin != baseLength {
			return fmt.Errorf("facet minLength: derived value (%d) must equal base length (%d) in a restriction", derivedMin, baseLength)
		}
		if hasDerivedMax && derivedMax != baseLength {
			return fmt.Errorf("facet maxLength: derived value (%d) must equal base length (%d) in a restriction", derivedMax, baseLength)
		}
		return nil
	}

	if hasBaseMin && hasDerivedMin && derivedMin < baseMin {
		return fmt.Errorf("facet minLength: derived value (%d) must be >= base value (%d) to be a valid restriction", derivedMin, baseMin)
	}
	if hasBaseMax && hasDerivedMax && derivedMax > baseMax {
		return fmt.Errorf("facet maxLength: derived value (%d) must be <= base value (%d) to be a valid restriction", derivedMax, baseMax)
	}

	if hasBaseMin && hasDerivedLength && derivedLength < baseMin {
		return fmt.Errorf("facet length: derived value (%d) must be >= base minLength (%d) to be a valid restriction", derivedLength, baseMin)
	}
	if hasBaseMax && hasDerivedLength && derivedLength > baseMax {
		return fmt.Errorf("facet length: derived value (%d) must be <= base maxLength (%d) to be a valid restriction", derivedLength, baseMax)
	}

	return nil
}

func facetsFromRestriction(restriction *types.Restriction) []types.Facet {
	if restriction == nil {
		return nil
	}
	result := make([]types.Facet, 0, len(restriction.Facets))
	for _, f := range restriction.Facets {
		if facet, ok := f.(types.Facet); ok {
			result = append(result, facet)
		}
	}
	return result
}

func findIntFacet(facetList []types.Facet, name string) (int, bool) {
	for _, facet := range facetList {
		if facet.Name() != name {
			continue
		}
		if iv, ok := facet.(types.IntValueFacet); ok {
			return iv.GetIntValue(), true
		}
	}
	return 0, false
}
