package attrwildcard

import (
	"errors"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

var (
	ErrIntersectionNotExpressible = errors.New("attribute wildcard intersection not expressible")
	ErrIntersectionEmpty          = errors.New("attribute wildcard intersection empty")
	ErrUnionNotExpressible        = errors.New("attribute wildcard union not expressible")
	ErrRestrictionAddsWildcard    = errors.New("attribute wildcard restriction adds wildcard")
	ErrRestrictionNotExpressible  = errors.New("attribute wildcard restriction not expressible")
	ErrRestrictionEmpty           = errors.New("attribute wildcard restriction empty")
)

type CollectOptions struct {
	Missing      attrgroupwalk.MissingPolicy
	Cycles       attrgroupwalk.CyclePolicy
	EmptyIsError bool
}

// CollectFromGroups gathers anyAttribute wildcards from attribute groups in walk order.
func CollectFromGroups(schema *parser.Schema, refs []model.QName, opts CollectOptions) ([]*model.AnyAttribute, error) {
	if schema == nil || len(refs) == 0 {
		return nil, nil
	}
	ctx := attrgroupwalk.NewContext(schema, attrgroupwalk.Options{
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

// CollectFromComplexType collects effective local anyAttribute by intersecting direct and group wildcards.
func CollectFromComplexType(schema *parser.Schema, ct *model.ComplexType, opts CollectOptions) (*model.AnyAttribute, error) {
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

	groupWildcards, err := CollectFromGroups(schema, refs, opts)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, groupWildcards...)
	if len(candidates) == 0 {
		return nil, nil
	}

	current := candidates[0]
	for i := 1; i < len(candidates); i++ {
		current, err = Intersect(current, candidates[i])
		if err != nil {
			if errors.Is(err, ErrIntersectionEmpty) && !opts.EmptyIsError {
				return nil, nil
			}
			return nil, err
		}
	}
	return current, nil
}

// Intersect intersects two anyAttribute wildcards.
func Intersect(a, b *model.AnyAttribute) (*model.AnyAttribute, error) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	intersected, expressible, empty := model.IntersectAnyAttributeDetailed(a, b)
	if !expressible {
		return nil, ErrIntersectionNotExpressible
	}
	if empty {
		return nil, ErrIntersectionEmpty
	}
	return intersected, nil
}

// Union unions two anyAttribute wildcards.
func Union(derived, base *model.AnyAttribute) (*model.AnyAttribute, error) {
	if derived == nil {
		return base, nil
	}
	if base == nil {
		return derived, nil
	}
	merged := model.UnionAnyAttribute(derived, base)
	if merged == nil {
		return nil, ErrUnionNotExpressible
	}
	return merged, nil
}

// Restrict applies restriction semantics where derived must be subset of base.
func Restrict(base, derived *model.AnyAttribute) (*model.AnyAttribute, error) {
	if derived == nil {
		return nil, nil
	}
	if base == nil {
		return nil, ErrRestrictionAddsWildcard
	}
	if !model.ProcessContentsStrongerOrEqual(derived.ProcessContents, base.ProcessContents) {
		return nil, ErrRestrictionNotExpressible
	}
	if !model.NamespaceConstraintSubset(
		derived.Namespace, derived.NamespaceList, derived.TargetNamespace,
		base.Namespace, base.NamespaceList, base.TargetNamespace,
	) {
		return nil, ErrRestrictionNotExpressible
	}
	intersected, expressible, empty := model.IntersectAnyAttributeDetailed(derived, base)
	if !expressible {
		return nil, ErrRestrictionNotExpressible
	}
	if empty {
		return nil, ErrRestrictionEmpty
	}
	if intersected != nil {
		intersected.ProcessContents = derived.ProcessContents
	}
	return intersected, nil
}

// ApplyDerivation applies extension/restriction wildcard derivation policy.
func ApplyDerivation(base, local *model.AnyAttribute, method model.DerivationMethod) (*model.AnyAttribute, error) {
	switch method {
	case model.DerivationExtension:
		return Union(local, base)
	case model.DerivationRestriction:
		return Restrict(base, local)
	default:
		if local != nil {
			return local, nil
		}
		return base, nil
	}
}
