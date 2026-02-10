package valueparse

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
)

// ParseUnionValueVariants parses a value against union members and returns all matching variants.
func ParseUnionValueVariants[T any, V any](lexical string, members []T, parseMember func(string, T) ([]V, error)) ([]V, error) {
	if len(members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	values := make([]V, 0, len(members))
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
func ParseListValueVariants[V any](lexical string, parseItem func(string) ([]V, error)) ([][]V, error) {
	parsed := make([][]V, 0, 4)
	for item := range value.FieldsXMLWhitespaceStringSeq(lexical) {
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
func AnyValueEqual[V any](left, right []V, equal func(V, V) bool) bool {
	if equal == nil {
		return false
	}
	for _, l := range left {
		for _, r := range right {
			if equal(l, r) {
				return true
			}
		}
	}
	return false
}

// ListValuesEqual reports whether two lists of value variants are equal item-by-item.
func ListValuesEqual[V any](left, right [][]V, equal func(V, V) bool) bool {
	if equal == nil {
		return false
	}
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !AnyValueEqual(left[i], right[i], equal) {
			return false
		}
	}
	return true
}
