package schemaast

import "testing"

func TestCheckBounds(t *testing.T) {
	tests := []struct {
		name      string
		minOccurs Occurs
		maxOccurs Occurs
		want      BoundsIssue
	}{
		{
			name:      "ok bounded",
			minOccurs: OccursFromInt(0),
			maxOccurs: OccursFromInt(2),
			want:      BoundsOK,
		},
		{
			name:      "overflow",
			minOccurs: OccursFromUint64(uint64(^uint32(0)) + 1),
			maxOccurs: OccursFromInt(2),
			want:      BoundsOverflow,
		},
		{
			name:      "max zero with min one",
			minOccurs: OccursFromInt(1),
			maxOccurs: OccursFromInt(0),
			want:      BoundsMaxZeroWithMinNonZero,
		},
		{
			name:      "min greater than max",
			minOccurs: OccursFromInt(2),
			maxOccurs: OccursFromInt(1),
			want:      BoundsMinGreaterThanMax,
		},
		{
			name:      "unbounded max",
			minOccurs: OccursFromInt(1),
			maxOccurs: OccursUnbounded,
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
		minOccurs Occurs
		maxOccurs Occurs
		want      AllGroupIssue
	}{
		{
			name:      "valid one one",
			minOccurs: OccursFromInt(1),
			maxOccurs: OccursFromInt(1),
			want:      AllGroupOK,
		},
		{
			name:      "valid zero one",
			minOccurs: OccursFromInt(0),
			maxOccurs: OccursFromInt(1),
			want:      AllGroupOK,
		},
		{
			name:      "invalid min",
			minOccurs: OccursFromInt(2),
			maxOccurs: OccursFromInt(1),
			want:      AllGroupMinNotZeroOrOne,
		},
		{
			name:      "invalid max",
			minOccurs: OccursFromInt(0),
			maxOccurs: OccursFromInt(2),
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
	if !IsAllGroupChildMaxValid(OccursFromInt(1)) {
		t.Fatalf("expected maxOccurs=1 to be valid")
	}
	if IsAllGroupChildMaxValid(OccursFromInt(2)) {
		t.Fatalf("expected maxOccurs=2 to be invalid")
	}
}
