package semantics

type cmpRule uint8

const (
	cmpLE cmpRule = iota + 1
	cmpGE
	cmpGT
	cmpLT
)

// RestrictionRangeRule defines derived-vs-base restriction policy for range
type RestrictionRangeRule struct {
	Comparator string
	rule       cmpRule
}

var restrictionRangeRules = map[string]RestrictionRangeRule{
	"maxInclusive": {Comparator: "<=", rule: cmpLE},
	"maxExclusive": {Comparator: "<=", rule: cmpLE},
	"minInclusive": {Comparator: ">=", rule: cmpGE},
	"minExclusive": {Comparator: ">=", rule: cmpGE},
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
	}
	return false
}
