package resolver

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

type idValuePolicy int

const (
	idValuesAllowed idValuePolicy = iota
	idValuesDisallowed
)

var errCircularReference = errors.New("circular type reference")

// validateDefaultOrFixedValueWithResolvedType validates a default/fixed value after type resolution.
func validateDefaultOrFixedValueWithResolvedType(schema *parser.Schema, value string, typ types.Type, context map[string]string) error {
	return validateDefaultOrFixedValueWithResolvedTypeVisited(schema, value, typ, context, make(map[types.Type]bool))
}

func validateDefaultOrFixedValueWithResolvedTypeVisited(schema *parser.Schema, value string, typ types.Type, context map[string]string, visited map[types.Type]bool) error {
	return validateDefaultOrFixedValueResolved(schema, value, typ, context, visited, idValuesDisallowed)
}

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
			normalized := types.NormalizeWhiteSpace(value, baseType)
			facets := typeops.CollectRestrictionFacets(schema, sc.Restriction, baseType, nil)
			return types.ValidateValueAgainstFacets(normalized, baseType, facets, context)
		}
		return validateDefaultOrFixedValueResolved(schema, value, baseType, context, visited, policy)
	}

	normalizedValue := types.NormalizeWhiteSpace(value, typ)

	if types.IsQNameOrNotationType(typ) {
		if _, err := types.ParseQNameValue(normalizedValue, context); err != nil {
			return err
		}
	}

	if typ.IsBuiltin() {
		bt := types.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
		if bt != nil {
			if policy == idValuesDisallowed && typeops.IsIDOnlyType(typ.Name()) {
				return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
			}
			if err := bt.Validate(normalizedValue); err != nil {
				return err
			}
			if types.IsQNameOrNotationType(typ) {
				if err := validateQNameContext(normalizedValue, context); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if st, ok := typ.(*types.SimpleType); ok {
		if policy == idValuesDisallowed && typeops.IsIDOnlyDerivedType(schema, st) {
			return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
		}
		switch st.Variety() {
		case types.UnionVariety:
			memberTypes := typeops.ResolveUnionMemberTypes(schema, st)
			if len(memberTypes) > 0 {
				var firstErr error
				sawCycle := false
				for _, member := range memberTypes {
					if err := validateDefaultOrFixedValueResolved(schema, normalizedValue, member, context, visited, idValuesAllowed); err == nil {
						facets := typeops.CollectSimpleTypeFacets(schema, st, nil)
						return types.ValidateValueAgainstFacets(normalizedValue, st, facets, context)
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
					return fmt.Errorf("cannot validate default/fixed value for circular union type '%s'", typ.Name().Local)
				}
				return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalizedValue, typ.Name().Local)
			}
		case types.ListVariety:
			itemType := typeops.ResolveListItemType(schema, st)
			if itemType != nil {
				for item := range types.FieldsXMLWhitespaceSeq(normalizedValue) {
					if err := validateDefaultOrFixedValueResolved(schema, item, itemType, context, visited, policy); err != nil {
						if errors.Is(err, errCircularReference) {
							return fmt.Errorf("cannot validate default/fixed value for circular list item type '%s'", typ.Name().Local)
						}
						return err
					}
				}
			}
			facets := typeops.CollectSimpleTypeFacets(schema, st, nil)
			return types.ValidateValueAgainstFacets(normalizedValue, st, facets, context)
		default:
			if types.IsQNameOrNotationType(st) {
				if err := validateQNameContext(normalizedValue, context); err != nil {
					return err
				}
			} else if err := st.Validate(normalizedValue); err != nil {
				return err
			}
			facets := typeops.CollectSimpleTypeFacets(schema, st, nil)
			return types.ValidateValueAgainstFacets(normalizedValue, st, facets, context)
		}
		return nil
	}

	return nil
}
func validateQNameContext(value string, context map[string]string) error {
	_, err := types.ParseQNameValue(value, context)
	return err
}
