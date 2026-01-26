package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

var emptyNamespaceContext = map[string]string{}

func requiresQNameEnumeration(ct *grammar.CompiledType) bool {
	if ct == nil {
		return false
	}
	if ct.IsQNameOrNotationType {
		return true
	}
	if ct.ItemType != nil {
		return requiresQNameEnumeration(ct.ItemType)
	}
	return slices.ContainsFunc(ct.MemberTypes, requiresQNameEnumeration)
}

func (r *streamRun) validateQNameEnumerationForType(value string, enum *types.Enumeration, ct *grammar.CompiledType, scopeDepth int, context map[string]string) error {
	if enum == nil || ct == nil {
		return nil
	}
	switch {
	case ct.IsQNameOrNotationType:
		return r.validateQNameEnumeration(value, enum, scopeDepth, context)
	case ct.ItemType != nil:
		return r.validateQNameListEnumeration(value, enum, ct, scopeDepth, context)
	case len(ct.MemberTypes) > 0:
		return r.validateQNameUnionEnumeration(value, enum, ct, scopeDepth, context)
	default:
		return enum.ValidateLexical(value, ct.Original)
	}
}

func (r *streamRun) validateQNameUnionEnumeration(value string, enum *types.Enumeration, ct *grammar.CompiledType, scopeDepth int, context map[string]string) error {
	actualValues, err := r.parseUnionValueVariantsWithContext(value, ct.MemberTypes, scopeDepth, context)
	if err != nil {
		return err
	}
	allowed, err := r.unionEnumerationValuesWithContext(enum, ct)
	if err != nil {
		return err
	}
	for _, actual := range actualValues {
		for _, candidate := range allowed {
			if types.ValuesEqual(actual, candidate) {
				return nil
			}
		}
	}
	return fmt.Errorf("value %s not in enumeration: %s", value, types.FormatEnumerationValues(enum.Values))
}

func (r *streamRun) validateQNameListEnumeration(value string, enum *types.Enumeration, ct *grammar.CompiledType, scopeDepth int, context map[string]string) error {
	actualItems, err := r.parseListValueVariantsWithContext(value, ct.ItemType, scopeDepth, context)
	if err != nil {
		return err
	}
	allowed, err := r.listEnumerationValuesWithContext(enum, ct)
	if err != nil {
		return err
	}
	for _, candidate := range allowed {
		if types.ListValuesEqual(actualItems, candidate) {
			return nil
		}
	}
	return fmt.Errorf("value %s not in enumeration: %s", value, types.FormatEnumerationValues(enum.Values))
}

func (r *streamRun) unionEnumerationValuesWithContext(enum *types.Enumeration, ct *grammar.CompiledType) ([]types.TypedValue, error) {
	values := make([]types.TypedValue, 0, len(enum.Values))
	for i, val := range enum.Values {
		normalized := types.NormalizeWhiteSpace(val, ct.Original)
		parsed, err := r.parseUnionValueVariantsWithContext(normalized, ct.MemberTypes, 0, enumContext(enum, i))
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values = append(values, parsed...)
	}
	return values, nil
}

func (r *streamRun) listEnumerationValuesWithContext(enum *types.Enumeration, ct *grammar.CompiledType) ([][][]types.TypedValue, error) {
	values := make([][][]types.TypedValue, len(enum.Values))
	for i, val := range enum.Values {
		normalized := types.NormalizeWhiteSpace(val, ct.Original)
		parsed, err := r.parseListValueVariantsWithContext(normalized, ct.ItemType, 0, enumContext(enum, i))
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values[i] = parsed
	}
	return values, nil
}

func (r *streamRun) parseUnionValueVariantsWithContext(value string, memberTypes []*grammar.CompiledType, scopeDepth int, context map[string]string) ([]types.TypedValue, error) {
	return types.ParseUnionValueVariants(value, memberTypes, func(val string, member *grammar.CompiledType) ([]types.TypedValue, error) {
		return r.parseValueVariantsForCompiledType(val, member, scopeDepth, context)
	})
}

func (r *streamRun) parseListValueVariantsWithContext(value string, itemType *grammar.CompiledType, scopeDepth int, context map[string]string) ([][]types.TypedValue, error) {
	if itemType == nil {
		return nil, fmt.Errorf("list item type is nil")
	}
	return types.ParseListValueVariants(value, func(item string) ([]types.TypedValue, error) {
		return r.parseValueVariantsForCompiledType(item, itemType, scopeDepth, context)
	})
}

func (r *streamRun) parseValueVariantsForCompiledType(value string, ct *grammar.CompiledType, scopeDepth int, context map[string]string) ([]types.TypedValue, error) {
	if ct == nil || ct.Original == nil {
		return nil, fmt.Errorf("type is nil")
	}
	if len(ct.MemberTypes) > 0 {
		return r.parseUnionValueVariantsWithContext(value, ct.MemberTypes, scopeDepth, context)
	}
	if ct.IsQNameOrNotationType {
		qname, err := r.parseQNameValueWithContext(value, scopeDepth, context)
		if err != nil {
			return nil, err
		}
		return []types.TypedValue{qnameTypedValue{typ: ct.Original, lexical: value, value: qname}}, nil
	}
	typed, err := r.parseValueAsTypeWithContext(value, ct.Original, context)
	if err != nil {
		return nil, err
	}
	return []types.TypedValue{typed}, nil
}

func enumContext(enum *types.Enumeration, index int) map[string]string {
	if enum == nil {
		return emptyNamespaceContext
	}
	contexts := enum.ValueContexts()
	if index < len(contexts) && contexts[index] != nil {
		return contexts[index]
	}
	return emptyNamespaceContext
}
