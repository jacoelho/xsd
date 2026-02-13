package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/facetvalue"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/valueparse"
)

func fixedValuesEqual(schema *parser.Schema, attr, target *types.AttributeDecl) (bool, error) {
	resolvedType := typeresolve.ResolveTypeReference(schema, target.Type, typeresolve.TypeReferenceAllowMissing)
	if resolvedType == nil {
		return attr.Fixed == target.Fixed, nil
	}

	left, err := types.NormalizeTypeValue(attr.Fixed, resolvedType)
	if err != nil {
		return false, err
	}
	right, err := types.NormalizeTypeValue(target.Fixed, resolvedType)
	if err != nil {
		return false, err
	}

	if facetvalue.IsQNameOrNotationType(resolvedType) {
		leftQName, qerr := qname.ParseQNameValue(left, attr.FixedContext)
		if qerr != nil {
			return false, qerr
		}
		rightQName, qerr := qname.ParseQNameValue(right, target.FixedContext)
		if qerr != nil {
			return false, qerr
		}
		return leftQName == rightQName, nil
	}

	if itemType, ok := types.ListItemType(resolvedType); ok {
		if itemType == nil {
			return false, fmt.Errorf("list item type is nil")
		}
		leftItems, lerr := valueparse.ParseListValueVariants(left, func(item string) ([]types.TypedValue, error) {
			return parseValueVariants(schema, item, itemType, attr.FixedContext)
		})
		if lerr != nil {
			return false, lerr
		}
		rightItems, rerr := valueparse.ParseListValueVariants(right, func(item string) ([]types.TypedValue, error) {
			return parseValueVariants(schema, item, itemType, target.FixedContext)
		})
		if rerr != nil {
			return false, rerr
		}
		return valueparse.ListValuesEqual(leftItems, rightItems, facetvalue.ValuesEqual), nil
	}

	leftValues, err := parseValueVariants(schema, left, resolvedType, attr.FixedContext)
	if err != nil {
		return false, err
	}
	rightValues, err := parseValueVariants(schema, right, resolvedType, target.FixedContext)
	if err != nil {
		return false, err
	}
	return valueparse.AnyValueEqual(leftValues, rightValues, facetvalue.ValuesEqual), nil
}

func parseValueVariants(schema *parser.Schema, lexical string, typ types.Type, context map[string]string) ([]types.TypedValue, error) {
	if st, ok := typ.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
		memberTypes := typeresolve.ResolveUnionMemberTypes(schema, st)
		return valueparse.ParseUnionValueVariants(lexical, memberTypes, func(value string, member types.Type) ([]types.TypedValue, error) {
			typed, err := parseTypedValueWithContext(value, member, context)
			if err != nil {
				return nil, err
			}
			return []types.TypedValue{typed}, nil
		})
	}
	typed, err := parseTypedValueWithContext(lexical, typ, context)
	if err != nil {
		return nil, err
	}
	return []types.TypedValue{typed}, nil
}

func parseTypedValueWithContext(lexical string, typ types.Type, context map[string]string) (types.TypedValue, error) {
	if facetvalue.IsQNameOrNotationType(typ) {
		normalized := types.NormalizeWhiteSpace(lexical, typ)
		parsedQName, err := qname.ParseQNameValue(normalized, context)
		if err != nil {
			return nil, err
		}
		return qnameTypedValue{typ: typ, lexical: normalized, value: parsedQName}, nil
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
