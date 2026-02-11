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
