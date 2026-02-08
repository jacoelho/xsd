package loadgraph

import "fmt"

// LoadingState exposes load-progress lookup needed for circular checks.
type LoadingState[K comparable, V any] interface {
	IsLoading(key K) bool
	LoadingValue(key K) (V, bool)
}

// CheckCircular returns the in-progress value when a circular dependency is found.
func CheckCircular[K comparable, V any](state LoadingState[K, V], key K, label string) (V, error) {
	var zero V
	if state == nil {
		return zero, fmt.Errorf("no loader state configured")
	}
	if !state.IsLoading(key) {
		return zero, nil
	}
	if inProgress, ok := state.LoadingValue(key); ok {
		return inProgress, nil
	}
	return zero, fmt.Errorf("circular dependency detected: %s", label)
}
