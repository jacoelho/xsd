package analysis

import "fmt"

type graphVisitState uint8

const (
	graphStateVisiting graphVisitState = iota + 1
	graphStateDone
)

// GraphCycleMissingPolicy controls behavior when a referenced node is missing.
type GraphCycleMissingPolicy uint8

const (
	GraphCycleMissingPolicyIgnore GraphCycleMissingPolicy = iota
	GraphCycleMissingPolicyError
)

// GraphCycleError reports a cycle at Key.
type GraphCycleError[K comparable] struct {
	Key K
}

// Error returns the error string.
func (e GraphCycleError[K]) Error() string {
	return "cycle detected"
}

// GraphMissingError reports a missing referenced node.
type GraphMissingError[K comparable] struct {
	From K
	Key  K
}

// Error returns the error string.
func (e GraphMissingError[K]) Error() string {
	return "missing node"
}

// GraphCycleConfig configures generic cycle detection traversal.
type GraphCycleConfig[K comparable] struct {
	Exists  func(K) bool
	Next    func(K) ([]K, error)
	Starts  []K
	Missing GraphCycleMissingPolicy
}

// DetectGraphCycle walks directed edges from Starts and reports first cycle or traversal error.
func DetectGraphCycle[K comparable](cfg GraphCycleConfig[K]) error {
	if cfg.Next == nil {
		return fmt.Errorf("cycle detect: next function is nil")
	}
	states := make(map[K]graphVisitState, len(cfg.Starts))

	var zero K
	var visit func(key, from K, hasFrom bool) error
	visit = func(key, from K, hasFrom bool) error {
		switch states[key] {
		case graphStateVisiting:
			return GraphCycleError[K]{Key: key}
		case graphStateDone:
			return nil
		}

		exists := true
		if cfg.Exists != nil {
			exists = cfg.Exists(key)
		}
		if !exists {
			if cfg.Missing == GraphCycleMissingPolicyIgnore {
				return nil
			}
			if !hasFrom {
				from = zero
			}
			return GraphMissingError[K]{From: from, Key: key}
		}

		states[key] = graphStateVisiting
		neighbors, err := cfg.Next(key)
		if err != nil {
			return err
		}
		for _, next := range neighbors {
			if err := visit(next, key, true); err != nil {
				return err
			}
		}
		states[key] = graphStateDone
		return nil
	}

	for _, start := range cfg.Starts {
		if err := visit(start, zero, false); err != nil {
			return err
		}
	}

	return nil
}
