package schemair

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
	"github.com/jacoelho/xsd/internal/valuekey"
	"github.com/jacoelho/xsd/internal/xsdlex"
)

type ValueKeyKind = valuekey.Kind

const (
	ValueKeyInvalid  = valuekey.Invalid
	ValueKeyBool     = valuekey.Bool
	ValueKeyDecimal  = valuekey.Decimal
	ValueKeyFloat32  = valuekey.Float32
	ValueKeyFloat64  = valuekey.Float64
	ValueKeyString   = valuekey.String
	ValueKeyBinary   = valuekey.Binary
	ValueKeyQName    = valuekey.QName
	ValueKeyDateTime = valuekey.DateTime
	ValueKeyDuration = valuekey.Duration
	ValueKeyList     = valuekey.List
)

type ValueKey struct {
	Kind  ValueKeyKind
	Bytes []byte
}

type ValueSpecResolver func(TypeRef) (SimpleTypeSpec, bool)

func ValueKeysForLexical(lexical string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	normalized := NormalizeValueLexical(lexical, spec)
	return ValueKeysForNormalized(lexical, normalized, spec, ctx, resolve)
}

func ValueKeysForNormalized(lexical, normalized string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	switch spec.Variety {
	case TypeVarietyList:
		return valueKeysForList(normalized, spec, ctx, resolve)
	case TypeVarietyUnion:
		return valueKeysForUnion(lexical, spec, ctx, resolve)
	default:
		key, err := valueKeyAtomic(normalized, spec, ctx)
		if err != nil {
			return nil, err
		}
		return []ValueKey{key}, nil
	}
}

func NormalizeValueLexical(lexical string, spec SimpleTypeSpec) string {
	normalized := value.NormalizeWhitespace(valueWhitespaceMode(spec.Whitespace), []byte(lexical), nil)
	return string(normalized)
}

func valueKeysForList(normalized string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	if resolve == nil {
		return nil, fmt.Errorf("list type missing item type")
	}
	item, ok := resolve(spec.Item)
	if !ok {
		return nil, fmt.Errorf("list type missing item type")
	}
	count := 0
	for range value.FieldsXMLWhitespaceStringSeq(normalized) {
		count++
	}
	keyBytes := valuekey.AppendUvarint(nil, uint64(count))
	for itemLex := range value.FieldsXMLWhitespaceStringSeq(normalized) {
		itemNorm := NormalizeValueLexical(itemLex, item)
		itemKeys, err := ValueKeysForNormalized(itemLex, itemNorm, item, ctx, resolve)
		if err != nil {
			return nil, err
		}
		if len(itemKeys) == 0 {
			return nil, fmt.Errorf("no value key produced")
		}
		itemKey := itemKeys[0]
		keyBytes = valuekey.AppendListEntry(keyBytes, byte(itemKey.Kind), itemKey.Bytes)
	}
	return []ValueKey{{Kind: ValueKeyList, Bytes: keyBytes}}, nil
}

func valueKeysForUnion(lexical string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	if len(spec.Members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	var out []ValueKey
	for _, ref := range spec.Members {
		if resolve == nil {
			continue
		}
		member, ok := resolve(ref)
		if !ok {
			continue
		}
		memberLex := NormalizeValueLexical(lexical, member)
		if err := validateSpecLexicalValueWithResolver(member, lexical, ctx, resolve, make(map[TypeRef]bool)); err != nil {
			continue
		}
		keys, err := ValueKeysForNormalized(lexical, memberLex, member, ctx, resolve)
		if err != nil {
			continue
		}
		out = append(out, keys...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("union value does not match any member type")
	}
	return out, nil
}

func valueKeyAtomic(normalized string, spec SimpleTypeSpec, ctx map[string]string) (ValueKey, error) {
	primitive := primitiveNameForValueKey(spec)
	if primitive == "decimal" && spec.IntegerDerived {
		if err := validateAtomicLexicalValue(specBuiltinNameForValueKey(spec), normalized, ctx); err != nil {
			return ValueKey{}, err
		}
		intVal, err := num.ParseInt([]byte(normalized))
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid integer")
		}
		return ValueKey{Kind: ValueKeyDecimal, Bytes: num.EncodeDecKey(nil, intVal.AsDec())}, nil
	}
	return valueKeyForPrimitiveName(primitive, normalized, ctx)
}

func valueKeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (ValueKey, error) {
	switch primitive {
	case "anySimpleType", "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return ValueKey{Kind: ValueKeyString, Bytes: valuekey.StringString(0, normalized)}, nil
	case "anyURI":
		return ValueKey{Kind: ValueKeyString, Bytes: valuekey.StringString(1, normalized)}, nil
	case "decimal":
		decVal, err := num.ParseDec([]byte(normalized))
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid decimal")
		}
		return ValueKey{Kind: ValueKeyDecimal, Bytes: num.EncodeDecKey(nil, decVal)}, nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return ValueKey{}, err
		}
		if v {
			return ValueKey{Kind: ValueKeyBool, Bytes: []byte{1}}, nil
		}
		return ValueKey{Kind: ValueKeyBool, Bytes: []byte{0}}, nil
	case "float":
		v, class, err := num.ParseFloat32([]byte(normalized))
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid float")
		}
		return ValueKey{Kind: ValueKeyFloat32, Bytes: valuekey.Float32Bytes(nil, v, class)}, nil
	case "double":
		v, class, err := num.ParseFloat([]byte(normalized), 64)
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid double")
		}
		return ValueKey{Kind: ValueKeyFloat64, Bytes: valuekey.Float64Bytes(nil, v, class)}, nil
	case "duration":
		dur, err := value.ParseDuration(normalized)
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyDuration, Bytes: valuekey.DurationBytes(nil, dur)}, nil
	case "hexBinary":
		b, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyBinary, Bytes: valuekey.BinaryBytes(nil, 0, b)}, nil
	case "base64Binary":
		b, err := value.ParseBase64Binary([]byte(normalized))
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyBinary, Bytes: valuekey.BinaryBytes(nil, 1, b)}, nil
	case "QName":
		qn, err := xsdlex.ParseQNameValue(normalized, ctx)
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyQName, Bytes: valuekey.QNameStrings(0, qn.Namespace, qn.Local)}, nil
	case "NOTATION":
		qn, err := xsdlex.ParseQNameValue(normalized, ctx)
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyQName, Bytes: valuekey.QNameStrings(1, qn.Namespace, qn.Local)}, nil
	default:
		if kind, ok := value.KindFromPrimitiveName(primitive); ok {
			tv, err := value.Parse(kind, []byte(normalized))
			if err != nil {
				return ValueKey{}, err
			}
			key, err := valuekey.TemporalFromValue(nil, tv)
			if err != nil {
				return ValueKey{}, err
			}
			return ValueKey{Kind: ValueKeyDateTime, Bytes: key}, nil
		}
		return ValueKey{}, fmt.Errorf("unsupported primitive type %s", primitive)
	}
}

func primitiveNameForValueKey(spec SimpleTypeSpec) string {
	if spec.Primitive != "" {
		return spec.Primitive
	}
	if spec.Name.Local == "anyType" || spec.Name.Local == "anySimpleType" {
		return "anySimpleType"
	}
	if spec.BuiltinBase != "" {
		return spec.BuiltinBase
	}
	return spec.Name.Local
}

func specBuiltinNameForValueKey(spec SimpleTypeSpec) string {
	if spec.BuiltinBase != "" {
		return spec.BuiltinBase
	}
	if spec.Name.Local != "" {
		return spec.Name.Local
	}
	return primitiveNameForValueKey(spec)
}
