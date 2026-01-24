package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateSimpleTypeStructure validates structural constraints of a simple type
// Does not validate type references (which might be forward references or imports)
func validateSimpleTypeStructure(schema *parser.Schema, st *types.SimpleType) error {
	switch st.Variety() {
	case types.AtomicVariety:
		if st.Restriction != nil {
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
			if baseType := resolveSimpleTypeRestrictionBase(schema, st, st.Restriction); baseType != nil {
				if baseST, ok := baseType.(*types.SimpleType); ok {
					if err := validateLengthFacetInheritance(facetsFromRestriction(st.Restriction), baseST); err != nil {
						return fmt.Errorf("restriction: %w", err)
					}
				}
			}
		}
	case types.ListVariety:
		// list types can be defined by xs:list or by restriction of a list type
		if st.List != nil {
			if err := validateListType(schema, st.List); err != nil {
				return fmt.Errorf("list: %w", err)
			}
		} else if st.Restriction != nil {
			// list type derived by restriction of another list type - validate facets
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	case types.UnionVariety:
		// union types can be defined by xs:union or by restriction of a union type
		if st.Union != nil {
			if err := validateUnionType(schema, st.Union); err != nil {
				return fmt.Errorf("union: %w", err)
			}
		} else if st.Restriction != nil {
			// union type derived by restriction of another union type - validate facets
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	}
	if err := validateSimpleTypeDerivationConstraints(schema, st); err != nil {
		return err
	}
	return nil
}

// validateSimpleTypeDerivationConstraints validates final constraints on simple type derivation
func validateSimpleTypeDerivationConstraints(schema *parser.Schema, st *types.SimpleType) error {
	if st == nil {
		return nil
	}

	if st.Restriction != nil {
		baseType := resolveSimpleTypeRestrictionBase(schema, st, st.Restriction)
		if baseST, ok := baseType.(*types.SimpleType); ok {
			if baseST.Final.Has(types.DerivationRestriction) {
				return fmt.Errorf("cannot restrict type '%s': base type is final for restriction", baseST.Name())
			}
		}
	}

	if st.List != nil {
		itemType := st.ItemType
		if itemType == nil && st.List.InlineItemType != nil {
			itemType = st.List.InlineItemType
		}
		if itemType == nil && !st.List.ItemType.IsZero() {
			itemType = resolveSimpleTypeReference(schema, st.List.ItemType)
		}
		if itemST, ok := itemType.(*types.SimpleType); ok {
			if itemST.Final.Has(types.DerivationList) {
				return fmt.Errorf("cannot derive list from type '%s': base type is final for list", itemST.Name())
			}
		}
	}

	if st.Union != nil {
		memberTypes := st.MemberTypes
		if len(memberTypes) == 0 {
			for _, inlineType := range st.Union.InlineTypes {
				memberTypes = append(memberTypes, inlineType)
			}
			for _, memberQName := range st.Union.MemberTypes {
				if member := resolveSimpleTypeReference(schema, memberQName); member != nil {
					memberTypes = append(memberTypes, member)
				}
			}
		}
		for _, member := range memberTypes {
			if memberST, ok := member.(*types.SimpleType); ok {
				if memberST.Final.Has(types.DerivationUnion) {
					return fmt.Errorf("cannot derive union from type '%s': base type is final for union", memberST.Name())
				}
			}
		}
	}

	return nil
}

// resolveSimpleTypeRestrictionBase resolves the base type for a simple type restriction
func resolveSimpleTypeRestrictionBase(schema *parser.Schema, st *types.SimpleType, restriction *types.Restriction) types.Type {
	if st != nil && st.ResolvedBase != nil {
		return st.ResolvedBase
	}
	if restriction == nil || restriction.Base.IsZero() {
		return nil
	}
	return resolveSimpleTypeReference(schema, restriction.Base)
}

// resolveSimpleTypeReference resolves a simple type reference by QName
func resolveSimpleTypeReference(schema *parser.Schema, qname types.QName) types.Type {
	if qname.IsZero() {
		return nil
	}
	if qname.Namespace == types.XSDNamespace {
		if bt := types.GetBuiltin(types.TypeName(qname.Local)); bt != nil {
			return bt
		}
	}
	if typ, ok := lookupTypeDef(schema, qname); ok {
		return typ
	}
	return nil
}

// resolveSimpleContentBaseType resolves the base type for a simpleContent restriction
func resolveSimpleContentBaseType(schema *parser.Schema, baseQName types.QName) (types.Type, types.QName) {
	if baseQName.IsZero() {
		return nil, baseQName
	}

	if baseQName.Namespace == types.XSDNamespace {
		if bt := types.GetBuiltin(types.TypeName(baseQName.Local)); bt != nil {
			return bt, baseQName
		}
	}

	baseType, ok := lookupTypeDef(schema, baseQName)
	if !ok || baseType == nil {
		return nil, baseQName
	}

	if ct, ok := baseType.(*types.ComplexType); ok {
		if _, ok := ct.Content().(*types.SimpleContent); ok {
			base := types.ResolveSimpleContentBaseType(ct.BaseType())
			if base != nil {
				return base, base.Name()
			}
		}
		return baseType, baseQName
	}

	return baseType, baseQName
}

// validateRestriction validates a simple type restriction
func validateRestriction(schema *parser.Schema, st *types.SimpleType, restriction *types.Restriction) error {
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
			if defType, ok := lookupTypeDef(schema, restriction.Base); ok {
				baseType = defType
			}
		}
	}

	// convert facets to []types.Facet for validation
	// also process deferred facets (range facets that couldn't be constructed during parsing)
	facetList := make([]types.Facet, 0, len(restriction.Facets))
	var deferredFacets []*types.DeferredFacet
	for _, f := range restriction.Facets {
		switch facet := f.(type) {
		case types.Facet:
			facetList = append(facetList, facet)
		case *types.DeferredFacet:
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
			return err
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
			if defType, ok := lookupTypeDef(schema, baseQName); ok {
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
func validateSimpleContentRestrictionFacets(schema *parser.Schema, restriction *types.Restriction) error {
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
		switch facet := f.(type) {
		case types.Facet:
			facetList = append(facetList, facet)
		case *types.DeferredFacet:
			deferredFacets = append(deferredFacets, facet)
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

// validateUnionType validates a union type definition
func validateUnionType(schema *parser.Schema, unionType *types.UnionType) error {
	// union must have at least one member type (from attribute or inline)
	if len(unionType.MemberTypes) == 0 && len(unionType.InlineTypes) == 0 {
		return fmt.Errorf("union type must have at least one member type")
	}

	// validate that all member types are simple types (not complex types)
	// union types can only have simple types as members
	for i, memberQName := range unionType.MemberTypes {
		// check if it's a built-in type (all built-in types in XSD namespace are simple)
		if memberQName.Namespace == types.XSDNamespace {
			// check if it's an XSD 1.1 type (not supported)
			if isXSD11Type(memberQName.Local) {
				return fmt.Errorf("union memberType %d: '%s' is an XSD 1.1 type (not supported in XSD 1.0)", i+1, memberQName.Local)
			}
			// built-in types in XSD namespace are always simple types
			continue
		}

		if memberType, ok := lookupTypeDef(schema, memberQName); ok {
			// union members must be simple types, not complex types
			if _, isComplex := memberType.(*types.ComplexType); isComplex {
				return fmt.Errorf("union memberType %d: '%s' is a complex type (union types can only have simple types as members)", i+1, memberQName.Local)
			}
		}
	}

	// validate inline types (they're already SimpleType, so no need to check)
	// inline types are parsed as SimpleType, so they're always valid

	return nil
}

// isXSD11Type checks if a type name is an XSD 1.1 type (not supported in XSD 1.0)
func isXSD11Type(typeName string) bool {
	xsd11Types := map[string]bool{
		"timeDuration":      true,
		"yearMonthDuration": true,
		"dayTimeDuration":   true,
		"dateTimeStamp":     true,
		"precisionDecimal":  true,
	}
	return xsd11Types[typeName]
}

// validateListType validates a list type definition
func validateListType(schema *parser.Schema, listType *types.ListType) error {
	// list type must have itemType (either via itemType attribute or inline simpleType child per XSD spec)
	if listType.ItemType.IsZero() {
		if listType.InlineItemType == nil {
			return fmt.Errorf("list type must have itemType attribute or inline simpleType child")
		}
		// inline simpleType is present - validate it
		if err := validateSimpleTypeStructure(schema, listType.InlineItemType); err != nil {
			return fmt.Errorf("inline simpleType in list: %w", err)
		}
		// list itemType must be atomic or union (NOT list)
		variety := listType.InlineItemType.Variety()
		if variety != types.AtomicVariety && variety != types.UnionVariety {
			return fmt.Errorf("list itemType must be atomic or union, got %s", variety)
		}
		return nil // inline simpleType is valid
	}

	// list itemType must be atomic or union (NOT list)
	// check if it's a built-in type (always atomic)
	if listType.ItemType.Namespace == types.XSDNamespace {
		return nil // built-in types are always atomic
	}

	// check if it's a user-defined type in this schema
	if defType, ok := lookupTypeDef(schema, listType.ItemType); ok {
		st, ok := defType.(*types.SimpleType)
		if !ok {
			return fmt.Errorf("list itemType must be a simple type, got %T", defType)
		}
		// list itemType must be atomic or union
		variety := st.Variety()
		if variety != types.AtomicVariety && variety != types.UnionVariety {
			return fmt.Errorf("list itemType must be atomic or union, got %s", variety)
		}
	}
	// if type not found, might be forward reference - skip validation

	return nil
}
