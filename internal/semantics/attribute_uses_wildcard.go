package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func localAttributeWildcard(schema *parser.Schema, ct *model.ComplexType) (*model.AnyAttribute, error) {
	wildcard, err := analysis.CollectComplexTypeWildcard(schema, ct, analysis.AttributeGroupCollectOptions{
		Missing:      analysis.MissingError,
		Cycles:       analysis.CycleIgnore,
		EmptyIsError: true,
	})
	if err != nil {
		switch {
		case errors.Is(err, analysis.ErrAttributeWildcardIntersectionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard intersection not expressible")
		case errors.Is(err, analysis.ErrAttributeWildcardIntersectionEmpty):
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
	out, err := analysis.ApplyAttributeWildcardDerivation(base, local, method)
	if err != nil {
		switch {
		case errors.Is(err, analysis.ErrAttributeWildcardUnionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard union not expressible")
		case errors.Is(err, analysis.ErrAttributeWildcardRestrictionAddsWildcard):
			return nil, fmt.Errorf("attribute wildcard restriction adds wildcard")
		case errors.Is(err, analysis.ErrAttributeWildcardRestrictionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard restriction not expressible")
		case errors.Is(err, analysis.ErrAttributeWildcardRestrictionEmpty):
			return nil, fmt.Errorf("attribute wildcard restriction empty")
		default:
			return nil, err
		}
	}
	return out, nil
}
