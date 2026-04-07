package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/value"
)

func fixedValuesEqual(schema *parser.Schema, attr, target *model.AttributeDecl) (bool, error) {
	resolvedType := typeresolve.ResolveTypeReference(schema, target.Type, typeresolve.TypeReferenceAllowMissing)
	if resolvedType == nil {
		return attr.Fixed == target.Fixed, nil
	}

	left, err := model.NormalizeTypeValue(attr.Fixed, resolvedType)
	if err != nil {
		return false, err
	}
	right, err := model.NormalizeTypeValue(target.Fixed, resolvedType)
	if err != nil {
		return false, err
	}

	if model.IsQNameOrNotationType(resolvedType) {
		leftQName, qerr := model.ParseQNameValue(left, attr.FixedContext)
		if qerr != nil {
			return false, qerr
		}
		rightQName, qerr := model.ParseQNameValue(right, target.FixedContext)
		if qerr != nil {
			return false, qerr
		}
		return leftQName == rightQName, nil
	}

	if itemType, ok := model.ListItemType(resolvedType); ok {
		if itemType == nil {
			return false, fmt.Errorf("list item type is nil")
		}
		leftItems, lerr := value.ParseListValueVariants(left, func(item string) ([]model.TypedValue, error) {
			return parseValueVariants(schema, item, itemType, attr.FixedContext)
		})
		if lerr != nil {
			return false, lerr
		}
		rightItems, rerr := value.ParseListValueVariants(right, func(item string) ([]model.TypedValue, error) {
			return parseValueVariants(schema, item, itemType, target.FixedContext)
		})
		if rerr != nil {
			return false, rerr
		}
		return value.ListValuesEqual(leftItems, rightItems, model.CompareTypedValues), nil
	}

	leftValues, err := parseValueVariants(schema, left, resolvedType, attr.FixedContext)
	if err != nil {
		return false, err
	}
	rightValues, err := parseValueVariants(schema, right, resolvedType, target.FixedContext)
	if err != nil {
		return false, err
	}
	return value.AnyValueEqual(leftValues, rightValues, model.CompareTypedValues), nil
}

func parseValueVariants(schema *parser.Schema, lexical string, typ model.Type, context map[string]string) ([]model.TypedValue, error) {
	if st, ok := typ.(*model.SimpleType); ok && st.Variety() == model.UnionVariety {
		memberTypes := typeresolve.ResolveUnionMemberTypes(schema, st)
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
	default:
		return nil, fmt.Errorf("unsupported type %T", typ)
	}
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
