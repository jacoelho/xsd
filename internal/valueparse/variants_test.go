package valueparse

import (
	"fmt"
	"testing"
)

func TestParseUnionValueVariants(t *testing.T) {
	members := []int{1, 2}
	parseMember := func(value string, member int) ([]int, error) {
		switch member {
		case 1:
			if value == "1" {
				return []int{1}, nil
			}
			return nil, fmt.Errorf("not one")
		case 2:
			return []int{len(value)}, nil
		default:
			return nil, fmt.Errorf("unknown member")
		}
	}

	values, err := ParseUnionValueVariants("1", members, parseMember)
	if err != nil {
		t.Fatalf("ParseUnionValueVariants() error = %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("union variants = %d, want 2", len(values))
	}
	if values[0] != 1 || values[1] != 1 {
		t.Fatalf("union variants = %v, want [1 1]", values)
	}

	values, err = ParseUnionValueVariants("abc", members, parseMember)
	if err != nil {
		t.Fatalf("ParseUnionValueVariants() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("union variants = %d, want 1", len(values))
	}
	if values[0] != 3 {
		t.Fatalf("union variant = %v, want [3]", values)
	}
}

func TestParseListValueVariants(t *testing.T) {
	parseItem := func(value string) ([]string, error) {
		return []string{value}, nil
	}

	items, err := ParseListValueVariants("\na b\tc\r\n", parseItem)
	if err != nil {
		t.Fatalf("ParseListValueVariants() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("list items = %d, want 3", len(items))
	}
	if got := items[0][0]; got != "a" {
		t.Fatalf("first item = %q, want %q", got, "a")
	}
	if got := items[1][0]; got != "b" {
		t.Fatalf("second item = %q, want %q", got, "b")
	}
	if got := items[2][0]; got != "c" {
		t.Fatalf("third item = %q, want %q", got, "c")
	}
}

func TestListAndAnyValueEqualComparator(t *testing.T) {
	eq := func(left, right string) bool { return left == right }

	if !AnyValueEqual([]string{"x", "y"}, []string{"a", "y"}, eq) {
		t.Fatal("AnyValueEqual() = false, want true")
	}
	if ListValuesEqual([][]string{{"a"}, {"b"}}, [][]string{{"a"}, {"c"}}, eq) {
		t.Fatal("ListValuesEqual() = true, want false")
	}
	if !ListValuesEqual([][]string{{"a"}, {"b"}}, [][]string{{"a"}, {"b"}}, eq) {
		t.Fatal("ListValuesEqual() = false, want true")
	}
}
