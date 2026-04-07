package analysis

import (
	"errors"
	"testing"
)

func TestDetectGraphCycleCycle(t *testing.T) {
	graph := map[int][]int{
		1: {2},
		2: {3},
		3: {1},
	}
	err := DetectGraphCycle(GraphCycleConfig[int]{
		Starts:  []int{1},
		Missing: GraphCycleMissingPolicyError,
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
	var cycleError GraphCycleError[int]
	if !errors.As(err, &cycleError) {
		t.Fatalf("DetectGraphCycle() error = %T, want GraphCycleError[int]", err)
	}
}

func TestDetectGraphCycleMissingPolicy(t *testing.T) {
	graph := map[int][]int{
		1: {2},
	}
	err := DetectGraphCycle(GraphCycleConfig[int]{
		Starts:  []int{1},
		Missing: GraphCycleMissingPolicyError,
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
	var missing GraphMissingError[int]
	ok := errors.As(err, &missing)
	if !ok {
		t.Fatalf("DetectGraphCycle() error = %T, want GraphMissingError[int]", err)
	}
	if missing.From != 1 || missing.Key != 2 {
		t.Fatalf("missing = %+v, want from=1 key=2", missing)
	}

	err = DetectGraphCycle(GraphCycleConfig[int]{
		Starts:  []int{1},
		Missing: GraphCycleMissingPolicyIgnore,
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
	err := DetectGraphCycle(GraphCycleConfig[int]{Starts: []int{1}})
	if err == nil {
		t.Fatal("DetectGraphCycle() error = nil, want error")
	}
}
