package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateSimpleTypeStructure validates structural constraints of a simple type.
// Does not validate type references, which may be forward references or imports.
func validateSimpleTypeStructure(schema *parser.Schema, st *model.SimpleType) error {
	switch st.Variety() {
	case model.AtomicVariety:
		if st.Restriction != nil {
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
			if baseType := resolveSimpleTypeRestrictionBase(schema, st, st.Restriction); baseType != nil {
				if baseST, ok := baseType.(*model.SimpleType); ok {
					if err := validateLengthFacetInheritance(facetsFromRestriction(st.Restriction), baseST); err != nil {
						return fmt.Errorf("restriction: %w", err)
					}
				}
			}
		}
	case model.ListVariety:
		if st.List != nil {
			if err := validateListType(schema, st.List); err != nil {
				return fmt.Errorf("list: %w", err)
			}
		} else if st.Restriction != nil {
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	case model.UnionVariety:
		if st.Union != nil {
			if err := validateUnionType(schema, st.Union); err != nil {
				return fmt.Errorf("union: %w", err)
			}
		} else if st.Restriction != nil {
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	}
	if st.Variety() == model.ListVariety && st.WhiteSpace() != model.WhiteSpaceCollapse {
		return fmt.Errorf("list whiteSpace facet must be 'collapse'")
	}
	return validateSimpleTypeDerivationConstraints(schema, st)
}

// validateRestriction validates a simple type restriction.
func validateRestriction(schema *parser.Schema, st *model.SimpleType, restriction *model.Restriction) error {
	baseType := resolveRestrictionBaseType(schema, st, restriction)

	baseQName := restriction.Base
	if baseQName.IsZero() && !model.IsNilType(baseType) {
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
	if !model.IsNilType(baseType) {
		if err := validateFacetInheritance(facetList, baseType); err != nil {
			return err
		}
	}
	if st.Variety() == model.ListVariety && st.WhiteSpace() != model.WhiteSpaceCollapse {
		return fmt.Errorf("list whiteSpace facet must be 'collapse'")
	}
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
		if !ok || model.IsNilType(defType) {
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

// validateSimpleContentRestrictionFacets validates facets in a simpleContent restriction.
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
	if baseST, ok := baseType.(*model.SimpleType); ok {
		if err := validateFacetInheritance(facetList, baseST); err != nil {
			return err
		}
		if err := validateLengthFacetInheritance(facetList, baseST); err != nil {
			return err
		}
	}

	return nil
}

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

// resolveSimpleTypeRestrictionBase resolves the base type for a simple type restriction.
func resolveSimpleTypeRestrictionBase(schema *parser.Schema, st *model.SimpleType, restriction *model.Restriction) model.Type {
	if st != nil && st.ResolvedBase != nil {
		return st.ResolvedBase
	}
	if restriction != nil && restriction.SimpleType != nil {
		return restriction.SimpleType
	}
	if restriction == nil || restriction.Base.IsZero() {
		return nil
	}
	return parser.ResolveSimpleTypeReferenceAllowMissing(schema, restriction.Base)
}

// resolveSimpleContentBaseTypeQName resolves the base type QName chain for simpleContent restriction checks.
func resolveSimpleContentBaseTypeQName(schema *parser.Schema, baseQName model.QName) (model.Type, model.QName) {
	visited := make(map[model.QName]bool)
	var visit func(model.QName) (model.Type, model.QName)
	visit = func(qname model.QName) (model.Type, model.QName) {
		if qname.IsZero() || visited[qname] {
			return nil, qname
		}
		visited[qname] = true

		if qname.Namespace == model.XSDNamespace {
			if bt := model.GetBuiltin(model.TypeName(qname.Local)); bt != nil {
				return bt, qname
			}
		}

		baseType, ok := LookupType(schema, qname)
		if !ok || baseType == nil {
			return nil, qname
		}

		ct, ok := baseType.(*model.ComplexType)
		if !ok {
			return baseType, qname
		}
		sc, ok := ct.Content().(*model.SimpleContent)
		if !ok {
			return baseType, qname
		}

		nextQName := sc.BaseTypeQName()
		if nextQName.IsZero() {
			return nil, qname
		}

		resolved, resolvedQName := visit(nextQName)
		if resolved != nil {
			return resolved, resolvedQName
		}
		return nil, nextQName
	}
	return visit(baseQName)
}

// validateSimpleTypeDerivationConstraints validates final constraints on simple type derivation.
func validateSimpleTypeDerivationConstraints(schema *parser.Schema, st *model.SimpleType) error {
	if st == nil {
		return nil
	}

	if st.Restriction != nil {
		baseType := resolveSimpleTypeRestrictionBase(schema, st, st.Restriction)
		if baseST, ok := baseType.(*model.SimpleType); ok && baseST.Final.Has(model.DerivationRestriction) {
			return fmt.Errorf("cannot restrict type '%s': base type is final for restriction", baseST.Name())
		}
	}

	if st.List != nil {
		itemType := st.ItemType
		if itemType == nil && st.List.InlineItemType != nil {
			itemType = st.List.InlineItemType
		}
		if itemType == nil && !st.List.ItemType.IsZero() {
			itemType = parser.ResolveSimpleTypeReferenceAllowMissing(schema, st.List.ItemType)
		}
		if itemST, ok := itemType.(*model.SimpleType); ok && itemST.Final.Has(model.DerivationList) {
			return fmt.Errorf("cannot derive list from type '%s': base type is final for list", itemST.Name())
		}
	}

	if st.Union != nil {
		memberTypes := st.MemberTypes
		if len(memberTypes) == 0 {
			for _, inlineType := range st.Union.InlineTypes {
				memberTypes = append(memberTypes, inlineType)
			}
			for _, memberQName := range st.Union.MemberTypes {
				if member := parser.ResolveSimpleTypeReferenceAllowMissing(schema, memberQName); member != nil {
					memberTypes = append(memberTypes, member)
				}
			}
		}
		for _, member := range memberTypes {
			if memberST, ok := member.(*model.SimpleType); ok && memberST.Final.Has(model.DerivationUnion) {
				return fmt.Errorf("cannot derive union from type '%s': base type is final for union", memberST.Name())
			}
		}
	}

	return nil
}

// validateUnionType validates a union type definition.
func validateUnionType(schema *parser.Schema, unionType *model.UnionType) error {
	if len(unionType.MemberTypes) == 0 && len(unionType.InlineTypes) == 0 {
		return fmt.Errorf("union type must have at least one member type")
	}

	for i, memberQName := range unionType.MemberTypes {
		if memberQName.Namespace == model.XSDNamespace {
			if isXSD11Type(memberQName.Local) {
				return fmt.Errorf("union memberType %d: '%s' is an XSD 1.1 type (not supported in XSD 1.0)", i+1, memberQName.Local)
			}
			continue
		}
		if memberType, ok := LookupType(schema, memberQName); ok {
			if _, isComplex := memberType.(*model.ComplexType); isComplex {
				return fmt.Errorf("union memberType %d: '%s' is a complex type (union types can only have simple types as members)", i+1, memberQName.Local)
			}
		}
	}

	return nil
}

// isXSD11Type checks if a type name is an XSD 1.1 type.
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

// validateListType validates a list type definition.
func validateListType(schema *parser.Schema, listType *model.ListType) error {
	if listType.ItemType.IsZero() {
		if listType.InlineItemType == nil {
			return fmt.Errorf("list type must have itemType attribute or inline simpleType child")
		}
		if err := validateSimpleTypeStructure(schema, listType.InlineItemType); err != nil {
			return fmt.Errorf("inline simpleType in list: %w", err)
		}
		variety := listType.InlineItemType.Variety()
		if variety != model.AtomicVariety && variety != model.UnionVariety {
			return fmt.Errorf("list itemType must be atomic or union, got %s", variety)
		}
		return nil
	}

	if listType.ItemType.Namespace == model.XSDNamespace {
		return nil
	}

	if defType, ok := LookupType(schema, listType.ItemType); ok {
		st, ok := defType.(*model.SimpleType)
		if !ok {
			return fmt.Errorf("list itemType must be a simple type, got %T", defType)
		}
		variety := st.Variety()
		if variety != model.AtomicVariety && variety != model.UnionVariety {
			return fmt.Errorf("list itemType must be atomic or union, got %s", variety)
		}
	}

	return nil
}
