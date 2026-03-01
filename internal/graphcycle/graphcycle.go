package graphcycle

import "fmt"

type visitState uint8

const (
	stateVisiting visitState = iota + 1
	stateDone
)

// MissingPolicy controls behavior when a referenced node is missing.
type MissingPolicy uint8

const (
	MissingPolicyIgnore MissingPolicy = iota
	MissingPolicyError
)

// CycleError reports a cycle at Key.
type CycleError[K comparable] struct {
	Key K
}

// Error returns the error string.
func (e CycleError[K]) Error() string {
	return "cycle detected"
}

// MissingError reports a missing referenced node.
type MissingError[K comparable] struct {
	From K
	Key  K
}

// Error returns the error string.
func (e MissingError[K]) Error() string {
	return "missing node"
}

// Config configures generic cycle detection traversal.
type Config[K comparable] struct {
	Exists  func(K) bool
	Next    func(K) ([]K, error)
	Starts  []K
	Missing MissingPolicy
}

// Detect walks directed edges from Starts and reports first cycle or traversal error.
func Detect[K comparable](cfg Config[K]) error {
	if cfg.Next == nil {
		return fmt.Errorf("cycle detect: next function is nil")
	}
	states := make(map[K]visitState, len(cfg.Starts))

	var zero K
	var visit func(key, from K, hasFrom bool) error
	visit = func(key, from K, hasFrom bool) error {
		switch states[key] {
		case stateVisiting:
			return CycleError[K]{Key: key}
		case stateDone:
			return nil
		}

		exists := true
		if cfg.Exists != nil {
			exists = cfg.Exists(key)
		}
		if !exists {
			if cfg.Missing == MissingPolicyIgnore {
				return nil
			}
			if !hasFrom {
				from = zero
			}
			return MissingError[K]{From: from, Key: key}
		}

		states[key] = stateVisiting
		neighbors, err := cfg.Next(key)
		if err != nil {
			return err
		}
		for _, next := range neighbors {
			if err := visit(next, key, true); err != nil {
				return err
			}
		}
		states[key] = stateDone
		return nil
	}

	for _, start := range cfg.Starts {
		if err := visit(start, zero, false); err != nil {
			return err
		}
	}

	return nil
}
