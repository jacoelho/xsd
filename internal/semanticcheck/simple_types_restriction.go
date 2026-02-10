package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
	"github.com/jacoelho/xsd/internal/typechain"
)

// validateRestriction validates a simple type restriction
func validateRestriction(schema *parser.Schema, st *model.SimpleType, restriction *model.Restriction) error {
	var baseType model.Type

	switch {
	// use ResolvedBase if available (after semantic resolution).
	case st.ResolvedBase != nil:
		baseType = st.ResolvedBase
	case restriction.SimpleType != nil:
		// inline simpleType base is available before resolution.
		baseType = restriction.SimpleType
	case !restriction.Base.IsZero():
		// fall back to resolving from QName if ResolvedBase is not set
		baseTypeName := restriction.Base.Local

		// check if it's a built-in type
		if restriction.Base.Namespace == model.XSDNamespace {
			bt := builtins.Get(builtins.TypeName(baseTypeName))
			if bt == nil {
				// unknown built-in type - might be a forward reference issue, skip for now
				baseType = nil
			} else {
				baseType = bt
			}
		} else {
			// check if it's a user-defined type in this schema
			if defType, ok := typechain.LookupType(schema, restriction.Base); ok {
				baseType = defType
			}
		}
	}

	// convert facets to []model.Facet for validation
	// also process deferred facets (range facets that couldn't be constructed during parsing)
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

	baseQName := restriction.Base
	if baseQName.IsZero() && baseType != nil {
		// for inline simpleType bases, use the base type's QName
		baseQName = baseType.Name()
	}

	// simple type restrictions must have a simple type base.
	// anyType is a complex type and cannot be restricted by a simpleType.
	if baseQName.Namespace == model.XSDNamespace && baseQName.Local == string(model.TypeNameAnyType) {
		return fmt.Errorf("simpleType restriction cannot have base type anyType")
	}

	// per XSD 1.0 tests: anySimpleType cannot be used as a restriction base in schema definitions.
	if baseQName.Namespace == model.XSDNamespace && baseQName.Local == string(model.TypeNameAnySimpleType) {
		return fmt.Errorf("simpleType restriction cannot have base type anySimpleType")
	}

	if _, isComplex := baseType.(*model.ComplexType); isComplex {
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
			return err
		}
		if resolvedFacet != nil {
			facetList = append(facetList, resolvedFacet)
		}
	}

	if err := facetengine.ValidateSchemaConstraints(
		facetengine.SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
		facetengine.SchemaConstraintCallbacks{
			ValidateRangeConsistency: facetengine.ValidateRangeConsistency,
			ValidateRangeValues:      facetengine.ValidateRangeValues,
			ValidateEnumerationValue: func(value string, baseType model.Type, context map[string]string) error {
				return validateValueAgainstTypeWithFacets(schema, value, baseType, context)
			},
		},
	); err != nil {
		return err
	}

	// validate facet inheritance (A9)
	if baseType != nil {
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

	// XSD 1.0 spec: NOTATION type cannot be used directly; must have enumeration facet
	// however, if restricting a NOTATION-derived type that already has enumeration, additional
	// restrictions (like length facets) are allowed without re-specifying enumeration.
	isDirectNotation := !baseQName.IsZero() &&
		baseQName.Namespace == model.XSDNamespace &&
		baseQName.Local == string(model.TypeNameNOTATION)
	if isDirectNotation {
		// directly restricting xs:NOTATION - must have enumeration in this restriction
		if !hasEnumerationFacet(facetList) {
			return fmt.Errorf("NOTATION restriction must have enumeration facet")
		}
		if err := validateNotationEnumeration(schema, facetList); err != nil {
			return err
		}
	} else if hasEnumerationFacet(facetList) {
		// if this restriction adds enumeration facets, validate them against declared notations
		// (if the base type is NOTATION-derived)
		isNotation := false
		if baseType != nil {
			isNotation = isNotationType(baseType)
		} else if !baseQName.IsZero() {
			if defType, ok := typechain.LookupType(schema, baseQName); ok {
				isNotation = isNotationType(defType)
			}
		}
		if isNotation {
			if err := validateNotationEnumeration(schema, facetList); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateSimpleContentRestrictionFacets validates facets in a simpleContent restriction
func validateSimpleContentRestrictionFacets(schema *parser.Schema, restriction *model.Restriction) error {
	if restriction == nil {
		return nil
	}

	baseType, baseQName := resolveSimpleContentBaseType(schema, restriction.Base)
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

	if err := facetengine.ValidateSchemaConstraints(
		facetengine.SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
		facetengine.SchemaConstraintCallbacks{
			ValidateRangeConsistency: facetengine.ValidateRangeConsistency,
			ValidateRangeValues:      facetengine.ValidateRangeValues,
			ValidateEnumerationValue: func(value string, baseType model.Type, context map[string]string) error {
				return validateValueAgainstTypeWithFacets(schema, value, baseType, context)
			},
		},
	); err != nil {
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
