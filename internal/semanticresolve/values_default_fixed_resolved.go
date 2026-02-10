package semanticresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

func validateDefaultOrFixedValueResolved(schema *parser.Schema, value string, typ model.Type, context map[string]string, visited map[model.Type]bool, policy idValuePolicy) error {
	if typ == nil {
		return nil
	}
	if st, ok := typ.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(st) {
		return fmt.Errorf("type %s not resolved", st.QName)
	}
	if visited[typ] {
		return errCircularReference
	}
	visited[typ] = true
	defer delete(visited, typ)

	if ct, ok := typ.(*model.ComplexType); ok {
		return validateDefaultOrFixedComplexType(schema, value, ct, context, visited, policy)
	}

	normalizedValue := model.NormalizeWhiteSpace(value, typ)
	if facetvalue.IsQNameOrNotationType(typ) {
		if err := facetengine.ValidateQNameContext(normalizedValue, context); err != nil {
			return err
		}
	}
	if typ.IsBuiltin() {
		return validateDefaultOrFixedBuiltinType(typ, normalizedValue, context, policy)
	}
	if st, ok := typ.(*model.SimpleType); ok {
		return validateDefaultOrFixedSimpleType(schema, normalizedValue, st, context, visited, policy)
	}
	return nil
}

func validateDefaultOrFixedComplexType(
	schema *parser.Schema,
	value string,
	ct *model.ComplexType,
	context map[string]string,
	visited map[model.Type]bool,
	policy idValuePolicy,
) error {
	sc, ok := ct.Content().(*model.SimpleContent)
	if !ok {
		return nil
	}
	baseType := typeresolve.ResolveSimpleContentBaseTypeFromContent(schema, sc)
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

func validateDefaultOrFixedBuiltinType(typ model.Type, normalizedValue string, context map[string]string, policy idValuePolicy) error {
	bt := builtins.GetNS(typ.Name().Namespace, typ.Name().Local)
	if bt == nil {
		return nil
	}
	if policy == idValuesDisallowed && typeresolve.IsIDOnlyType(typ.Name()) {
		return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
	}
	if err := bt.Validate(normalizedValue); err != nil {
		return err
	}
	if facetvalue.IsQNameOrNotationType(typ) {
		if err := facetengine.ValidateQNameContext(normalizedValue, context); err != nil {
			return err
		}
	}
	return nil
}

func validateDefaultOrFixedSimpleType(
	schema *parser.Schema,
	normalizedValue string,
	st *model.SimpleType,
	context map[string]string,
	visited map[model.Type]bool,
	policy idValuePolicy,
) error {
	if policy == idValuesDisallowed && typeresolve.IsIDOnlyDerivedType(schema, st) {
		return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", st.Name().Local)
	}
	switch st.Variety() {
	case model.UnionVariety:
		return validateDefaultOrFixedUnion(schema, normalizedValue, st, context, visited)
	case model.ListVariety:
		return validateDefaultOrFixedList(schema, normalizedValue, st, context, visited, policy)
	default:
		if facetvalue.IsQNameOrNotationType(st) {
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
	st *model.SimpleType,
	context map[string]string,
	visited map[model.Type]bool,
) error {
	memberTypes := typeresolve.ResolveUnionMemberTypes(schema, st)
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
	st *model.SimpleType,
	context map[string]string,
	visited map[model.Type]bool,
	policy idValuePolicy,
) error {
	itemType := typeresolve.ResolveListItemType(schema, st)
	if itemType != nil {
		for item := range model.FieldsXMLWhitespaceSeq(normalizedValue) {
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
