package validatorcompile

import (
	"cmp"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typegraph"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

type attrCollectionMode uint8

const (
	attrMerge attrCollectionMode = iota
	attrRestriction
)

// CollectAttributeUses resolves effective attribute uses and wildcard.
// It follows complex-type derivation from base to leaf in deterministic order.
func CollectAttributeUses(schema *parser.Schema, ct *types.ComplexType) ([]*types.AttributeDecl, *types.AnyAttribute, error) {
	if schema == nil || ct == nil {
		return nil, nil, nil
	}
	attrMap := make(map[types.QName]*types.AttributeDecl)
	chain := typegraph.CollectComplexTypeChain(schema, ct, typegraph.ComplexTypeChainAllowImplicitAnyType)
	var wildcard *types.AnyAttribute
	for i := len(chain) - 1; i >= 0; i-- {
		current := chain[i]
		if err := mergeAttributesFromComplexType(schema, current, attrMap); err != nil {
			return nil, nil, err
		}
		localWildcard, err := localAttributeWildcard(schema, current)
		if err != nil {
			return nil, nil, err
		}
		if i == len(chain)-1 {
			wildcard = localWildcard
		} else {
			wildcard, err = applyDerivedWildcard(wildcard, localWildcard, current)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	out := make([]*types.AttributeDecl, 0, len(attrMap))
	for _, decl := range attrMap {
		out = append(out, decl)
	}
	slices.SortFunc(out, func(a, b *types.AttributeDecl) int {
		left := typeops.EffectiveAttributeQName(schema, a)
		right := typeops.EffectiveAttributeQName(schema, b)
		if left.Namespace != right.Namespace {
			return cmp.Compare(left.Namespace, right.Namespace)
		}
		return cmp.Compare(left.Local, right.Local)
	})
	return out, wildcard, nil
}

func mergeAttributesFromComplexType(schema *parser.Schema, ct *types.ComplexType, attrMap map[types.QName]*types.AttributeDecl) error {
	if ct == nil {
		return nil
	}
	if err := mergeAttributes(schema, ct.Attributes(), ct.AttrGroups, attrMap, attrMerge); err != nil {
		return err
	}
	content := ct.Content()
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := mergeAttributes(schema, ext.Attributes, ext.AttrGroups, attrMap, attrMerge); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := mergeAttributes(schema, restr.Attributes, restr.AttrGroups, attrMap, attrRestriction); err != nil {
			return err
		}
	}
	return nil
}
