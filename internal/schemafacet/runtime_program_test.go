package schemafacet

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestRuntimeProgramSlice(t *testing.T) {
	t.Parallel()

	meta := runtime.ValidatorMeta{
		Facets: runtime.FacetProgramRef{Off: 1, Len: 2},
	}
	facets := []runtime.FacetInstr{
		{Op: runtime.FPattern},
		{Op: runtime.FLength},
		{Op: runtime.FEnum, Arg0: 7},
	}
	program, err := RuntimeProgramSlice(meta, facets)
	if err != nil {
		t.Fatalf("RuntimeProgramSlice() error = %v", err)
	}
	if len(program) != 2 {
		t.Fatalf("program len = %d, want 2", len(program))
	}
	if program[0].Op != runtime.FLength || program[1].Op != runtime.FEnum {
		t.Fatalf("unexpected program ops = %v, %v", program[0].Op, program[1].Op)
	}
}

func TestRuntimeProgramSliceOutOfRange(t *testing.T) {
	t.Parallel()

	meta := runtime.ValidatorMeta{
		Facets: runtime.FacetProgramRef{Off: 2, Len: 2},
	}
	facets := []runtime.FacetInstr{{Op: runtime.FPattern}, {Op: runtime.FLength}}
	if _, err := RuntimeProgramSlice(meta, facets); err == nil {
		t.Fatal("expected out-of-range error")
	}
}

func TestRuntimeProgramHasOp(t *testing.T) {
	t.Parallel()

	meta := runtime.ValidatorMeta{
		Facets: runtime.FacetProgramRef{Off: 0, Len: 3},
	}
	facets := []runtime.FacetInstr{
		{Op: runtime.FPattern},
		{Op: runtime.FMaxLength},
		{Op: runtime.FEnum, Arg0: 3},
	}

	has, err := RuntimeProgramHasOp(meta, facets, runtime.FLength, runtime.FMinLength, runtime.FMaxLength)
	if err != nil {
		t.Fatalf("RuntimeProgramHasOp() error = %v", err)
	}
	if !has {
		t.Fatal("expected length op to be present")
	}

	has, err = RuntimeProgramHasOp(meta, facets, runtime.FTotalDigits)
	if err != nil {
		t.Fatalf("RuntimeProgramHasOp() error = %v", err)
	}
	if has {
		t.Fatal("expected totalDigits op to be absent")
	}
}

func TestRuntimeProgramEnumIDs(t *testing.T) {
	t.Parallel()

	program := []runtime.FacetInstr{
		{Op: runtime.FPattern},
		{Op: runtime.FEnum, Arg0: 2},
		{Op: runtime.FEnum, Arg0: 9},
	}
	ids := RuntimeProgramEnumIDs(program)
	if len(ids) != 2 {
		t.Fatalf("enum id len = %d, want 2", len(ids))
	}
	if ids[0] != 2 || ids[1] != 9 {
		t.Fatalf("enum ids = %v, want [2 9]", ids)
	}
}
