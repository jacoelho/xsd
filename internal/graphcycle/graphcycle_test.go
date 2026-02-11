package graphcycle

import "testing"

func TestDetectCycle(t *testing.T) {
	graph := map[int][]int{
		1: {2},
		2: {3},
		3: {1},
	}
	err := Detect(Config[int]{
		Starts:  []int{1},
		Missing: MissingPolicyError,
		Exists: func(n int) bool {
			_, ok := graph[n]
			return ok
		},
		Next: func(n int) ([]int, error) {
			return graph[n], nil
		},
	})
	if err == nil {
		t.Fatalf("Detect() expected cycle error")
	}
	if _, ok := err.(CycleError[int]); !ok {
		t.Fatalf("Detect() error = %T, want CycleError[int]", err)
	}
}

func TestDetectMissingPolicy(t *testing.T) {
	graph := map[int][]int{
		1: {2},
	}
	err := Detect(Config[int]{
		Starts:  []int{1},
		Missing: MissingPolicyError,
		Exists: func(n int) bool {
			_, ok := graph[n]
			return ok
		},
		Next: func(n int) ([]int, error) {
			return graph[n], nil
		},
	})
	if err == nil {
		t.Fatalf("Detect() expected missing error")
	}
	missing, ok := err.(MissingError[int])
	if !ok {
		t.Fatalf("Detect() error = %T, want MissingError[int]", err)
	}
	if missing.From != 1 || missing.Key != 2 {
		t.Fatalf("missing = %+v, want from=1 key=2", missing)
	}

	err = Detect(Config[int]{
		Starts:  []int{1},
		Missing: MissingPolicyIgnore,
		Exists: func(n int) bool {
			_, ok := graph[n]
			return ok
		},
		Next: func(n int) ([]int, error) {
			return graph[n], nil
		},
	})
	if err != nil {
		t.Fatalf("Detect() with MissingIgnore error = %v", err)
	}
}
