package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

type keyBytes struct {
	bytes []byte
	kind  runtime.ValueKind
}

func (c *artifactCompiler) valueKeysForNormalized(lexical, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]runtime.ValueKey, error) {
	keys, err := c.keyBytesForNormalized(lexical, normalized, spec, ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.ValueKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, c.makeValueKey(key.kind, key.bytes))
	}
	return out, nil
}

func (c *artifactCompiler) keyBytesForNormalized(lexical, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]keyBytes, error) {
	switch spec.Variety {
	case schemair.TypeVarietyList:
		return c.keyBytesForList(normalized, spec, ctx)
	case schemair.TypeVarietyUnion:
		return c.keyBytesForUnion(lexical, spec, ctx)
	default:
		key, err := keyBytesAtomic(normalized, spec, ctx)
		if err != nil {
			return nil, err
		}
		return []keyBytes{key}, nil
	}
}

func (c *artifactCompiler) keyBytesForNormalizedSingle(normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) (keyBytes, error) {
	keys, err := c.keyBytesForNormalized(normalized, normalized, spec, ctx)
	if err != nil {
		return keyBytes{}, err
	}
	if len(keys) == 0 {
		return keyBytes{}, fmt.Errorf("no value key produced")
	}
	return keys[0], nil
}

func (c *artifactCompiler) keyBytesForList(normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]keyBytes, error) {
	item, ok := c.specForRef(spec.Item)
	if !ok {
		return nil, fmt.Errorf("list type missing item type")
	}
	count := 0
	for range value.FieldsXMLWhitespaceStringSeq(normalized) {
		count++
	}
	keyBytesBuf := runtime.AppendUvarint(nil, uint64(count))
	for itemLex := range value.FieldsXMLWhitespaceStringSeq(normalized) {
		itemKey, err := c.keyBytesForNormalizedSingle(itemLex, item, ctx)
		if err != nil {
			return nil, err
		}
		keyBytesBuf = runtime.AppendListEntry(keyBytesBuf, byte(itemKey.kind), itemKey.bytes)
	}
	return []keyBytes{{kind: runtime.VKList, bytes: keyBytesBuf}}, nil
}

func (c *artifactCompiler) keyBytesForUnion(lexical string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]keyBytes, error) {
	if len(spec.Members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	var out []keyBytes
	for _, ref := range spec.Members {
		member, ok := c.specForRef(ref)
		if !ok {
			continue
		}
		memberLex := c.normalizeLexical(lexical, member)
		if err := c.validateMemberFacets(memberLex, member, member.Facets, ctx, true); err != nil {
			continue
		}
		keys, err := c.keyBytesForNormalized(lexical, memberLex, member, ctx)
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

func keyBytesAtomic(normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) (keyBytes, error) {
	primitive := primitiveForSpec(spec)
	if primitive == "decimal" && spec.IntegerDerived {
		intVal, parseErr := parseInt(normalized)
		if parseErr != nil {
			return keyBytes{}, parseErr
		}
		if validateErr := runtime.ValidateIntegerKind(integerKindForBuiltin(specBuiltinName(spec)), intVal); validateErr != nil {
			return keyBytes{}, validateErr
		}
		return keyBytes{kind: runtime.VKDecimal, bytes: num.EncodeDecKey(nil, intVal.AsDec())}, nil
	}

	kind, bytes, err := runtime.KeyForPrimitiveName(primitive, normalized, ctx)
	if err != nil {
		return keyBytes{}, err
	}
	return keyBytes{kind: kind, bytes: bytes}, nil
}

func parseInt(normalized string) (num.Int, error) {
	val, err := num.ParseInt([]byte(normalized))
	if err != nil {
		return num.Int{}, fmt.Errorf("invalid integer")
	}
	return val, nil
}

func (c *artifactCompiler) makeValueKey(kind runtime.ValueKind, bytes []byte) runtime.ValueKey {
	copied := append([]byte(nil), bytes...)
	return runtime.ValueKey{
		Kind:  kind,
		Bytes: copied,
		Hash:  runtime.HashKey(kind, copied),
	}
}
