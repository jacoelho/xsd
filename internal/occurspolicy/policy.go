package occurspolicy

import "github.com/jacoelho/xsd/internal/occurs"

// BoundsIssue defines an exported type.
type BoundsIssue uint8

const (
	// BoundsOK is an exported constant.
	BoundsOK BoundsIssue = iota
	// BoundsOverflow is an exported constant.
	BoundsOverflow
	// BoundsMaxZeroWithMinNonZero is an exported constant.
	BoundsMaxZeroWithMinNonZero
	// BoundsMinGreaterThanMax is an exported constant.
	BoundsMinGreaterThanMax
)

// AllGroupIssue defines an exported type.
type AllGroupIssue uint8

const (
	// AllGroupOK is an exported constant.
	AllGroupOK AllGroupIssue = iota
	// AllGroupMinNotZeroOrOne is an exported constant.
	AllGroupMinNotZeroOrOne
	// AllGroupMaxNotOne is an exported constant.
	AllGroupMaxNotOne
)

// CheckBounds is an exported function.
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

// CheckAllGroupBounds is an exported function.
func CheckAllGroupBounds(minOccurs, maxOccurs occurs.Occurs) AllGroupIssue {
	if !minOccurs.IsZero() && !minOccurs.IsOne() {
		return AllGroupMinNotZeroOrOne
	}
	if !maxOccurs.IsOne() {
		return AllGroupMaxNotOne
	}
	return AllGroupOK
}

// IsAllGroupChildMaxValid is an exported function.
func IsAllGroupChildMaxValid(maxOccurs occurs.Occurs) bool {
	return maxOccurs.CmpInt(1) <= 0
}
