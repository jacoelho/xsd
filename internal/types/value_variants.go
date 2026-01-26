package types

import "fmt"

// ParseUnionValueVariants parses a value against union member types, returning all matching variants.
func ParseUnionValueVariants[T any](lexical string, members []T, parseMember func(string, T) ([]TypedValue, error)) ([]TypedValue, error) {
	if len(members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	values := make([]TypedValue, 0, len(members))
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

// ParseListValueVariants parses list values into per-item variants using XML whitespace splitting.
func ParseListValueVariants(lexical string, parseItem func(string) ([]TypedValue, error)) ([][]TypedValue, error) {
	items := SplitXMLWhitespaceFields(lexical)
	if len(items) == 0 {
		return nil, nil
	}
	parsed := make([][]TypedValue, len(items))
	for i, item := range items {
		values, err := parseItem(item)
		if err != nil {
			return nil, fmt.Errorf("invalid list item %q: %w", item, err)
		}
		parsed[i] = values
	}
	return parsed, nil
}

// AnyValueEqual reports whether any value in left equals any in right.
func AnyValueEqual(left, right []TypedValue) bool {
	for _, l := range left {
		for _, r := range right {
			if ValuesEqual(l, r) {
				return true
			}
		}
	}
	return false
}

// ListValuesEqual reports whether two lists of typed-value variants are equal item-by-item.
func ListValuesEqual(left, right [][]TypedValue) bool {
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
