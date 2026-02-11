package occurspolicy

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestCheckBounds(t *testing.T) {
	tests := []struct {
		name      string
		minOccurs model.Occurs
		maxOccurs model.Occurs
		want      BoundsIssue
	}{
		{
			name:      "ok bounded",
			minOccurs: model.OccursFromInt(0),
			maxOccurs: model.OccursFromInt(2),
			want:      BoundsOK,
		},
		{
			name:      "overflow",
			minOccurs: model.OccursFromUint64(uint64(^uint32(0)) + 1),
			maxOccurs: model.OccursFromInt(2),
			want:      BoundsOverflow,
		},
		{
			name:      "max zero with min one",
			minOccurs: model.OccursFromInt(1),
			maxOccurs: model.OccursFromInt(0),
			want:      BoundsMaxZeroWithMinNonZero,
		},
		{
			name:      "min greater than max",
			minOccurs: model.OccursFromInt(2),
			maxOccurs: model.OccursFromInt(1),
			want:      BoundsMinGreaterThanMax,
		},
		{
			name:      "unbounded max",
			minOccurs: model.OccursFromInt(1),
			maxOccurs: model.OccursUnbounded,
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
		minOccurs model.Occurs
		maxOccurs model.Occurs
		want      AllGroupIssue
	}{
		{
			name:      "valid one one",
			minOccurs: model.OccursFromInt(1),
			maxOccurs: model.OccursFromInt(1),
			want:      AllGroupOK,
		},
		{
			name:      "valid zero one",
			minOccurs: model.OccursFromInt(0),
			maxOccurs: model.OccursFromInt(1),
			want:      AllGroupOK,
		},
		{
			name:      "invalid min",
			minOccurs: model.OccursFromInt(2),
			maxOccurs: model.OccursFromInt(1),
			want:      AllGroupMinNotZeroOrOne,
		},
		{
			name:      "invalid max",
			minOccurs: model.OccursFromInt(0),
			maxOccurs: model.OccursFromInt(2),
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
	if !IsAllGroupChildMaxValid(model.OccursFromInt(1)) {
		t.Fatalf("expected maxOccurs=1 to be valid")
	}
	if IsAllGroupChildMaxValid(model.OccursFromInt(2)) {
		t.Fatalf("expected maxOccurs=2 to be invalid")
	}
}
