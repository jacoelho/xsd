package semantics

import (
	"errors"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

var (
	ErrAttributeWildcardIntersectionNotExpressible = errors.New("attribute wildcard intersection not expressible")
	ErrAttributeWildcardIntersectionEmpty          = errors.New("attribute wildcard intersection empty")
	ErrAttributeWildcardUnionNotExpressible        = errors.New("attribute wildcard union not expressible")
	ErrAttributeWildcardRestrictionAddsWildcard    = errors.New("attribute wildcard restriction adds wildcard")
	ErrAttributeWildcardRestrictionNotExpressible  = errors.New("attribute wildcard restriction not expressible")
	ErrAttributeWildcardRestrictionEmpty           = errors.New("attribute wildcard restriction empty")
)

// AttributeGroupCollectOptions controls traversal and empty-intersection handling.
type AttributeGroupCollectOptions struct {
	Missing      MissingPolicy
	Cycles       CyclePolicy
	EmptyIsError bool
}

// CollectAttributeGroupWildcards gathers anyAttribute wildcards from attribute groups in walk order.
func CollectAttributeGroupWildcards(
	schema *parser.Schema,
	refs []model.QName,
	opts AttributeGroupCollectOptions,
) ([]*model.AnyAttribute, error) {
	if schema == nil || len(refs) == 0 {
		return nil, nil
	}
	ctx := NewAttributeGroupContext(schema, AttributeGroupWalkOptions{
		Missing: opts.Missing,
		Cycles:  opts.Cycles,
	})
	out := make([]*model.AnyAttribute, 0)
	if err := ctx.Walk(refs, func(_ model.QName, group *model.AttributeGroup) error {
		if group != nil && group.AnyAttribute != nil {
			out = append(out, group.AnyAttribute)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// CollectComplexTypeWildcard collects effective local anyAttribute by intersecting direct and group wildcards.
func CollectComplexTypeWildcard(
	schema *parser.Schema,
	ct *model.ComplexType,
	opts AttributeGroupCollectOptions,
) (*model.AnyAttribute, error) {
	if schema == nil || ct == nil {
		return nil, nil
	}
	var refs []model.QName
	candidates := make([]*model.AnyAttribute, 0, 4)

	refs = append(refs, ct.AttrGroups...)
	if anyAttr := ct.AnyAttribute(); anyAttr != nil {
		candidates = append(candidates, anyAttr)
	}
	if content := ct.Content(); content != nil {
		if ext := content.ExtensionDef(); ext != nil {
			refs = append(refs, ext.AttrGroups...)
			if ext.AnyAttribute != nil {
				candidates = append(candidates, ext.AnyAttribute)
			}
		}
		if restr := content.RestrictionDef(); restr != nil {
			refs = append(refs, restr.AttrGroups...)
			if restr.AnyAttribute != nil {
				candidates = append(candidates, restr.AnyAttribute)
			}
		}
	}

	groupWildcards, err := CollectAttributeGroupWildcards(schema, refs, opts)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, groupWildcards...)
	if len(candidates) == 0 {
		return nil, nil
	}

	current := candidates[0]
	for i := 1; i < len(candidates); i++ {
		current, err = IntersectAttributeWildcards(current, candidates[i])
		if err != nil {
			if errors.Is(err, ErrAttributeWildcardIntersectionEmpty) && !opts.EmptyIsError {
				return nil, nil
			}
			return nil, err
		}
	}
	return current, nil
}

func IntersectAttributeWildcards(a, b *model.AnyAttribute) (*model.AnyAttribute, error) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	intersected, expressible, empty := model.IntersectAnyAttributeDetailed(a, b)
	if !expressible {
		return nil, ErrAttributeWildcardIntersectionNotExpressible
	}
	if empty {
		return nil, ErrAttributeWildcardIntersectionEmpty
	}
	return intersected, nil
}

func UnionAttributeWildcards(derived, base *model.AnyAttribute) (*model.AnyAttribute, error) {
	if derived == nil {
		return base, nil
	}
	if base == nil {
		return derived, nil
	}
	merged := model.UnionAnyAttribute(derived, base)
	if merged == nil {
		return nil, ErrAttributeWildcardUnionNotExpressible
	}
	return merged, nil
}

func RestrictAttributeWildcard(base, derived *model.AnyAttribute) (*model.AnyAttribute, error) {
	if derived == nil {
		return nil, nil
	}
	if base == nil {
		return nil, ErrAttributeWildcardRestrictionAddsWildcard
	}
	if !model.ProcessContentsStrongerOrEqual(derived.ProcessContents, base.ProcessContents) {
		return nil, ErrAttributeWildcardRestrictionNotExpressible
	}
	if !model.NamespaceConstraintSubset(
		derived.Namespace, derived.NamespaceList, derived.TargetNamespace,
		base.Namespace, base.NamespaceList, base.TargetNamespace,
	) {
		return nil, ErrAttributeWildcardRestrictionNotExpressible
	}
	intersected, expressible, empty := model.IntersectAnyAttributeDetailed(derived, base)
	if !expressible {
		return nil, ErrAttributeWildcardRestrictionNotExpressible
	}
	if empty {
		return nil, ErrAttributeWildcardRestrictionEmpty
	}
	if intersected != nil {
		intersected.ProcessContents = derived.ProcessContents
	}
	return intersected, nil
}

func ApplyAttributeWildcardDerivation(
	base, local *model.AnyAttribute,
	method model.DerivationMethod,
) (*model.AnyAttribute, error) {
	switch method {
	case model.DerivationExtension:
		return UnionAttributeWildcards(local, base)
	case model.DerivationRestriction:
		return RestrictAttributeWildcard(base, local)
	default:
		if local != nil {
			return local, nil
		}
		return base, nil
	}
}
