package semanticcheck

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/attrwildcard"
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
)

// validateAnyAttributeDerivation validates anyAttribute constraints in type derivation
// According to XSD 1.0 spec:
// - For extension: anyAttribute must union with base type's anyAttribute (cos-aw-union)
// - For restriction: anyAttribute namespace constraint must be a subset of base type's anyAttribute (cos-aw-subset)
func validateAnyAttributeDerivation(schema *parser.Schema, ct *model.ComplexType) error {
	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}

	baseCT, ok := typechain.LookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}

	baseAnyAttr, err := collectAnyAttributeFromType(schema, baseCT)
	if err != nil {
		return err
	}
	derivedAnyAttr, err := collectAnyAttributeFromType(schema, ct)
	if err != nil {
		return err
	}

	if ct.IsExtension() {
		if baseAnyAttr != nil && derivedAnyAttr != nil {
			if _, err := attrwildcard.Union(derivedAnyAttr, baseAnyAttr); err != nil {
				return fmt.Errorf("anyAttribute extension: union of derived and base anyAttribute is not expressible")
			}
		}
	} else if ct.IsRestriction() {
		if baseAnyAttr == nil && derivedAnyAttr != nil {
			return fmt.Errorf("anyAttribute restriction: cannot add anyAttribute when base type has no anyAttribute")
		}
		if derivedAnyAttr != nil && baseAnyAttr != nil {
			if _, err := attrwildcard.Restrict(baseAnyAttr, derivedAnyAttr); err != nil {
				return fmt.Errorf("anyAttribute restriction: derived anyAttribute is not a valid subset of base anyAttribute")
			}
		}
	}

	return nil
}

// collectAnyAttributeFromType collects anyAttribute from a complex type
// Checks both direct anyAttribute and anyAttribute in extension/restriction
func collectAnyAttributeFromType(schema *parser.Schema, ct *model.ComplexType) (*model.AnyAttribute, error) {
	result, err := attrwildcard.CollectFromComplexType(schema, ct, attrwildcard.CollectOptions{
		Missing:      attrgroupwalk.MissingIgnore,
		Cycles:       attrgroupwalk.CycleIgnore,
		EmptyIsError: false,
	})
	if err != nil {
		switch {
		case errors.Is(err, attrwildcard.ErrIntersectionNotExpressible):
			return nil, fmt.Errorf("anyAttribute intersection is not expressible")
		case errors.Is(err, attrwildcard.ErrIntersectionEmpty):
			return nil, nil
		default:
			return nil, err
		}
	}
	return result, nil
}

// collectAnyAttributeFromGroups collects anyAttribute from attribute groups (recursively)
func collectAnyAttributeFromGroups(schema *parser.Schema, agRefs []model.QName) []*model.AnyAttribute {
	ctx := attrgroupwalk.NewContext(schema, attrgroupwalk.Options{
		Missing: attrgroupwalk.MissingIgnore,
		Cycles:  attrgroupwalk.CycleIgnore,
	})
	var result []*model.AnyAttribute
	_ = ctx.Walk(agRefs, func(_ model.QName, ag *model.AttributeGroup) error {
		if ag.AnyAttribute != nil {
			result = append(result, ag.AnyAttribute)
		}
		return nil
	})
	return result
}
