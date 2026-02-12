package validator

import (
	"github.com/jacoelho/xsd/internal/facetrules"
	"github.com/jacoelho/xsd/internal/runtime"
)

func compareRange(op runtime.FacetOp, cmp int) error {
	matches, ok := facetrules.RuntimeRangeSatisfied(op, cmp)
	if !ok || !matches {
		return rangeViolation(op)
	}
	return nil
}

func rangeViolation(op runtime.FacetOp) error {
	if rule, ok := facetrules.RuntimeRange(op); ok {
		return valueErrorMsg(valueErrFacet, rule.Violation)
	}
	return valueErrorMsg(valueErrFacet, "range violation")
}
