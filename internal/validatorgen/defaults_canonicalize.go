package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *compiler) canonicalizeDefaultFixed(lexical string, typ model.Type, ctx map[string]string) ([]byte, runtime.ValidatorID, runtime.ValueKeyRef, error) {
	normalized := c.normalizeLexical(lexical, typ)
	facets, err := c.facetsForType(typ)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	err = c.validatePartialFacets(normalized, typ, facets)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	canon, memberType, err := c.canonicalizeNormalizedDefaultWithMember(lexical, normalized, typ, ctx)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	enumErr := c.validateEnumSets(lexical, normalized, typ, ctx)
	if enumErr != nil {
		return nil, 0, runtime.ValueKeyRef{}, enumErr
	}
	keyRef, err := c.defaultFixedKeyRef(lexical, normalized, typ, memberType, ctx)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	memberID := runtime.ValidatorID(0)
	if memberType != nil {
		memberID, err = c.compileType(memberType)
		if err != nil {
			return nil, 0, runtime.ValueKeyRef{}, err
		}
	}
	return canon, memberID, keyRef, nil
}

func (c *compiler) defaultFixedKeyRef(lexical, normalized string, typ, memberType model.Type, ctx map[string]string) (runtime.ValueKeyRef, error) {
	keyType := typ
	keyLexical := normalized
	if memberType != nil {
		keyType = memberType
		keyLexical = c.normalizeLexical(lexical, memberType)
	}
	keys, err := c.valueKeysForNormalized(lexical, keyLexical, keyType, ctx)
	if err != nil {
		return runtime.ValueKeyRef{}, err
	}
	if len(keys) == 0 {
		return runtime.ValueKeyRef{}, fmt.Errorf("no value key produced")
	}
	key := keys[0]
	return runtime.ValueKeyRef{
		Kind: key.Kind,
		Ref:  c.values.addWithHash(key.Bytes, runtime.HashBytes(key.Bytes)),
	}, nil
}

func (c *compiler) canonicalizeNormalizedDefaultWithMember(lexical, normalized string, typ model.Type, ctx map[string]string) ([]byte, model.Type, error) {
	if c.res.varietyForType(typ) != model.UnionVariety {
		canon, err := c.canonicalizeNormalizedCore(lexical, normalized, typ, ctx, canonicalizeDefault)
		return canon, nil, err
	}
	members := c.res.unionMemberTypesFromType(typ)
	if len(members) == 0 {
		return nil, nil, fmt.Errorf("union has no member types")
	}
	for _, member := range members {
		memberLex := c.normalizeLexical(lexical, member)
		memberFacets, err := c.facetsForType(member)
		if err != nil {
			return nil, nil, err
		}
		if validateErr := c.validatePartialFacets(memberLex, member, memberFacets); validateErr != nil {
			continue
		}
		canon, canonErr := c.canonicalizeNormalizedCore(lexical, memberLex, member, ctx, canonicalizeDefault)
		if canonErr != nil {
			continue
		}
		if enumErr := c.validateEnumSets(lexical, memberLex, member, ctx); enumErr != nil {
			continue
		}
		return canon, member, nil
	}
	return nil, nil, fmt.Errorf("union value does not match any member type")
}
