package lower

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func localAttributeWildcard(schema *parser.Schema, ct *model.ComplexType) (*model.AnyAttribute, error) {
	wildcard, err := attrgroupwalk.CollectFromComplexType(schema, ct, attrgroupwalk.CollectOptions{
		Missing:      attrgroupwalk.MissingError,
		Cycles:       attrgroupwalk.CycleIgnore,
		EmptyIsError: true,
	})
	if err != nil {
		switch {
		case errors.Is(err, attrgroupwalk.ErrIntersectionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard intersection not expressible")
		case errors.Is(err, attrgroupwalk.ErrIntersectionEmpty):
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
	out, err := attrgroupwalk.ApplyDerivation(base, local, method)
	if err != nil {
		switch {
		case errors.Is(err, attrgroupwalk.ErrUnionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard union not expressible")
		case errors.Is(err, attrgroupwalk.ErrRestrictionAddsWildcard):
			return nil, fmt.Errorf("attribute wildcard restriction adds wildcard")
		case errors.Is(err, attrgroupwalk.ErrRestrictionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard restriction not expressible")
		case errors.Is(err, attrgroupwalk.ErrRestrictionEmpty):
			return nil, fmt.Errorf("attribute wildcard restriction empty")
		default:
			return nil, err
		}
	}
	return out, nil
}
