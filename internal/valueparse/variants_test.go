package valueparse

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestParseUnionValueVariants(t *testing.T) {
	members := []types.Type{
		types.GetBuiltin(types.TypeNameInt),
		types.GetBuiltin(types.TypeNameString),
	}

	parseMember := func(value string, typ types.Type) ([]types.TypedValue, error) {
		switch t := typ.(type) {
		case *types.BuiltinType:
			v, err := t.ParseValue(value)
			if err != nil {
				return nil, err
			}
			return []types.TypedValue{v}, nil
		case *types.SimpleType:
			v, err := t.ParseValue(value)
			if err != nil {
				return nil, err
			}
			return []types.TypedValue{v}, nil
		default:
			return nil, nil
		}
	}

	values, err := ParseUnionValueVariants("1", members, parseMember)
	if err != nil {
		t.Fatalf("ParseUnionValueVariants() error = %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("union variants = %d, want 2", len(values))
	}

	values, err = ParseUnionValueVariants("abc", members, parseMember)
	if err != nil {
		t.Fatalf("ParseUnionValueVariants() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("union variants = %d, want 1", len(values))
	}
}

func TestParseListValueVariants(t *testing.T) {
	parseItem := func(value string) ([]types.TypedValue, error) {
		bt := types.GetBuiltin(types.TypeNameString)
		v, err := bt.ParseValue(value)
		if err != nil {
			return nil, err
		}
		return []types.TypedValue{v}, nil
	}

	items, err := ParseListValueVariants("a b\tc", parseItem)
	if err != nil {
		t.Fatalf("ParseListValueVariants() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("list items = %d, want 3", len(items))
	}
}
