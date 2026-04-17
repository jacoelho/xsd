package semantics_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/semantics"
)

func TestDetectGraphCycleCycle(t *testing.T) {
	graph := map[int][]int{
		1: {2},
		2: {3},
		3: {1},
	}
	err := semantics.DetectGraphCycle(semantics.GraphCycleConfig[int]{
		Starts:  []int{1},
		Missing: semantics.GraphCycleMissingPolicyError,
		Exists: func(n int) bool {
			_, ok := graph[n]
			return ok
		},
		Next: func(n int) ([]int, error) {
			return graph[n], nil
		},
	})
	if err == nil {
		t.Fatalf("DetectGraphCycle() expected cycle error")
	}
	var cycleErr semantics.GraphCycleError[int]
	if !errors.As(err, &cycleErr) {
		t.Fatalf("DetectGraphCycle() error = %T, want GraphCycleError[int]", err)
	}
}

func TestDetectGraphCycleMissingPolicy(t *testing.T) {
	graph := map[int][]int{
		1: {2},
	}
	err := semantics.DetectGraphCycle(semantics.GraphCycleConfig[int]{
		Starts:  []int{1},
		Missing: semantics.GraphCycleMissingPolicyError,
		Exists: func(n int) bool {
			_, ok := graph[n]
			return ok
		},
		Next: func(n int) ([]int, error) {
			return graph[n], nil
		},
	})
	if err == nil {
		t.Fatalf("DetectGraphCycle() expected missing error")
	}
	var missing semantics.GraphMissingError[int]
	if !errors.As(err, &missing) {
		t.Fatalf("DetectGraphCycle() error = %T, want GraphMissingError[int]", err)
	}
	if missing.From != 1 || missing.Key != 2 {
		t.Fatalf("missing = %+v, want from=1 key=2", missing)
	}

	err = semantics.DetectGraphCycle(semantics.GraphCycleConfig[int]{
		Starts:  []int{1},
		Missing: semantics.GraphCycleMissingPolicyIgnore,
		Exists: func(n int) bool {
			_, ok := graph[n]
			return ok
		},
		Next: func(n int) ([]int, error) {
			return graph[n], nil
		},
	})
	if err != nil {
		t.Fatalf("DetectGraphCycle() with MissingIgnore error = %v", err)
	}
}

func TestDetectGraphCycleNilNext(t *testing.T) {
	err := semantics.DetectGraphCycle(semantics.GraphCycleConfig[int]{Starts: []int{1}})
	if err == nil {
		t.Fatal("DetectGraphCycle() error = nil, want error")
	}
}
