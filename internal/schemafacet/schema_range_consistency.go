package schemafacet

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
)

func isDurationType(baseType model.Type, baseQName model.QName) bool {
	if baseQName.Namespace == model.XSDNamespace && baseQName.Local == "duration" {
		return true
	}
	if baseType == nil {
		return false
	}
	primitive := baseType.PrimitiveType()
	if primitive == nil {
		return false
	}
	return primitive.Name().Namespace == model.XSDNamespace && primitive.Name().Local == "duration"
}

// ValidateRangeConsistency validates min/max range facet combinations.
func ValidateRangeConsistency(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType model.Type, baseQName model.QName) error {
	if isDurationType(baseType, baseQName) {
		return ValidateDurationRangeConsistency(minExclusive, maxExclusive, minInclusive, maxInclusive)
	}
	if maxInclusive != nil && maxExclusive != nil {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}
	if minInclusive != nil && minExclusive != nil {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}

	baseTypeForCompare := baseType
	if baseTypeForCompare == nil {
		if bt := builtins.GetNS(baseQName.Namespace, baseQName.Local); bt != nil {
			baseTypeForCompare = bt
		}
	}

	compare := func(v1, v2 string) (int, bool, error) {
		if baseTypeForCompare == nil {
			return 0, false, nil
		}
		if facets := baseTypeForCompare.FundamentalFacets(); facets != nil && facets.Ordered == model.OrderedNone {
			return 0, false, nil
		}
		cmp, err := CompareFacetValues(v1, v2, baseTypeForCompare)
		if errors.Is(err, ErrDateTimeNotComparable) || errors.Is(err, ErrDurationNotComparable) || errors.Is(err, ErrFloatNotComparable) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return cmp, true, nil
	}

	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}
	if minExclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minExclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
		}
	}
	if minInclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minInclusive/maxInclusive: %w", err)
		} else if ok && cmp > 0 {
			return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
		}
	}
	if minInclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minInclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minInclusive (%s) must be < maxExclusive (%s)", *minInclusive, *maxExclusive)
		}
	}

	return nil
}

// ValidateDurationRangeConsistency validates duration min/max facet combinations.
func ValidateDurationRangeConsistency(minExclusive, maxExclusive, minInclusive, maxInclusive *string) error {
	compare := func(v1, v2 string) (int, bool, error) {
		cmp, err := compareDurationValues(v1, v2)
		if errors.Is(err, ErrDurationNotComparable) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return cmp, true, nil
	}

	if maxInclusive != nil && maxExclusive != nil {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}
	if minInclusive != nil && minExclusive != nil {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}
	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}
	if minExclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minExclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
		}
	}
	if minInclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minInclusive/maxInclusive: %w", err)
		} else if ok && cmp > 0 {
			return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
		}
	}
	if minInclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minInclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minInclusive (%s) must be < maxExclusive (%s)", *minInclusive, *maxExclusive)
		}
	}
	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}

	return nil
}
