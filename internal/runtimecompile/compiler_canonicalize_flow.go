package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) canonicalizeNormalized(lexical, normalized string, typ types.Type, ctx map[string]string) ([]byte, error) {
	return c.canonicalizeNormalizedCore(lexical, normalized, typ, ctx, canonicalizeGeneral)
}

func (c *compiler) canonicalizeNormalizedCore(lexical, normalized string, typ types.Type, ctx map[string]string, mode canonicalizeMode) ([]byte, error) {
	switch c.res.varietyForType(typ) {
	case types.ListVariety:
		item, ok := c.res.listItemTypeFromType(typ)
		if !ok || item == nil {
			return nil, fmt.Errorf("list type missing item type")
		}
		var buf []byte
		count := 0
		for itemLex := range types.FieldsXMLWhitespaceSeq(normalized) {
			itemNorm := c.normalizeLexical(itemLex, item)
			canon, err := c.canonicalizeNormalizedCore(itemLex, itemNorm, item, ctx, mode)
			if err != nil {
				return nil, err
			}
			if count > 0 {
				buf = append(buf, ' ')
			}
			buf = append(buf, canon...)
			count++
		}
		if count == 0 {
			return []byte{}, nil
		}
		return buf, nil
	case types.UnionVariety:
		members := c.res.unionMemberTypesFromType(typ)
		if len(members) == 0 {
			return nil, fmt.Errorf("union has no member types")
		}
		for _, member := range members {
			memberLex := c.normalizeLexical(lexical, member)
			memberFacets, facetErr := c.facetsForType(member)
			if facetErr != nil {
				return nil, facetErr
			}
			switch mode {
			case canonicalizeDefault:
				if validateErr := c.validatePartialFacets(memberLex, member, memberFacets); validateErr != nil {
					continue
				}
				canon, canonErr := c.canonicalizeNormalizedCore(lexical, memberLex, member, ctx, mode)
				if canonErr != nil {
					continue
				}
				if enumErr := c.validateEnumSets(lexical, memberLex, member, ctx); enumErr != nil {
					continue
				}
				return canon, nil
			default:
				if validateErr := c.validateMemberFacets(memberLex, member, memberFacets, ctx, true); validateErr != nil {
					continue
				}
				canon, canonErr := c.canonicalizeNormalizedCore(lexical, memberLex, member, ctx, mode)
				if canonErr == nil {
					return canon, nil
				}
			}
		}
		return nil, fmt.Errorf("union value does not match any member type")
	default:
		return c.canonicalizeAtomic(normalized, typ, ctx)
	}
}
