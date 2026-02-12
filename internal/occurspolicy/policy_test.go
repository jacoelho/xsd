package occurspolicy

import (
	"testing"

	"github.com/jacoelho/xsd/internal/occurs"
)

func TestCheckBounds(t *testing.T) {
	tests := []struct {
		name      string
		minOccurs occurs.Occurs
		maxOccurs occurs.Occurs
		want      BoundsIssue
	}{
		{
			name:      "ok bounded",
			minOccurs: occurs.OccursFromInt(0),
			maxOccurs: occurs.OccursFromInt(2),
			want:      BoundsOK,
		},
		{
			name:      "overflow",
			minOccurs: occurs.OccursFromUint64(uint64(^uint32(0)) + 1),
			maxOccurs: occurs.OccursFromInt(2),
			want:      BoundsOverflow,
		},
		{
			name:      "max zero with min one",
			minOccurs: occurs.OccursFromInt(1),
			maxOccurs: occurs.OccursFromInt(0),
			want:      BoundsMaxZeroWithMinNonZero,
		},
		{
			name:      "min greater than max",
			minOccurs: occurs.OccursFromInt(2),
			maxOccurs: occurs.OccursFromInt(1),
			want:      BoundsMinGreaterThanMax,
		},
		{
			name:      "unbounded max",
			minOccurs: occurs.OccursFromInt(1),
			maxOccurs: occurs.OccursUnbounded,
			want:      BoundsOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckBounds(tt.minOccurs, tt.maxOccurs)
			if got != tt.want {
				t.Fatalf("CheckBounds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckAllGroupBounds(t *testing.T) {
	tests := []struct {
		name      string
		minOccurs occurs.Occurs
		maxOccurs occurs.Occurs
		want      AllGroupIssue
	}{
		{
			name:      "valid one one",
			minOccurs: occurs.OccursFromInt(1),
			maxOccurs: occurs.OccursFromInt(1),
			want:      AllGroupOK,
		},
		{
			name:      "valid zero one",
			minOccurs: occurs.OccursFromInt(0),
			maxOccurs: occurs.OccursFromInt(1),
			want:      AllGroupOK,
		},
		{
			name:      "invalid min",
			minOccurs: occurs.OccursFromInt(2),
			maxOccurs: occurs.OccursFromInt(1),
			want:      AllGroupMinNotZeroOrOne,
		},
		{
			name:      "invalid max",
			minOccurs: occurs.OccursFromInt(0),
			maxOccurs: occurs.OccursFromInt(2),
			want:      AllGroupMaxNotOne,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckAllGroupBounds(tt.minOccurs, tt.maxOccurs)
			if got != tt.want {
				t.Fatalf("CheckAllGroupBounds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAllGroupChildMaxValid(t *testing.T) {
	if !IsAllGroupChildMaxValid(occurs.OccursFromInt(1)) {
		t.Fatalf("expected maxOccurs=1 to be valid")
	}
	if IsAllGroupChildMaxValid(occurs.OccursFromInt(2)) {
		t.Fatalf("expected maxOccurs=2 to be invalid")
	}
}
