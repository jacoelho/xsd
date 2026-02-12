package facets

import (
	"strings"
	"testing"
)

func TestCheckRangeConstraintsChecksEachPairOnce(t *testing.T) {
	t.Parallel()

	minExclusive := "1"
	maxExclusive := "5"
	minInclusive := "2"
	maxInclusive := "6"

	calls := 0
	err := checkRangeConstraints(&minExclusive, &maxExclusive, &minInclusive, &maxInclusive, func(_, _ string) (int, bool, error) {
		calls++
		return -1, true, nil
	})
	if err != nil {
		t.Fatalf("checkRangeConstraints() error = %v", err)
	}
	if calls != 4 {
		t.Fatalf("checkRangeConstraints() calls = %d, want 4", calls)
	}
}

func TestValidateDurationRangeConsistencyMinExclusiveVsMaxInclusive(t *testing.T) {
	t.Parallel()

	minExclusive := "P2D"
	maxInclusive := "P1D"

	err := ValidateDurationRangeConsistency(&minExclusive, nil, nil, &maxInclusive)
	if err == nil {
		t.Fatal("ValidateDurationRangeConsistency() error = nil, want non-nil")
	}
	want := "minExclusive (P2D) must be < maxInclusive (P1D)"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("ValidateDurationRangeConsistency() error = %q, want substring %q", err.Error(), want)
	}
}

func TestValidateDurationRangeConsistencyMutualExclusion(t *testing.T) {
	t.Parallel()

	maxInclusive := "P2D"
	maxExclusive := "P3D"

	err := ValidateDurationRangeConsistency(nil, &maxExclusive, nil, &maxInclusive)
	if err == nil {
		t.Fatal("ValidateDurationRangeConsistency() error = nil, want non-nil")
	}
	want := "maxInclusive and maxExclusive cannot both be specified"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("ValidateDurationRangeConsistency() error = %q, want substring %q", err.Error(), want)
	}
}
