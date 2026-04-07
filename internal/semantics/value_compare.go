package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/value"
)

func fixedValuesEqual(schema *parser.Schema, attr, target *model.AttributeDecl) (bool, error) {
	resolvedType := parser.ResolveTypeReferenceAllowMissing(schema, target.Type)
	if resolvedType == nil {
		return attr.Fixed == target.Fixed, nil
	}

	left, right, err := normalizeFixedComparisonValues(attr, target, resolvedType)
	if err != nil {
		return false, err
	}

	if model.IsQNameOrNotationType(resolvedType) {
		return compareQNameFixedValues(left, right, attr.FixedContext, target.FixedContext)
	}

	if itemType, ok := model.ListItemType(resolvedType); ok {
		return compareListFixedValues(schema, left, right, itemType, attr.FixedContext, target.FixedContext)
	}

	return compareTypedFixedValues(schema, left, right, resolvedType, attr.FixedContext, target.FixedContext)
}

func normalizeFixedComparisonValues(attr, target *model.AttributeDecl, resolvedType model.Type) (string, string, error) {
	left, err := model.NormalizeTypeValue(attr.Fixed, resolvedType)
	if err != nil {
		return "", "", err
	}
	right, err := model.NormalizeTypeValue(target.Fixed, resolvedType)
	if err != nil {
		return "", "", err
	}
	return left, right, nil
}

func compareQNameFixedValues(left, right string, leftContext, rightContext map[string]string) (bool, error) {
	leftQName, err := model.ParseQNameValue(left, leftContext)
	if err != nil {
		return false, err
	}
	rightQName, err := model.ParseQNameValue(right, rightContext)
	if err != nil {
		return false, err
	}
	return leftQName == rightQName, nil
}

func compareListFixedValues(
	schema *parser.Schema,
	left string,
	right string,
	itemType model.Type,
	leftContext map[string]string,
	rightContext map[string]string,
) (bool, error) {
	if itemType == nil {
		return false, fmt.Errorf("list item type is nil")
	}
	leftItems, err := value.ParseListValueVariants(left, func(item string) ([]model.TypedValue, error) {
		return parseValueVariants(schema, item, itemType, leftContext)
	})
	if err != nil {
		return false, err
	}
	rightItems, err := value.ParseListValueVariants(right, func(item string) ([]model.TypedValue, error) {
		return parseValueVariants(schema, item, itemType, rightContext)
	})
	if err != nil {
		return false, err
	}
	return value.ListValuesEqual(leftItems, rightItems, model.CompareTypedValues), nil
}

func compareTypedFixedValues(
	schema *parser.Schema,
	left string,
	right string,
	resolvedType model.Type,
	leftContext map[string]string,
	rightContext map[string]string,
) (bool, error) {
	leftValues, err := parseValueVariants(schema, left, resolvedType, leftContext)
	if err != nil {
		return false, err
	}
	rightValues, err := parseValueVariants(schema, right, resolvedType, rightContext)
	if err != nil {
		return false, err
	}
	return value.AnyValueEqual(leftValues, rightValues, model.CompareTypedValues), nil
}

func parseValueVariants(schema *parser.Schema, lexical string, typ model.Type, context map[string]string) ([]model.TypedValue, error) {
	if st, ok := typ.(*model.SimpleType); ok && st.Variety() == model.UnionVariety {
		memberTypes := parser.ResolveUnionMemberTypes(schema, st)
		return value.ParseUnionValueVariants(lexical, memberTypes, func(value string, member model.Type) ([]model.TypedValue, error) {
			typed, err := parseTypedValueWithContext(value, member, context)
			if err != nil {
				return nil, err
			}
			return []model.TypedValue{typed}, nil
		})
	}
	typed, err := parseTypedValueWithContext(lexical, typ, context)
	if err != nil {
		return nil, err
	}
	return []model.TypedValue{typed}, nil
}

func parseTypedValueWithContext(lexical string, typ model.Type, context map[string]string) (model.TypedValue, error) {
	if model.IsQNameOrNotationType(typ) {
		normalized := model.NormalizeWhiteSpace(lexical, typ)
		parsedQName, err := model.ParseQNameValue(normalized, context)
		if err != nil {
			return nil, err
		}
		return qnameTypedValue{typ: typ, lexical: normalized, value: parsedQName}, nil
	}
	switch t := typ.(type) {
	case *model.SimpleType:
		return t.ParseValue(lexical)
	case *model.BuiltinType:
		return t.ParseValue(lexical)
	}
	return nil, fmt.Errorf("unsupported type %T", typ)
}

type qnameTypedValue struct {
	typ     model.Type
	lexical string
	value   model.QName
}

func (v qnameTypedValue) Type() model.Type { return v.typ }
func (v qnameTypedValue) Lexical() string  { return v.lexical }
func (v qnameTypedValue) Native() any      { return v.value }
func (v qnameTypedValue) String() string   { return v.lexical }
