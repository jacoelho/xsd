package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	qnamelex "github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/valueparse"
)

func fixedValuesEqual(schema *parser.Schema, attr, target *types.AttributeDecl) (bool, error) {
	resolvedType := typeops.ResolveTypeReference(schema, target.Type, typeops.TypeReferenceAllowMissing)
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
		leftQName, qerr := qnamelex.ParseQNameValue(left, attr.FixedContext)
		if qerr != nil {
			return false, qerr
		}
		rightQName, qerr := qnamelex.ParseQNameValue(right, target.FixedContext)
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
		return valueparse.ListValuesEqual(leftItems, rightItems), nil
	}

	leftValues, err := parseValueVariants(schema, left, resolvedType, attr.FixedContext)
	if err != nil {
		return false, err
	}
	rightValues, err := parseValueVariants(schema, right, resolvedType, target.FixedContext)
	if err != nil {
		return false, err
	}
	return valueparse.AnyValueEqual(leftValues, rightValues), nil
}

func parseValueVariants(schema *parser.Schema, lexical string, typ types.Type, context map[string]string) ([]types.TypedValue, error) {
	if st, ok := typ.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
		memberTypes := typeops.ResolveUnionMemberTypes(schema, st)
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
	if types.IsQNameOrNotationType(typ) {
		normalized := types.NormalizeWhiteSpace(lexical, typ)
		qname, err := qnamelex.ParseQNameValue(normalized, context)
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
