package types

import "testing"

func TestParseUnionValueVariants(t *testing.T) {
	members := []Type{
		GetBuiltin(TypeNameInt),
		GetBuiltin(TypeNameString),
	}

	parseMember := func(value string, typ Type) ([]TypedValue, error) {
		switch t := typ.(type) {
		case *BuiltinType:
			v, err := t.ParseValue(value)
			if err != nil {
				return nil, err
			}
			return []TypedValue{v}, nil
		case *SimpleType:
			v, err := t.ParseValue(value)
			if err != nil {
				return nil, err
			}
			return []TypedValue{v}, nil
		default:
			return nil, nil
		}
	}

	values, err := ParseUnionValueVariants("1", members, parseMember)
	if err != nil {
		t.Fatalf("ParseUnionValueVariants error = %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(values))
	}

	values, err = ParseUnionValueVariants("abc", members, parseMember)
	if err != nil {
		t.Fatalf("ParseUnionValueVariants error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(values))
	}

	if _, err := ParseUnionValueVariants("1", nil, parseMember); err == nil {
		t.Fatalf("expected error for empty union members")
	}
}

func TestParseListValueVariants(t *testing.T) {
	parseItem := func(value string) ([]TypedValue, error) {
		bt := GetBuiltin(TypeNameString)
		v, err := bt.ParseValue(value)
		if err != nil {
			return nil, err
		}
		return []TypedValue{v}, nil
	}

	items, err := ParseListValueVariants("a b\tc", parseItem)
	if err != nil {
		t.Fatalf("ParseListValueVariants error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 list items, got %d", len(items))
	}

}
