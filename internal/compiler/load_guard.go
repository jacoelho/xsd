package compiler

import "fmt"

type loadingState[K comparable, V any] interface {
	IsLoading(key K) bool
	LoadingValue(key K) (V, bool)
}

func checkCircularLoad[K comparable, V any](state loadingState[K, V], key K, label string) (V, error) {
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

type entryLifecycle[E any] struct {
	Enter    func() (*E, func())
	Init     func(entry *E) error
	Finalize func(entry *E)
	Reset    func(entry *E)
}

func (l entryLifecycle[E]) Begin() (*E, func(), error) {
	if l.Enter == nil {
		return nil, nil, fmt.Errorf("entry lifecycle begin callback is nil")
	}
	entry, cleanup := l.Enter()
	if cleanup == nil {
		cleanup = func() {}
	}
	return entry, cleanup, nil
}

func (l entryLifecycle[E]) Commit(entry *E) {
	if l.Finalize == nil {
		return
	}
	l.Finalize(entry)
}

func (l entryLifecycle[E]) Rollback(entry *E) {
	if l.Reset == nil {
		return
	}
	l.Reset(entry)
}
