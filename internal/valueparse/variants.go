package valueparse

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

// ParseUnionValueVariants parses a value against union members and returns all matching variants.
func ParseUnionValueVariants[T any](lexical string, members []T, parseMember func(string, T) ([]types.TypedValue, error)) ([]types.TypedValue, error) {
	if len(members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	values := make([]types.TypedValue, 0, len(members))
	var firstErr error
	for _, member := range members {
		parsed, err := parseMember(lexical, member)
		if err == nil {
			values = append(values, parsed...)
			continue
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if len(values) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("value %q does not match any union member type", lexical)
	}
	return values, nil
}

// ParseListValueVariants parses list items into per-item variants using XML whitespace splitting.
func ParseListValueVariants(lexical string, parseItem func(string) ([]types.TypedValue, error)) ([][]types.TypedValue, error) {
	parsed := make([][]types.TypedValue, 0, 4)
	for item := range types.FieldsXMLWhitespaceSeq(lexical) {
		values, err := parseItem(item)
		if err != nil {
			return nil, fmt.Errorf("invalid list item %q: %w", item, err)
		}
		parsed = append(parsed, values)
	}
	if len(parsed) == 0 {
		return nil, nil
	}
	return parsed, nil
}

// AnyValueEqual reports whether any value in left equals any in right.
func AnyValueEqual(left, right []types.TypedValue) bool {
	for _, l := range left {
		for _, r := range right {
			if types.ValuesEqual(l, r) {
				return true
			}
		}
	}
	return false
}

// ListValuesEqual reports whether two lists of typed-value variants are equal item-by-item.
func ListValuesEqual(left, right [][]types.TypedValue) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !AnyValueEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}
