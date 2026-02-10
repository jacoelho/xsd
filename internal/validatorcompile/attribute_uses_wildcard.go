package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func localAttributeWildcard(schema *parser.Schema, ct *model.ComplexType) (*model.AnyAttribute, error) {
	if schema == nil || ct == nil {
		return nil, nil
	}
	var groups []model.QName
	var explicit []*model.AnyAttribute

	groups = append(groups, ct.AttrGroups...)
	if anyAttr := ct.AnyAttribute(); anyAttr != nil {
		explicit = append(explicit, anyAttr)
	}

	if content := ct.Content(); content != nil {
		if ext := content.ExtensionDef(); ext != nil {
			groups = append(groups, ext.AttrGroups...)
			if ext.AnyAttribute != nil {
				explicit = append(explicit, ext.AnyAttribute)
			}
		}
		if restr := content.RestrictionDef(); restr != nil {
			groups = append(groups, restr.AttrGroups...)
			if restr.AnyAttribute != nil {
				explicit = append(explicit, restr.AnyAttribute)
			}
		}
	}

	groupWildcard, err := collectAttributeGroupWildcard(schema, groups)
	if err != nil {
		return nil, err
	}
	wildcard := groupWildcard
	for _, anyAttr := range explicit {
		var err error
		wildcard, err = intersectLocalAnyAttribute(anyAttr, wildcard)
		if err != nil {
			return nil, err
		}
	}
	return wildcard, nil
}

func collectAttributeGroupWildcard(schema *parser.Schema, groups []model.QName) (*model.AnyAttribute, error) {
	if schema == nil || len(groups) == 0 {
		return nil, nil
	}
	visited := make(map[*model.AttributeGroup]bool)
	var wildcard *model.AnyAttribute
	for _, ref := range groups {
		group, ok := schema.AttributeGroups[ref]
		if !ok {
			return nil, fmt.Errorf("attributeGroup %s not found", ref)
		}
		groupWildcard, err := attributeGroupWildcard(schema, group, visited)
		if err != nil {
			return nil, err
		}
		wildcard, err = intersectLocalAnyAttribute(groupWildcard, wildcard)
		if err != nil {
			return nil, err
		}
	}
	return wildcard, nil
}

func attributeGroupWildcard(schema *parser.Schema, group *model.AttributeGroup, visited map[*model.AttributeGroup]bool) (*model.AnyAttribute, error) {
	if schema == nil || group == nil {
		return nil, nil
	}
	if visited[group] {
		return nil, nil
	}
	visited[group] = true
	wildcard := group.AnyAttribute
	for _, ref := range group.AttrGroups {
		nested, ok := schema.AttributeGroups[ref]
		if !ok {
			return nil, fmt.Errorf("attributeGroup %s not found", ref)
		}
		nestedWildcard, err := attributeGroupWildcard(schema, nested, visited)
		if err != nil {
			return nil, err
		}
		wildcard, err = intersectLocalAnyAttribute(nestedWildcard, wildcard)
		if err != nil {
			return nil, err
		}
	}
	return wildcard, nil
}

func intersectLocalAnyAttribute(a, b *model.AnyAttribute) (*model.AnyAttribute, error) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	intersected, expressible, empty := model.IntersectAnyAttributeDetailed(a, b)
	if !expressible {
		return nil, fmt.Errorf("attribute wildcard intersection not expressible")
	}
	if empty {
		return nil, fmt.Errorf("attribute wildcard intersection empty")
	}
	return intersected, nil
}

func applyDerivedWildcard(base, local *model.AnyAttribute, ct *model.ComplexType) (*model.AnyAttribute, error) {
	method := model.DerivationRestriction
	if ct != nil && ct.DerivationMethod != 0 {
		method = ct.DerivationMethod
	}
	switch method {
	case model.DerivationExtension:
		return unionAnyAttribute(local, base)
	case model.DerivationRestriction:
		return restrictAnyAttribute(base, local)
	default:
		if local != nil {
			return local, nil
		}
		return base, nil
	}
}

func unionAnyAttribute(derived, base *model.AnyAttribute) (*model.AnyAttribute, error) {
	if derived == nil {
		return base, nil
	}
	if base == nil {
		return derived, nil
	}
	merged := model.UnionAnyAttribute(derived, base)
	if merged == nil {
		return nil, fmt.Errorf("attribute wildcard union not expressible")
	}
	return merged, nil
}

func restrictAnyAttribute(base, derived *model.AnyAttribute) (*model.AnyAttribute, error) {
	if derived == nil {
		return nil, nil
	}
	if base == nil {
		return nil, fmt.Errorf("attribute wildcard restriction adds wildcard")
	}
	intersected, expressible, empty := model.IntersectAnyAttributeDetailed(derived, base)
	if !expressible {
		return nil, fmt.Errorf("attribute wildcard restriction not expressible")
	}
	if empty {
		return nil, fmt.Errorf("attribute wildcard restriction empty")
	}
	if intersected != nil {
		intersected.ProcessContents = derived.ProcessContents
	}
	return intersected, nil
}
