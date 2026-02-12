package facets

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
	return checkRangeConstraints(minExclusive, maxExclusive, minInclusive, maxInclusive, compare)
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
	return checkRangeConstraints(minExclusive, maxExclusive, minInclusive, maxInclusive, compare)
}

func checkRangeConstraints(
	minExclusive, maxExclusive, minInclusive, maxInclusive *string,
	compare func(v1, v2 string) (int, bool, error),
) error {
	checks := [...]struct {
		left      *string
		right     *string
		failWhen  func(int) bool
		leftName  string
		rightName string
		op        string
	}{
		{
			left:      minExclusive,
			right:     maxInclusive,
			leftName:  "minExclusive",
			rightName: "maxInclusive",
			op:        "<",
			failWhen:  func(cmp int) bool { return cmp >= 0 },
		},
		{
			left:      minExclusive,
			right:     maxExclusive,
			leftName:  "minExclusive",
			rightName: "maxExclusive",
			op:        "<",
			failWhen:  func(cmp int) bool { return cmp >= 0 },
		},
		{
			left:      minInclusive,
			right:     maxInclusive,
			leftName:  "minInclusive",
			rightName: "maxInclusive",
			op:        "<=",
			failWhen:  func(cmp int) bool { return cmp > 0 },
		},
		{
			left:      minInclusive,
			right:     maxExclusive,
			leftName:  "minInclusive",
			rightName: "maxExclusive",
			op:        "<",
			failWhen:  func(cmp int) bool { return cmp >= 0 },
		},
	}

	for _, check := range checks {
		if check.left == nil || check.right == nil {
			continue
		}
		cmp, ok, err := compare(*check.left, *check.right)
		if err != nil {
			return fmt.Errorf("%s/%s: %w", check.leftName, check.rightName, err)
		}
		if ok && check.failWhen(cmp) {
			return fmt.Errorf("%s (%s) must be %s %s (%s)", check.leftName, *check.left, check.op, check.rightName, *check.right)
		}
	}
	return nil
}
