package facetrules

import "github.com/jacoelho/xsd/internal/runtime"

type cmpRule uint8

const (
	cmpLE cmpRule = iota + 1
	cmpGE
	cmpGT
	cmpLT
)

// RestrictionRangeRule defines derived-vs-base restriction policy for range facets.
type RestrictionRangeRule struct {
	Comparator string
	rule       cmpRule
}

// RuntimeRangeRule defines runtime bound-check policy for range facet ops.
type RuntimeRangeRule struct {
	Violation string
	rule      cmpRule
}

var restrictionRangeRules = map[string]RestrictionRangeRule{
	"maxInclusive": {Comparator: "<=", rule: cmpLE},
	"maxExclusive": {Comparator: "<=", rule: cmpLE},
	"minInclusive": {Comparator: ">=", rule: cmpGE},
	"minExclusive": {Comparator: ">=", rule: cmpGE},
}

var runtimeRangeRules = map[runtime.FacetOp]RuntimeRangeRule{
	runtime.FMinInclusive: {Violation: "minInclusive violation", rule: cmpGE},
	runtime.FMaxInclusive: {Violation: "maxInclusive violation", rule: cmpLE},
	runtime.FMinExclusive: {Violation: "minExclusive violation", rule: cmpGT},
	runtime.FMaxExclusive: {Violation: "maxExclusive violation", rule: cmpLT},
}

// RestrictionRange returns range restriction policy for facetName.
func RestrictionRange(facetName string) (RestrictionRangeRule, bool) {
	rule, ok := restrictionRangeRules[facetName]
	return rule, ok
}

// RestrictionRangeSatisfied reports whether cmp satisfies derived-vs-base restriction policy.
func RestrictionRangeSatisfied(facetName string, cmp int) (bool, bool) {
	rule, ok := RestrictionRange(facetName)
	if !ok {
		return false, false
	}
	return compareMatches(rule.rule, cmp), true
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
	return compareMatches(rule.rule, cmp), true
}

func compareMatches(rule cmpRule, cmp int) bool {
	switch rule {
	case cmpLE:
		return cmp <= 0
	case cmpGE:
		return cmp >= 0
	case cmpGT:
		return cmp > 0
	case cmpLT:
		return cmp < 0
	default:
		return false
	}
}
