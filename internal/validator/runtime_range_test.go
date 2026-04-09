package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestRuntimeRangeSatisfied(t *testing.T) {
	tests := []struct {
		op   runtime.FacetOp
		cmp  int
		want bool
	}{
		{op: runtime.FMinInclusive, cmp: -1, want: false},
		{op: runtime.FMinInclusive, cmp: 0, want: true},
		{op: runtime.FMinInclusive, cmp: 1, want: true},
		{op: runtime.FMaxExclusive, cmp: -1, want: true},
		{op: runtime.FMaxExclusive, cmp: 0, want: false},
		{op: runtime.FMaxExclusive, cmp: 1, want: false},
	}
	for _, tt := range tests {
		got, ok := RuntimeRangeSatisfied(tt.op, tt.cmp)
		if !ok {
			t.Fatalf("missing rule for op %d", tt.op)
		}
		if got != tt.want {
			t.Fatalf("op=%d cmp=%d got=%v want=%v", tt.op, tt.cmp, got, tt.want)
		}
	}
}

func TestUnknownRuntimeRangeRules(t *testing.T) {
	if _, ok := RuntimeRangeSatisfied(runtime.FacetOp(255), 0); ok {
		t.Fatalf("expected no runtime range rule for unknown op")
	}
}
