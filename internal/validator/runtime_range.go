package validator

import "github.com/jacoelho/xsd/internal/runtime"

type runtimeCmpRule uint8

const (
	runtimeCmpLE runtimeCmpRule = iota + 1
	runtimeCmpGE
	runtimeCmpGT
	runtimeCmpLT
)

// RuntimeRangeRule defines runtime bound-check policy for range facet ops.
type RuntimeRangeRule struct {
	Violation string
	rule      runtimeCmpRule
}

var runtimeRangeRules = map[runtime.FacetOp]RuntimeRangeRule{
	runtime.FMinInclusive: {Violation: "minInclusive violation", rule: runtimeCmpGE},
	runtime.FMaxInclusive: {Violation: "maxInclusive violation", rule: runtimeCmpLE},
	runtime.FMinExclusive: {Violation: "minExclusive violation", rule: runtimeCmpGT},
	runtime.FMaxExclusive: {Violation: "maxExclusive violation", rule: runtimeCmpLT},
}

// RuntimeRange returns runtime range-check policy for op.
func RuntimeRange(op runtime.FacetOp) (RuntimeRangeRule, bool) {
	rule, ok := runtimeRangeRules[op]
	return rule, ok
}

// RuntimeRangeSatisfied reports whether cmp satisfies runtime range-check policy.
func RuntimeRangeSatisfied(op runtime.FacetOp, cmp int) (bool, bool) {
	rule, ok := RuntimeRange(op)
	if !ok {
		return false, false
	}
	return runtimeCompareMatches(rule.rule, cmp), true
}

func runtimeCompareMatches(rule runtimeCmpRule, cmp int) bool {
	switch rule {
	case runtimeCmpLE:
		return cmp <= 0
	case runtimeCmpGE:
		return cmp >= 0
	case runtimeCmpGT:
		return cmp > 0
	case runtimeCmpLT:
		return cmp < 0
	}
	return false
}
