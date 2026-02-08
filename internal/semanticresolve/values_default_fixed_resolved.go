package semanticresolve

import (
	"errors"
	"fmt"

	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func validateDefaultOrFixedValueResolved(schema *parser.Schema, value string, typ types.Type, context map[string]string, visited map[types.Type]bool, policy idValuePolicy) error {
	if typ == nil {
		return nil
	}
	if st, ok := typ.(*types.SimpleType); ok && types.IsPlaceholderSimpleType(st) {
		return fmt.Errorf("type %s not resolved", st.QName)
	}
	if visited[typ] {
		return errCircularReference
	}
	visited[typ] = true
	defer delete(visited, typ)

	if ct, ok := typ.(*types.ComplexType); ok {
		return validateDefaultOrFixedComplexType(schema, value, ct, context, visited, policy)
	}

	normalizedValue := types.NormalizeWhiteSpace(value, typ)
	if types.IsQNameOrNotationType(typ) {
		if err := facetengine.ValidateQNameContext(normalizedValue, context); err != nil {
			return err
		}
	}
	if typ.IsBuiltin() {
		return validateDefaultOrFixedBuiltinType(typ, normalizedValue, context, policy)
	}
	if st, ok := typ.(*types.SimpleType); ok {
		return validateDefaultOrFixedSimpleType(schema, normalizedValue, st, context, visited, policy)
	}
	return nil
}

func validateDefaultOrFixedComplexType(
	schema *parser.Schema,
	value string,
	ct *types.ComplexType,
	context map[string]string,
	visited map[types.Type]bool,
	policy idValuePolicy,
) error {
	sc, ok := ct.Content().(*types.SimpleContent)
	if !ok {
		return nil
	}
	baseType := typeops.ResolveSimpleContentBaseTypeFromContent(schema, sc)
	if baseType == nil {
		return nil
	}
	if sc.Restriction != nil {
		if err := validateDefaultOrFixedValueResolved(schema, value, baseType, context, visited, policy); err != nil {
			return err
		}
		return facetengine.ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, nil)
	}
	return validateDefaultOrFixedValueResolved(schema, value, baseType, context, visited, policy)
}

func validateDefaultOrFixedBuiltinType(typ types.Type, normalizedValue string, context map[string]string, policy idValuePolicy) error {
	bt := types.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
	if bt == nil {
		return nil
	}
	if policy == idValuesDisallowed && typeops.IsIDOnlyType(typ.Name()) {
		return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
	}
	if err := bt.Validate(normalizedValue); err != nil {
		return err
	}
	if types.IsQNameOrNotationType(typ) {
		if err := facetengine.ValidateQNameContext(normalizedValue, context); err != nil {
			return err
		}
	}
	return nil
}

func validateDefaultOrFixedSimpleType(
	schema *parser.Schema,
	normalizedValue string,
	st *types.SimpleType,
	context map[string]string,
	visited map[types.Type]bool,
	policy idValuePolicy,
) error {
	if policy == idValuesDisallowed && typeops.IsIDOnlyDerivedType(schema, st) {
		return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", st.Name().Local)
	}
	switch st.Variety() {
	case types.UnionVariety:
		return validateDefaultOrFixedUnion(schema, normalizedValue, st, context, visited)
	case types.ListVariety:
		return validateDefaultOrFixedList(schema, normalizedValue, st, context, visited, policy)
	default:
		if types.IsQNameOrNotationType(st) {
			if err := facetengine.ValidateQNameContext(normalizedValue, context); err != nil {
				return err
			}
		} else if err := st.Validate(normalizedValue); err != nil {
			return err
		}
		return facetengine.ValidateSimpleTypeFacets(schema, st, normalizedValue, context, nil)
	}
}

func validateDefaultOrFixedUnion(
	schema *parser.Schema,
	normalizedValue string,
	st *types.SimpleType,
	context map[string]string,
	visited map[types.Type]bool,
) error {
	memberTypes := typeops.ResolveUnionMemberTypes(schema, st)
	if len(memberTypes) == 0 {
		return nil
	}
	var firstErr error
	sawCycle := false
	for _, member := range memberTypes {
		if err := validateDefaultOrFixedValueResolved(schema, normalizedValue, member, context, visited, idValuesAllowed); err == nil {
			return facetengine.ValidateSimpleTypeFacets(schema, st, normalizedValue, context, nil)
		} else if errors.Is(err, errCircularReference) {
			sawCycle = true
		} else if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if sawCycle {
		return fmt.Errorf("cannot validate default/fixed value for circular union type '%s'", st.Name().Local)
	}
	return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalizedValue, st.Name().Local)
}

func validateDefaultOrFixedList(
	schema *parser.Schema,
	normalizedValue string,
	st *types.SimpleType,
	context map[string]string,
	visited map[types.Type]bool,
	policy idValuePolicy,
) error {
	itemType := typeops.ResolveListItemType(schema, st)
	if itemType != nil {
		for item := range types.FieldsXMLWhitespaceSeq(normalizedValue) {
			if err := validateDefaultOrFixedValueResolved(schema, item, itemType, context, visited, policy); err != nil {
				if errors.Is(err, errCircularReference) {
					return fmt.Errorf("cannot validate default/fixed value for circular list item type '%s'", st.Name().Local)
				}
				return err
			}
		}
	}
	return facetengine.ValidateSimpleTypeFacets(schema, st, normalizedValue, context, nil)
}
