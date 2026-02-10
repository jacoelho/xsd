package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/valuecodec"
)

type keyBytes struct {
	bytes []byte
	kind  runtime.ValueKind
}

func (c *compiler) valueKeysForNormalized(lexical, normalized string, typ model.Type, ctx map[string]string) ([]runtime.ValueKey, error) {
	keys, err := c.keyBytesForNormalized(lexical, normalized, typ, ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.ValueKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, c.makeValueKey(key.kind, key.bytes))
	}
	return out, nil
}

func (c *compiler) keyBytesForNormalized(lexical, normalized string, typ model.Type, ctx map[string]string) ([]keyBytes, error) {
	switch c.res.varietyForType(typ) {
	case model.ListVariety:
		return c.keyBytesForList(normalized, typ, ctx)
	case model.UnionVariety:
		return c.keyBytesForUnion(lexical, typ, ctx)
	default:
		key, err := c.keyBytesAtomic(normalized, typ, ctx)
		if err != nil {
			return nil, err
		}
		return []keyBytes{key}, nil
	}
}

func (c *compiler) keyBytesForNormalizedSingle(normalized string, typ model.Type, ctx map[string]string) (keyBytes, error) {
	keys, err := c.keyBytesForNormalized(normalized, normalized, typ, ctx)
	if err != nil {
		return keyBytes{}, err
	}
	if len(keys) == 0 {
		return keyBytes{}, fmt.Errorf("no value key produced")
	}
	return keys[0], nil
}

func (c *compiler) keyBytesForList(normalized string, typ model.Type, ctx map[string]string) ([]keyBytes, error) {
	item, ok := c.res.listItemTypeFromType(typ)
	if !ok || item == nil {
		return nil, fmt.Errorf("list type missing item type")
	}
	count := 0
	for range model.FieldsXMLWhitespaceSeq(normalized) {
		count++
	}
	keyBytesBuf := valuecodec.AppendUvarint(nil, uint64(count))
	for itemLex := range model.FieldsXMLWhitespaceSeq(normalized) {
		itemKey, err := c.keyBytesForNormalizedSingle(itemLex, item, ctx)
		if err != nil {
			return nil, err
		}
		keyBytesBuf = valuecodec.AppendListEntry(keyBytesBuf, byte(itemKey.kind), itemKey.bytes)
	}
	return []keyBytes{{kind: runtime.VKList, bytes: keyBytesBuf}}, nil
}

func (c *compiler) keyBytesForUnion(lexical string, typ model.Type, ctx map[string]string) ([]keyBytes, error) {
	members := c.res.unionMemberTypesFromType(typ)
	if len(members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	var out []keyBytes
	for _, member := range members {
		memberLex := c.normalizeLexical(lexical, member)
		memberFacets, err := c.facetsForType(member)
		if err != nil {
			return nil, err
		}
		if validateErr := c.validateMemberFacets(memberLex, member, memberFacets, ctx, true); validateErr != nil {
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
