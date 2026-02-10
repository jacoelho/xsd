package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func validateLengthFacetInheritance(derivedFacets []model.Facet, baseType *model.SimpleType) error {
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

func facetsFromRestriction(restriction *model.Restriction) []model.Facet {
	if restriction == nil {
		return nil
	}
	result := make([]model.Facet, 0, len(restriction.Facets))
	for _, f := range restriction.Facets {
		if facet, ok := f.(model.Facet); ok {
			result = append(result, facet)
		}
	}
	return result
}

func findIntFacet(facetList []model.Facet, name string) (int, bool) {
	for _, facet := range facetList {
		if facet.Name() != name {
			continue
		}
		if iv, ok := facet.(model.IntValueFacet); ok {
			return iv.GetIntValue(), true
		}
	}
	return 0, false
}
