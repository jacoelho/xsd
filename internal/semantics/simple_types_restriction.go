package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateRestriction validates a simple type restriction
func validateRestriction(schema *parser.Schema, st *model.SimpleType, restriction *model.Restriction) error {
	baseType := resolveRestrictionBaseType(schema, st, restriction)

	baseQName := restriction.Base
	if baseQName.IsZero() && !model.IsNilType(baseType) {
		// for inline simpleType bases, use the base type's QName
		baseQName = baseType.Name()
	}

	if err := validateRestrictionBaseType(baseType, baseQName); err != nil {
		return err
	}

	facetList, err := buildRestrictionFacetList(restriction.Facets, baseType, baseQName)
	if err != nil {
		return err
	}
	if err := ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
	); err != nil {
		return err
	}
	if err := validateSchemaEnumerationValues(schema, facetList, baseType); err != nil {
		return err
	}

	// validate facet inheritance (A9)
	if !model.IsNilType(baseType) {
		if err := validateFacetInheritance(facetList, baseType); err != nil {
			return err
		}
	}

	if st.Variety() == model.ListVariety && st.WhiteSpace() != model.WhiteSpaceCollapse {
		return fmt.Errorf("list whiteSpace facet must be 'collapse'")
	}

	// validate whiteSpace restriction: derived type can only be stricter, not relaxed
	// order of restrictiveness: preserve < replace < collapse
	if err := validateWhiteSpaceRestriction(st, baseType, baseQName); err != nil {
		return err
	}
	return validateNotationRestriction(schema, facetList, baseType, baseQName)
}

func resolveRestrictionBaseType(schema *parser.Schema, st *model.SimpleType, restriction *model.Restriction) model.Type {
	switch {
	case !model.IsNilType(st.ResolvedBase):
		return st.ResolvedBase
	case !model.IsNilType(restriction.SimpleType):
		return restriction.SimpleType
	case restriction.Base.IsZero():
		return nil
	case restriction.Base.Namespace == model.XSDNamespace:
		return model.GetBuiltin(model.TypeName(restriction.Base.Local))
	default:
		defType, ok := LookupType(schema, restriction.Base)
		if !ok {
			return nil
		}
		if model.IsNilType(defType) {
			return nil
		}
		return defType
	}
}

func validateRestrictionBaseType(baseType model.Type, baseQName model.QName) error {
	if baseQName.Namespace == model.XSDNamespace && baseQName.Local == string(model.TypeNameAnyType) {
		return fmt.Errorf("simpleType restriction cannot have base type anyType")
	}
	if baseQName.Namespace == model.XSDNamespace && baseQName.Local == string(model.TypeNameAnySimpleType) {
		return fmt.Errorf("simpleType restriction cannot have base type anySimpleType")
	}
	if _, isComplex := baseType.(*model.ComplexType); isComplex {
		return fmt.Errorf("simpleType restriction cannot have complex base type '%s'", baseQName)
	}
	return nil
}

func buildRestrictionFacetList(facetsRaw []any, baseType model.Type, baseQName model.QName) ([]model.Facet, error) {
	facetList, deferredFacets := splitRestrictionFacets(facetsRaw)
	for _, df := range deferredFacets {
		if err := validateDeferredFacetApplicability(df, baseType, baseQName); err != nil {
			return nil, err
		}
	}
	for _, df := range deferredFacets {
		resolvedFacet, err := convertDeferredFacet(df, baseType)
		if err != nil {
			return nil, err
		}
		if resolvedFacet != nil {
			facetList = append(facetList, resolvedFacet)
		}
	}
	return facetList, nil
}

func splitRestrictionFacets(facetsRaw []any) ([]model.Facet, []*model.DeferredFacet) {
	facetList := make([]model.Facet, 0, len(facetsRaw))
	var deferredFacets []*model.DeferredFacet
	for _, rawFacet := range facetsRaw {
		switch facet := rawFacet.(type) {
		case model.Facet:
			facetList = append(facetList, facet)
		case *model.DeferredFacet:
			deferredFacets = append(deferredFacets, facet)
		}
	}
	return facetList, deferredFacets
}

func validateNotationRestriction(schema *parser.Schema, facetList []model.Facet, baseType model.Type, baseQName model.QName) error {
	if isDirectNotationRestriction(baseQName) {
		if !hasEnumerationFacet(facetList) {
			return fmt.Errorf("NOTATION restriction must have enumeration facet")
		}
		return validateNotationEnumeration(schema, facetList)
	}
	if !hasEnumerationFacet(facetList) || !restrictionBaseIsNotation(schema, baseType, baseQName) {
		return nil
	}
	return validateNotationEnumeration(schema, facetList)
}

func isDirectNotationRestriction(baseQName model.QName) bool {
	return !baseQName.IsZero() &&
		baseQName.Namespace == model.XSDNamespace &&
		baseQName.Local == string(model.TypeNameNOTATION)
}

func restrictionBaseIsNotation(schema *parser.Schema, baseType model.Type, baseQName model.QName) bool {
	if baseType != nil {
		return isNotationType(baseType)
	}
	if baseQName.IsZero() {
		return false
	}
	defType, ok := LookupType(schema, baseQName)
	return ok && isNotationType(defType)
}

// validateSimpleContentRestrictionFacets validates facets in a simpleContent restriction
func validateSimpleContentRestrictionFacets(schema *parser.Schema, restriction *model.Restriction) error {
	if restriction == nil {
		return nil
	}

	baseType, baseQName := resolveSimpleContentBaseTypeQName(schema, restriction.Base)
	if baseQName.IsZero() {
		baseQName = restriction.Base
	}

	if baseQName.Namespace == model.XSDNamespace && baseQName.Local == string(model.TypeNameAnyType) {
		return fmt.Errorf("simpleContent restriction cannot have base type anyType")
	}
	if baseQName.Namespace == model.XSDNamespace && baseQName.Local == string(model.TypeNameAnySimpleType) {
		if len(restriction.Facets) > 0 {
			return fmt.Errorf("simpleContent restriction cannot apply facets to base type anySimpleType")
		}
	}

	if _, isComplex := baseType.(*model.ComplexType); isComplex {
		return fmt.Errorf("simpleContent restriction cannot have complex base type '%s'", baseQName)
	}

	// convert facets to []model.Facet for validation
	facetList := make([]model.Facet, 0, len(restriction.Facets))
	var deferredFacets []*model.DeferredFacet
	for _, f := range restriction.Facets {
		switch facet := f.(type) {
		case model.Facet:
			facetList = append(facetList, facet)
		case *model.DeferredFacet:
			deferredFacets = append(deferredFacets, facet)
		}
	}

	for _, df := range deferredFacets {
		if err := validateDeferredFacetApplicability(df, baseType, baseQName); err != nil {
			return err
		}
	}

	if err := ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
	); err != nil {
		return err
	}
	if err := validateSchemaEnumerationValues(schema, facetList, baseType); err != nil {
		return err
	}

	if baseType != nil {
		if baseST, ok := baseType.(*model.SimpleType); ok {
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
