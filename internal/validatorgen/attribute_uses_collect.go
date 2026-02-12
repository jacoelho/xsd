package validatorgen

import (
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	qnameorder "github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/typechain"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// CollectAttributeUses resolves effective attribute uses and wildcard.
// It follows complex-type derivation from base to leaf in deterministic order.
func CollectAttributeUses(schema *parser.Schema, ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error) {
	if schema == nil || ct == nil {
		return nil, nil, nil
	}
	attrMap := make(map[model.QName]*model.AttributeDecl)
	chain := typechain.CollectComplexTypeChain(schema, ct, typechain.ComplexTypeChainExplicitBaseOnly)
	var wildcard *model.AnyAttribute
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
	out := make([]*model.AttributeDecl, 0, len(attrMap))
	for _, decl := range attrMap {
		out = append(out, decl)
	}
	slices.SortFunc(out, func(a, b *model.AttributeDecl) int {
		left := typeresolve.EffectiveAttributeQName(schema, a)
		right := typeresolve.EffectiveAttributeQName(schema, b)
		return qnameorder.Compare(left, right)
	})
	return out, wildcard, nil
}

func mergeAttributesFromComplexType(schema *parser.Schema, ct *model.ComplexType, attrMap map[model.QName]*model.AttributeDecl) error {
	if ct == nil {
		return nil
	}
	if err := mergeAttributes(schema, ct.Attributes(), ct.AttrGroups, attrMap); err != nil {
		return err
	}
	content := ct.Content()
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := mergeAttributes(schema, ext.Attributes, ext.AttrGroups, attrMap); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := mergeAttributes(schema, restr.Attributes, restr.AttrGroups, attrMap); err != nil {
			return err
		}
	}
	return nil
}
