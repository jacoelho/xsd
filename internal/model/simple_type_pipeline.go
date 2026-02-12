package model

import (
	"errors"
	"fmt"

	qnamelex "github.com/jacoelho/xsd/internal/qname"
)

// SimpleTypeValidationOptions configures shared simple-type validation behavior.
type SimpleTypeValidationOptions struct {
	CycleError            error
	ResolveListItem       func(*SimpleType) Type
	ResolveUnionMembers   func(*SimpleType) []Type
	ResolveFacetType      FacetTypeResolver
	ConvertDeferredFacets DeferredFacetConverter
	ValidateFacets        func(normalized string, st *SimpleType, context map[string]string) error
	UnionNoMatch          func(st *SimpleType, normalized string, firstErr error, sawCycle bool) error
	ListItemErr           func(st *SimpleType, err error, isCycle bool) error
	ValidateType          func(typ Type, scope SimpleTypeValidationScope) error
	RequireQNameContext   bool
	ValidationScope       SimpleTypeValidationScope
}

// SimpleTypeValidationScope identifies how type validation reached a nested type.
type SimpleTypeValidationScope uint8

const (
	// SimpleTypeValidationScopeRoot is an exported constant.
	SimpleTypeValidationScopeRoot SimpleTypeValidationScope = iota
	// SimpleTypeValidationScopeListItem is an exported constant.
	SimpleTypeValidationScopeListItem
	// SimpleTypeValidationScopeUnionMember is an exported constant.
	SimpleTypeValidationScopeUnionMember
)

// ValidateSimpleTypeWithOptions validates lexical input for a simple type with shared pipeline options.
func ValidateSimpleTypeWithOptions(st *SimpleType, lexical string, context map[string]string, opts SimpleTypeValidationOptions) error {
	if st == nil {
		return fmt.Errorf("cannot validate value for nil simple type")
	}
	opts = validationOptionsForScope(opts, SimpleTypeValidationScopeRoot)
	normalized, err := normalizeValue(lexical, st)
	if err != nil {
		return err
	}
	return validateSimpleTypeNormalizedWithOptions(st, normalized, context, make(map[*SimpleType]bool), opts)
}

func validateSimpleTypeNormalizedWithOptions(
	st *SimpleType,
	normalized string,
	context map[string]string,
	visited map[*SimpleType]bool,
	opts SimpleTypeValidationOptions,
) error {
	if st == nil {
		return nil
	}
	if visited[st] {
		if opts.CycleError != nil {
			return opts.CycleError
		}
		return nil
	}
	visited[st] = true
	defer delete(visited, st)

	switch st.Variety() {
	case ListVariety:
		itemType := listItemTypeForValidation(st, opts)
		if itemType == nil {
			return fmt.Errorf("list item type is missing")
		}
		itemOpts := validationOptionsForScope(opts, SimpleTypeValidationScopeListItem)
		for item := range FieldsXMLWhitespaceSeq(normalized) {
			if err := validateTypeWithOptions(itemType, item, context, visited, itemOpts); err != nil {
				isCycle := itemOpts.CycleError != nil && errors.Is(err, itemOpts.CycleError)
				if opts.ListItemErr != nil {
					return opts.ListItemErr(st, err, isCycle)
				}
				return err
			}
		}
	case UnionVariety:
		memberTypes := unionMemberTypesForValidation(st, opts)
		if len(memberTypes) == 0 {
			return fmt.Errorf("union has no member types")
		}
		var (
			firstErr error
			sawCycle bool
			matched  bool
		)
		memberOpts := validationOptionsForScope(opts, SimpleTypeValidationScopeUnionMember)
		for _, member := range memberTypes {
			if err := validateTypeWithOptions(member, normalized, context, visited, memberOpts); err == nil {
				matched = true
				break
			} else if memberOpts.CycleError != nil && errors.Is(err, memberOpts.CycleError) {
				sawCycle = true
			} else if firstErr == nil {
				firstErr = err
			}
		}
		if !matched {
			if opts.UnionNoMatch != nil {
				return opts.UnionNoMatch(st, normalized, firstErr, sawCycle)
			}
			if firstErr != nil {
				return firstErr
			}
			return fmt.Errorf("value %q does not match any member type", normalized)
		}
	default:
		if err := validateAtomicLexicalWithOptions(st, normalized, context, opts.RequireQNameContext); err != nil {
			return err
		}
	}

	if opts.ValidateFacets != nil {
		return opts.ValidateFacets(normalized, st, context)
	}
	facets, err := CollectSimpleTypeFacetsWithResolver(st, opts.ResolveFacetType, opts.ConvertDeferredFacets)
	if err != nil {
		return err
	}
	if len(facets) == 0 {
		return nil
	}
	return validateNormalizedFacetsWithContext(normalized, st, facets, context)
}

func validateTypeWithOptions(
	typ Type,
	lexical string,
	context map[string]string,
	visited map[*SimpleType]bool,
	opts SimpleTypeValidationOptions,
) error {
	if typ == nil {
		return nil
	}
	if opts.ValidateType != nil {
		if err := opts.ValidateType(typ, opts.ValidationScope); err != nil {
			return err
		}
	}
	normalized, err := normalizeValue(lexical, typ)
	if err != nil {
		return err
	}
	if st, ok := AsSimpleType(typ); ok {
		return validateSimpleTypeNormalizedWithOptions(st, normalized, context, visited, opts)
	}
	if bt, ok := AsBuiltinType(typ); ok {
		if isQNameOrNotationType(bt) {
			if opts.RequireQNameContext && context == nil {
				return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
			}
			if _, err := qnamelex.ParseQNameValue(normalized, context); err != nil {
				return err
			}
		}
		return bt.Validate(normalized)
	}
	return nil
}

func validateAtomicLexicalWithOptions(st *SimpleType, normalized string, context map[string]string, requireQNameContext bool) error {
	if isQNameOrNotationType(st) {
		if requireQNameContext && context == nil {
			return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
		}
		if _, err := qnamelex.ParseQNameValue(normalized, context); err != nil {
			return err
		}
	}
	if st.IsBuiltin() {
		if builtinType := getBuiltinNS(st.QName.Namespace, st.QName.Local); builtinType != nil {
			return builtinType.Validate(normalized)
		}
	}
	if st.Restriction != nil {
		primitive := st.PrimitiveType()
		if builtinType, ok := AsBuiltinType(primitive); ok {
			return builtinType.Validate(normalized)
		}
		if primitiveST, ok := AsSimpleType(primitive); ok && primitiveST.IsBuiltin() {
			if builtinType := getBuiltinNS(primitiveST.QName.Namespace, primitiveST.QName.Local); builtinType != nil {
				return builtinType.Validate(normalized)
			}
		}
	}
	return nil
}

func listItemTypeForValidation(st *SimpleType, opts SimpleTypeValidationOptions) Type {
	if st == nil {
		return nil
	}
	if opts.ResolveListItem != nil {
		return opts.ResolveListItem(st)
	}
	itemType, _ := ListItemType(st)
	return itemType
}

func unionMemberTypesForValidation(st *SimpleType, opts SimpleTypeValidationOptions) []Type {
	if st == nil {
		return nil
	}
	if opts.ResolveUnionMembers != nil {
		return opts.ResolveUnionMembers(st)
	}
	return unionMemberTypes(st)
}

func validationOptionsForScope(opts SimpleTypeValidationOptions, scope SimpleTypeValidationScope) SimpleTypeValidationOptions {
	next := opts
	next.ValidationScope = scope
	return next
}
