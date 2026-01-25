package resolver

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func fixedValuesEqual(schema *parser.Schema, attr, target *types.AttributeDecl) (bool, error) {
	resolvedType := resolveTypeForFinalValidation(schema, target.Type)
	if resolvedType == nil {
		return attr.Fixed == target.Fixed, nil
	}

	left, err := types.NormalizeValue(attr.Fixed, resolvedType)
	if err != nil {
		return false, err
	}
	right, err := types.NormalizeValue(target.Fixed, resolvedType)
	if err != nil {
		return false, err
	}

	if types.IsQNameOrNotationType(resolvedType) {
		leftQName, qerr := types.ParseQNameValue(left, attr.FixedContext)
		if qerr != nil {
			return false, qerr
		}
		rightQName, qerr := types.ParseQNameValue(right, target.FixedContext)
		if qerr != nil {
			return false, qerr
		}
		return leftQName == rightQName, nil
	}

	if itemType, ok := types.ListItemType(resolvedType); ok {
		leftItems, lerr := parseListValueVariants(schema, left, itemType, attr.FixedContext)
		if lerr != nil {
			return false, lerr
		}
		rightItems, rerr := parseListValueVariants(schema, right, itemType, target.FixedContext)
		if rerr != nil {
			return false, rerr
		}
		return listValuesEqual(leftItems, rightItems), nil
	}

	leftValues, err := parseValueVariants(schema, left, resolvedType, attr.FixedContext)
	if err != nil {
		return false, err
	}
	rightValues, err := parseValueVariants(schema, right, resolvedType, target.FixedContext)
	if err != nil {
		return false, err
	}
	return anyValueEqual(leftValues, rightValues), nil
}

func parseValueVariants(schema *parser.Schema, lexical string, typ types.Type, context map[string]string) ([]types.TypedValue, error) {
	if st, ok := typ.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
		memberTypes := resolveUnionMemberTypes(schema, st)
		return parseUnionValueVariants(lexical, memberTypes, context)
	}
	typed, err := parseTypedValueWithContext(lexical, typ, context)
	if err != nil {
		return nil, err
	}
	return []types.TypedValue{typed}, nil
}

func parseUnionValueVariants(lexical string, memberTypes []types.Type, context map[string]string) ([]types.TypedValue, error) {
	if len(memberTypes) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	values := make([]types.TypedValue, 0, len(memberTypes))
	var firstErr error
	for _, memberType := range memberTypes {
		typed, err := parseTypedValueWithContext(lexical, memberType, context)
		if err == nil {
			values = append(values, typed)
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

func parseListValueVariants(schema *parser.Schema, lexical string, itemType types.Type, context map[string]string) ([][]types.TypedValue, error) {
	if itemType == nil {
		return nil, fmt.Errorf("list item type is nil")
	}
	items := splitXMLWhitespaceFields(lexical)
	if len(items) == 0 {
		return nil, nil
	}
	parsed := make([][]types.TypedValue, len(items))
	for i, item := range items {
		values, err := parseValueVariants(schema, item, itemType, context)
		if err != nil {
			return nil, fmt.Errorf("invalid list item %q: %w", item, err)
		}
		parsed[i] = values
	}
	return parsed, nil
}

func parseTypedValueWithContext(lexical string, typ types.Type, context map[string]string) (types.TypedValue, error) {
	if types.IsQNameOrNotationType(typ) {
		normalized := types.NormalizeWhiteSpace(lexical, typ)
		qname, err := types.ParseQNameValue(normalized, context)
		if err != nil {
			return nil, err
		}
		return qnameTypedValue{typ: typ, lexical: normalized, value: qname}, nil
	}
	switch t := typ.(type) {
	case *types.SimpleType:
		return t.ParseValue(lexical)
	case *types.BuiltinType:
		return t.ParseValue(lexical)
	default:
		return nil, fmt.Errorf("unsupported type %T", typ)
	}
}

type qnameTypedValue struct {
	typ     types.Type
	lexical string
	value   types.QName
}

func (v qnameTypedValue) Type() types.Type { return v.typ }
func (v qnameTypedValue) Lexical() string  { return v.lexical }
func (v qnameTypedValue) Native() any      { return v.value }
func (v qnameTypedValue) String() string   { return v.lexical }

func anyValueEqual(left, right []types.TypedValue) bool {
	for _, l := range left {
		for _, r := range right {
			if types.ValuesEqual(l, r) {
				return true
			}
		}
	}
	return false
}

func listValuesEqual(left, right [][]types.TypedValue) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !anyValueEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func splitXMLWhitespaceFields(value string) []string {
	if value == "" {
		return nil
	}
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}
