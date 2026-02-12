package occurspolicy

import "github.com/jacoelho/xsd/internal/occurs"

type BoundsIssue uint8

const (
	BoundsOK BoundsIssue = iota
	BoundsOverflow
	BoundsMaxZeroWithMinNonZero
	BoundsMinGreaterThanMax
)

type AllGroupIssue uint8

const (
	AllGroupOK AllGroupIssue = iota
	AllGroupMinNotZeroOrOne
	AllGroupMaxNotOne
)

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

func CheckAllGroupBounds(minOccurs, maxOccurs occurs.Occurs) AllGroupIssue {
	if !minOccurs.IsZero() && !minOccurs.IsOne() {
		return AllGroupMinNotZeroOrOne
	}
	if !maxOccurs.IsOne() {
		return AllGroupMaxNotOne
	}
	return AllGroupOK
}

func IsAllGroupChildMaxValid(maxOccurs occurs.Occurs) bool {
	return maxOccurs.CmpInt(1) <= 0
}
