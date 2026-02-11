package validatorgen

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/attrwildcard"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func localAttributeWildcard(schema *parser.Schema, ct *model.ComplexType) (*model.AnyAttribute, error) {
	wildcard, err := attrwildcard.CollectFromComplexType(schema, ct, attrwildcard.CollectOptions{
		Missing:      attrgroupwalk.MissingError,
		Cycles:       attrgroupwalk.CycleIgnore,
		EmptyIsError: true,
	})
	if err != nil {
		switch {
		case errors.Is(err, attrwildcard.ErrIntersectionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard intersection not expressible")
		case errors.Is(err, attrwildcard.ErrIntersectionEmpty):
			return nil, nil
		default:
			return nil, err
		}
	}
	return wildcard, nil
}

func collectAttributeGroupWildcard(schema *parser.Schema, groups []model.QName) (*model.AnyAttribute, error) {
	groupWildcards, err := attrwildcard.CollectFromGroups(schema, groups, attrwildcard.CollectOptions{
		Missing: attrgroupwalk.MissingError,
		Cycles:  attrgroupwalk.CycleIgnore,
	})
	if err != nil {
		return nil, err
	}
	if len(groupWildcards) == 0 {
		return nil, nil
	}
	wildcard := groupWildcards[0]
	for i := 1; i < len(groupWildcards); i++ {
		var err error
		wildcard, err = intersectLocalAnyAttribute(groupWildcards[i], wildcard)
		if err != nil {
			return nil, err
		}
	}
	return wildcard, nil
}

func intersectLocalAnyAttribute(a, b *model.AnyAttribute) (*model.AnyAttribute, error) {
	intersected, err := attrwildcard.Intersect(a, b)
	if err != nil {
		switch {
		case errors.Is(err, attrwildcard.ErrIntersectionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard intersection not expressible")
		case errors.Is(err, attrwildcard.ErrIntersectionEmpty):
			return nil, nil
		default:
			return nil, err
		}
	}
	if intersected == nil && (a != nil && b != nil) {
		return nil, fmt.Errorf("attribute wildcard intersection not expressible")
	}
	return intersected, nil
}

func applyDerivedWildcard(base, local *model.AnyAttribute, ct *model.ComplexType) (*model.AnyAttribute, error) {
	method := model.DerivationRestriction
	if ct != nil {
		if ct.DerivationMethod != 0 {
			method = ct.DerivationMethod
		} else if content := ct.Content(); content != nil {
			switch {
			case content.ExtensionDef() != nil:
				method = model.DerivationExtension
			case content.RestrictionDef() != nil:
				method = model.DerivationRestriction
			}
		}
	}
	out, err := attrwildcard.ApplyDerivation(base, local, method)
	if err != nil {
		switch {
		case errors.Is(err, attrwildcard.ErrUnionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard union not expressible")
		case errors.Is(err, attrwildcard.ErrRestrictionAddsWildcard):
			return nil, fmt.Errorf("attribute wildcard restriction adds wildcard")
		case errors.Is(err, attrwildcard.ErrRestrictionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard restriction not expressible")
		case errors.Is(err, attrwildcard.ErrRestrictionEmpty):
			return nil, fmt.Errorf("attribute wildcard restriction empty")
		default:
			return nil, err
		}
	}
	return out, nil
}

func unionAnyAttribute(derived, base *model.AnyAttribute) (*model.AnyAttribute, error) {
	merged, err := attrwildcard.Union(derived, base)
	if err != nil {
		return nil, fmt.Errorf("attribute wildcard union not expressible")
	}
	return merged, nil
}

func restrictAnyAttribute(base, derived *model.AnyAttribute) (*model.AnyAttribute, error) {
	intersected, err := attrwildcard.Restrict(base, derived)
	if err != nil {
		switch {
		case errors.Is(err, attrwildcard.ErrRestrictionAddsWildcard):
			return nil, fmt.Errorf("attribute wildcard restriction adds wildcard")
		case errors.Is(err, attrwildcard.ErrRestrictionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard restriction not expressible")
		case errors.Is(err, attrwildcard.ErrRestrictionEmpty):
			return nil, fmt.Errorf("attribute wildcard restriction empty")
		default:
			return nil, err
		}
	}
	return intersected, nil
}
