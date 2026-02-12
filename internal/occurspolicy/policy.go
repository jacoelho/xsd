package occurspolicy

import "github.com/jacoelho/xsd/internal/occurs"

// BoundsIssue enumerates bounds issue values.
type BoundsIssue uint8

const (
	BoundsOK BoundsIssue = iota
	BoundsOverflow
	BoundsMaxZeroWithMinNonZero
	BoundsMinGreaterThanMax
)

// AllGroupIssue enumerates all group issue values.
type AllGroupIssue uint8

const (
	AllGroupOK AllGroupIssue = iota
	AllGroupMinNotZeroOrOne
	AllGroupMaxNotOne
)

// CheckBounds validates general minOccurs/maxOccurs consistency.
func CheckBounds(minOccurs, maxOccurs occurs.Occurs) BoundsIssue {
	if maxOccurs.IsOverflow() || minOccurs.IsOverflow() {
		return BoundsOverflow
	}
	if maxOccurs.IsZero() && !minOccurs.IsZero() {
		return BoundsMaxZeroWithMinNonZero
	}
	if !maxOccurs.IsUnbounded() && !maxOccurs.IsZero() && maxOccurs.Cmp(minOccurs) < 0 {
		return BoundsMinGreaterThanMax
	}
	return BoundsOK
}

// CheckAllGroupBounds validates minOccurs/maxOccurs constraints for xs:all particles.
func CheckAllGroupBounds(minOccurs, maxOccurs occurs.Occurs) AllGroupIssue {
	if !minOccurs.IsZero() && !minOccurs.IsOne() {
		return AllGroupMinNotZeroOrOne
	}
	if !maxOccurs.IsOne() {
		return AllGroupMaxNotOne
	}
	return AllGroupOK
}

// IsAllGroupChildMaxValid reports whether an xs:all child maxOccurs is within the XSD 1.0 limit.
func IsAllGroupChildMaxValid(maxOccurs occurs.Occurs) bool {
	return maxOccurs.CmpInt(1) <= 0
}
